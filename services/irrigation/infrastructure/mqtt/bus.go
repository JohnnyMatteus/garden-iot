// Package mqtt liga o serviço de irrigação ao EMQX: publica comandos (MQTT 5, com Correlation
// Data e Message Expiry) e mantém o estado reportado dos nós. Implementa application.Actuator e
// application.Twin. O nó decide; este adaptador só pede (ADR 0004).
package mqtt

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/eclipse/paho.golang/autopaho"
	"github.com/eclipse/paho.golang/paho"

	"github.com/JohnnyMatteus/garden-iot/services/irrigation/application"
)

const (
	actionOpen  = "open_valve"
	actionClose = "close_valve"
)

type Config struct {
	BrokerURL       string
	TLS             *tls.Config
	ClientID        string
	ResponseTimeout time.Duration // espera pelo cmd/resposta; também é a validade do comando
}

type Bus struct {
	cm  *autopaho.ConnectionManager
	cfg Config
	log *slog.Logger
	now func() time.Time

	mu       sync.Mutex
	pending  map[string]chan bool
	reported map[application.NodeRef]application.ActuatorState
	desired  map[application.NodeRef]application.Valve
}

var (
	_ application.Actuator = (*Bus)(nil)
	_ application.Twin     = (*Bus)(nil)
)

func newBus(cfg Config, log *slog.Logger) *Bus {
	if cfg.ResponseTimeout == 0 {
		cfg.ResponseTimeout = 15 * time.Second
	}
	return &Bus{cfg: cfg, log: log, now: time.Now, pending: map[string]chan bool{},
		reported: map[application.NodeRef]application.ActuatorState{}, desired: map[application.NodeRef]application.Valve{}}
}

// Connect inicia a conexão (reconexão automática) e espera a primeira.
func Connect(ctx context.Context, cfg Config, log *slog.Logger) (*Bus, error) {
	u, err := url.Parse(cfg.BrokerURL)
	if err != nil {
		return nil, fmt.Errorf("url do broker: %w", err)
	}
	b := newBus(cfg, log)
	cm, err := autopaho.NewConnection(ctx, autopaho.ClientConfig{
		ServerUrls: []*url.URL{u}, TlsCfg: cfg.TLS, KeepAlive: 30, SessionExpiryInterval: 3600,
		OnConnectionUp: func(cm *autopaho.ConnectionManager, _ *paho.Connack) {
			_, err := cm.Subscribe(ctx, &paho.Subscribe{Subscriptions: []paho.SubscribeOptions{
				{Topic: "horta/+/+/+/estado", QoS: 1},
				{Topic: "horta/+/+/+/cmd/resposta", QoS: 1},
			}})
			if err != nil {
				log.Error("assinatura MQTT falhou", "err", err)
			}
		},
		OnConnectError: func(err error) { log.Warn("conexão MQTT falhou", "err", err) },
		ClientConfig: paho.ClientConfig{
			ClientID:          cfg.ClientID,
			OnPublishReceived: []func(paho.PublishReceived) (bool, error){b.onMessage},
		},
	})
	if err != nil {
		return nil, err
	}
	b.cm = cm
	if err := cm.AwaitConnection(ctx); err != nil {
		return nil, fmt.Errorf("conexão MQTT: %w", err)
	}
	return b, nil
}

func (b *Bus) Disconnect(ctx context.Context) error { return b.cm.Disconnect(ctx) }

func topic(ref application.NodeRef, suffix string) string {
	return fmt.Sprintf("horta/%s/%s/%s/%s", ref.Tenant, ref.Garden, ref.Node, suffix)
}

type commandPayload struct {
	ID        string `json:"id"`
	Action    string `json:"action"`
	DurationS int    `json:"duration_s"`
	ExpiresAt int64  `json:"expires_at"`
}

func newCommandID() string {
	var raw [12]byte
	_, _ = rand.Read(raw[:])
	return hex.EncodeToString(raw[:])
}

func (b *Bus) Open(ctx context.Context, ref application.NodeRef, maxDuration time.Duration) error {
	return b.send(ctx, ref, actionOpen, int(maxDuration.Seconds()))
}

// Close envia close_valve e espera a resposta.
func (b *Bus) Close(ctx context.Context, ref application.NodeRef) error {
	return b.send(ctx, ref, actionClose, 0)
}

// send publica o comando e espera a resposta correlacionada.
func (b *Bus) send(ctx context.Context, ref application.NodeRef, action string, durationS int) error {
	id := newCommandID()
	exp := b.now().Add(b.cfg.ResponseTimeout)
	payload, err := json.Marshal(commandPayload{ID: id, Action: action, DurationS: durationS, ExpiresAt: exp.Unix()})
	if err != nil {
		return err
	}
	ch := make(chan bool, 1)
	b.mu.Lock()
	b.pending[id] = ch
	b.mu.Unlock()
	defer func() {
		b.mu.Lock()
		delete(b.pending, id)
		b.mu.Unlock()
	}()

	ttl := uint32(b.cfg.ResponseTimeout.Seconds())
	resp, err := b.cm.Publish(ctx, &paho.Publish{
		Topic: topic(ref, "cmd"), QoS: 1, Payload: payload,
		Properties: &paho.PublishProperties{
			CorrelationData: []byte(id), ResponseTopic: topic(ref, "cmd/resposta"),
			MessageExpiry: &ttl, ContentType: "application/json",
		},
	})
	if err != nil {
		return fmt.Errorf("publicar %s: %w", action, err)
	}
	if resp != nil && resp.ReasonCode >= 0x80 {
		return fmt.Errorf("broker recusou %s (código %#x)", action, resp.ReasonCode)
	}

	select {
	case accepted := <-ch:
		if !accepted {
			return fmt.Errorf("nó recusou %s", action)
		}
		return nil
	case <-time.After(b.cfg.ResponseTimeout):
		return fmt.Errorf("sem cmd/resposta para %s em %s", action, b.cfg.ResponseTimeout)
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (b *Bus) SetDesired(_ context.Context, ref application.NodeRef, v application.Valve) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.desired[ref] = v
	return nil
}

// Desired devolve o estado desejado, para diagnóstico e painéis.
func (b *Bus) Desired(ref application.NodeRef) (application.Valve, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	v, ok := b.desired[ref]
	return v, ok
}

func (b *Bus) Reported(_ context.Context, ref application.NodeRef) (application.ActuatorState, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	st, ok := b.reported[ref]
	return st, ok
}

type statePayload struct {
	Valve   string   `json:"valve"`
	FlowLpm *float64 `json:"flow_lpm"`
	TS      int64    `json:"ts"`
}

func decodeState(payload []byte, now time.Time) (application.ActuatorState, error) {
	var p statePayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return application.ActuatorState{}, err
	}
	v := application.Valve(p.Valve)
	if v != application.ValveOpen && v != application.ValveClosed {
		return application.ActuatorState{}, fmt.Errorf("válvula inválida %q", p.Valve)
	}
	at := now
	if p.TS > 0 {
		at = time.Unix(p.TS, 0).UTC()
	}
	return application.ActuatorState{Valve: v, FlowLpm: p.FlowLpm, At: at}, nil
}

type responsePayload struct {
	ID       string `json:"id"`
	Accepted bool   `json:"accepted"`
}

// decodeResponse prefere o Correlation Data (MQTT 5) e cai para o id no corpo.
func decodeResponse(payload, correlation []byte) (string, bool, error) {
	var p responsePayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return "", false, err
	}
	id := p.ID
	if len(correlation) > 0 {
		id = string(correlation)
	}
	if id == "" {
		return "", false, errors.New("resposta sem id de comando")
	}
	return id, p.Accepted, nil
}

// parseTopic extrai o nó e o sufixo de horta/{tenant}/{horta}/{no}/{sufixo}.
func parseTopic(t string) (application.NodeRef, string, bool) {
	p := strings.Split(t, "/")
	if len(p) < 5 || p[0] != "horta" {
		return application.NodeRef{}, "", false
	}
	return application.NodeRef{Tenant: p[1], Garden: p[2], Node: p[3]}, strings.Join(p[4:], "/"), true
}

func (b *Bus) onMessage(pr paho.PublishReceived) (bool, error) {
	ref, suffix, ok := parseTopic(pr.Packet.Topic)
	if !ok {
		return false, nil
	}
	switch suffix {
	case "estado":
		st, err := decodeState(pr.Packet.Payload, b.now())
		if err != nil {
			b.log.Warn("estado inválido", "topic", pr.Packet.Topic, "err", err)
			return true, nil
		}
		b.mu.Lock()
		b.reported[ref] = st
		b.mu.Unlock()
	case "cmd/resposta":
		var corr []byte
		if pr.Packet.Properties != nil {
			corr = pr.Packet.Properties.CorrelationData
		}
		id, accepted, err := decodeResponse(pr.Packet.Payload, corr)
		if err != nil {
			b.log.Warn("resposta inválida", "topic", pr.Packet.Topic, "err", err)
			return true, nil
		}
		b.mu.Lock()
		ch, found := b.pending[id]
		b.mu.Unlock()
		if found { // comando de outro serviço (a API também comanda os nós) é ignorado
			select {
			case ch <- accepted:
			default:
			}
		}
	}
	return true, nil
}
