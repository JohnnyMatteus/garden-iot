package mqtt

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"net/url"
	"time"

	"github.com/eclipse/paho.golang/autopaho"
	"github.com/eclipse/paho.golang/paho"

	"github.com/JohnnyMatteus/garden-iot/services/api/domain"
)

// Handler é a porta de entrada dos eventos dos nós (implementada pelos casos de uso).
type Handler interface {
	ApplyNodeReport(ctx context.Context, tenant, gardenID, nodeID string, st domain.ActuatorState) error
	ApplyCommandResponse(ctx context.Context, tenant, commandID string, accepted bool) error
}

type Config struct {
	BrokerURL string // ex.: tls://localhost:8883
	TLS       *tls.Config
	ClientID  string
}

type Bus struct {
	cm  *autopaho.ConnectionManager
	h   Handler
	log *slog.Logger
	now func() time.Time
}

// Connect inicia a conexão (com reconexão automática) e espera a primeira.
func Connect(ctx context.Context, cfg Config, h Handler, log *slog.Logger) (*Bus, error) {
	u, err := url.Parse(cfg.BrokerURL)
	if err != nil {
		return nil, fmt.Errorf("url do broker: %w", err)
	}
	b := &Bus{h: h, log: log, now: time.Now}
	cm, err := autopaho.NewConnection(ctx, autopaho.ClientConfig{
		ServerUrls: []*url.URL{u}, TlsCfg: cfg.TLS, KeepAlive: 30,
		SessionExpiryInterval: 3600,
		OnConnectionUp: func(cm *autopaho.ConnectionManager, _ *paho.Connack) {
			_, err := cm.Subscribe(ctx, &paho.Subscribe{Subscriptions: []paho.SubscribeOptions{
				{Topic: "horta/+/+/+/" + suffixState, QoS: 1},
				{Topic: "horta/+/+/+/" + suffixResponse, QoS: 1},
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

func (b *Bus) Close(ctx context.Context) error { return b.cm.Disconnect(ctx) }

// Publish envia o comando com QoS 1, Correlation Data = id, Response Topic e Message Expiry,
// para o nó não executar um comando atrasado (design doc, seção 5.1).
func (b *Bus) Publish(ctx context.Context, tenant string, c domain.Command) error {
	ref := NodeRef{tenant, c.GardenID, c.NodeID}
	payload, err := encodeCommand(c)
	if err != nil {
		return err
	}
	ttl := uint32(time.Until(c.ExpiresAt).Seconds())
	if ttl == 0 {
		return fmt.Errorf("comando %s já expirou", c.ID)
	}
	resp, err := b.cm.Publish(ctx, &paho.Publish{
		Topic: ref.topic(suffixCommand), QoS: 1, Payload: payload,
		Properties: &paho.PublishProperties{
			CorrelationData: []byte(c.ID),
			ResponseTopic:   ref.topic(suffixResponse),
			MessageExpiry:   &ttl,
			ContentType:     "application/json",
		},
	})
	if err != nil {
		return err
	}
	if resp != nil && resp.ReasonCode >= 0x80 {
		return fmt.Errorf("broker recusou o comando (código %#x)", resp.ReasonCode)
	}
	return nil
}

func (b *Bus) onMessage(pr paho.PublishReceived) (bool, error) {
	ctx := context.Background()
	ref, suffix, err := parseTopic(pr.Packet.Topic)
	if err != nil {
		b.log.Warn("mensagem ignorada", "topic", pr.Packet.Topic, "err", err)
		return false, nil
	}
	switch suffix {
	case suffixState:
		st, err := decodeState(pr.Packet.Payload, b.now())
		if err == nil {
			err = b.h.ApplyNodeReport(ctx, ref.Tenant, ref.Garden, ref.Node, st)
		}
		b.logIf(err, "estado do nó", pr.Packet.Topic)
	case suffixResponse:
		var corr []byte
		if pr.Packet.Properties != nil {
			corr = pr.Packet.Properties.CorrelationData
		}
		id, accepted, err := decodeResponse(pr.Packet.Payload, corr)
		if err == nil {
			err = b.h.ApplyCommandResponse(ctx, ref.Tenant, id, accepted)
		}
		b.logIf(err, "resposta de comando", pr.Packet.Topic)
	}
	return true, nil
}

func (b *Bus) logIf(err error, what, topic string) {
	if err != nil {
		b.log.Warn("falha ao tratar "+what, "topic", topic, "err", err)
	}
}
