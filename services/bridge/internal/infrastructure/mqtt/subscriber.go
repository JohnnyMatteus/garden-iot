// Package mqtt consome a telemetria do EMQX com assinatura compartilhada e ack manual.
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
)

// Handler devolve nil quando a mensagem pode ser confirmada ao broker.
type Handler interface {
	Handle(ctx context.Context, topic string, payload []byte) error
}

type Config struct {
	BrokerURL       string
	ClientID        string
	TLS             *tls.Config
	ShareGroup      string
	TopicFilter     string
	SessionExpiry   time.Duration
	MaxInFlight     int
	RetryBackoff    time.Duration
	RetryBackoffMax time.Duration
}

type Subscriber struct {
	cfg     Config
	handler Handler
	log     *slog.Logger
	sem     chan struct{}
}

func NewSubscriber(cfg Config, h Handler, log *slog.Logger) *Subscriber {
	if cfg.MaxInFlight <= 0 {
		cfg.MaxInFlight = 32
	}
	if cfg.RetryBackoff <= 0 {
		cfg.RetryBackoff = 500 * time.Millisecond
	}
	if cfg.RetryBackoffMax <= 0 {
		cfg.RetryBackoffMax = 10 * time.Second
	}
	return &Subscriber{cfg: cfg, handler: h, log: log, sem: make(chan struct{}, cfg.MaxInFlight)}
}

// SharedFilter monta o filtro $share/{grupo}/{filtro}, que reparte as mensagens entre as réplicas.
func SharedFilter(group, filter string) string { return "$share/" + group + "/" + filter }

// Run conecta, assina e bloqueia até o contexto ser cancelado.
func (s *Subscriber) Run(ctx context.Context) error {
	u, err := url.Parse(s.cfg.BrokerURL)
	if err != nil {
		return fmt.Errorf("url do broker: %w", err)
	}
	connCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	cm, err := autopaho.NewConnection(connCtx, autopaho.ClientConfig{
		ServerUrls:                    []*url.URL{u},
		TlsCfg:                        s.cfg.TLS,
		KeepAlive:                     30,
		CleanStartOnInitialConnection: false,
		SessionExpiryInterval:         uint32(s.cfg.SessionExpiry.Seconds()),
		ReconnectBackoff:              func(int) time.Duration { return 2 * time.Second },
		OnConnectionUp: func(cm *autopaho.ConnectionManager, _ *paho.Connack) {
			// a função não pode bloquear, então a assinatura roda em goroutine
			go s.subscribe(connCtx, cm)
		},
		OnConnectError: func(err error) { s.log.Warn("falha de conexão MQTT", "erro", err) },
		ClientConfig: paho.ClientConfig{
			ClientID:                   s.cfg.ClientID,
			EnableManualAcknowledgment: true,
			OnPublishReceived: []func(paho.PublishReceived) (bool, error){
				func(pr paho.PublishReceived) (bool, error) { s.dispatch(connCtx, pr); return true, nil },
			},
			OnClientError: func(err error) { s.log.Error("erro do cliente MQTT", "erro", err) },
		},
	})
	if err != nil {
		return fmt.Errorf("conexão MQTT: %w", err)
	}

	<-ctx.Done()
	s.drain()
	<-cm.Done()
	return nil
}

// dispatch processa em goroutine limitada; o ack só sai depois de o handler confirmar.
// O cliente reordena os acks, então a conclusão fora de ordem é segura.
func (s *Subscriber) dispatch(ctx context.Context, pr paho.PublishReceived) {
	select {
	case s.sem <- struct{}{}:
	case <-ctx.Done():
		return
	}
	go func() {
		defer func() { <-s.sem }()
		if s.process(ctx, pr.Packet) {
			if err := pr.Client.Ack(pr.Packet); err != nil {
				s.log.Warn("falha ao confirmar mensagem", "erro", err)
			}
		}
	}()
}

// process repete com backoff enquanto o handler falhar; sem ack, o broker reentrega após a reconexão.
func (s *Subscriber) process(ctx context.Context, p *paho.Publish) bool {
	backoff := s.cfg.RetryBackoff
	for {
		err := s.handler.Handle(ctx, p.Topic, p.Payload)
		if err == nil {
			return true
		}
		s.log.Error("falha ao processar, nova tentativa", "topico", p.Topic, "erro", err, "espera", backoff)
		select {
		case <-ctx.Done():
			return false
		case <-time.After(backoff):
		}
		backoff = min(backoff*2, s.cfg.RetryBackoffMax)
	}
}

// drain espera os handlers em andamento terminarem no desligamento.
func (s *Subscriber) drain() {
	for i := 0; i < cap(s.sem); i++ {
		s.sem <- struct{}{}
	}
}

func (s *Subscriber) subscribe(ctx context.Context, cm *autopaho.ConnectionManager) {
	filter := SharedFilter(s.cfg.ShareGroup, s.cfg.TopicFilter)
	ack, err := cm.Subscribe(ctx, &paho.Subscribe{Subscriptions: []paho.SubscribeOptions{{Topic: filter, QoS: 1}}})
	if err != nil {
		s.log.Error("falha ao assinar", "filtro", filter, "erro", err)
		return
	}
	for _, code := range ack.Reasons {
		if code >= 0x80 {
			s.log.Error("assinatura recusada pelo broker", "filtro", filter, "codigo", code)
			return
		}
	}
	s.log.Info("assinatura ativa", "filtro", filter)
}
