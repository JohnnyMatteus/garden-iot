// Package publisher implementa o Dialer do simulador sobre MQTT 5 com mTLS.
package publisher

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/eclipse/paho.golang/autopaho"
	"github.com/eclipse/paho.golang/paho"

	"github.com/JohnnyMatteus/garden-iot/tools/fleet-sim/internal/fleet"
)

type Config struct {
	BrokerURL string
	CertFile  string
	KeyFile   string
	CAFile    string
}

type Dialer struct {
	cfg Config
	tls *tls.Config
}

// New carrega o certificado do simulador (CN svc:fleet-sim); sem arquivos, conecta sem TLS.
func New(cfg Config) (*Dialer, error) {
	d := &Dialer{cfg: cfg}
	if cfg.CertFile == "" && cfg.CAFile == "" {
		return d, nil
	}
	t := &tls.Config{MinVersion: tls.VersionTLS12}
	if cfg.CAFile != "" {
		pem, err := os.ReadFile(cfg.CAFile)
		if err != nil {
			return nil, fmt.Errorf("ca: %w", err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(pem) {
			return nil, fmt.Errorf("ca sem certificados válidos")
		}
		t.RootCAs = pool
	}
	if cfg.CertFile != "" {
		cert, err := tls.LoadX509KeyPair(cfg.CertFile, cfg.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("certificado: %w", err)
		}
		t.Certificates = []tls.Certificate{cert}
	}
	d.tls = t
	return d, nil
}

func (d *Dialer) Dial(ctx context.Context, clientID, willTopic string, willPayload []byte) (fleet.Link, error) {
	u, err := url.Parse(d.cfg.BrokerURL)
	if err != nil {
		return nil, err
	}
	connCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
	cm, err := autopaho.NewConnection(connCtx, autopaho.ClientConfig{
		ServerUrls: []*url.URL{u}, TlsCfg: d.tls, KeepAlive: 30, CleanStartOnInitialConnection: true,
		ReconnectBackoff: func(int) time.Duration { return time.Second },
		WillMessage:      &paho.WillMessage{Topic: willTopic, Payload: willPayload, QoS: 1},
		ClientConfig:     paho.ClientConfig{ClientID: clientID},
	})
	if err != nil {
		cancel()
		return nil, err
	}
	awaitCtx, awaitCancel := context.WithTimeout(ctx, 15*time.Second)
	defer awaitCancel()
	if err := cm.AwaitConnection(awaitCtx); err != nil {
		cancel()
		return nil, fmt.Errorf("conexão de %s: %w", clientID, err)
	}
	return &link{cm: cm, cancel: cancel}, nil
}

type link struct {
	cm     *autopaho.ConnectionManager
	cancel context.CancelFunc
}

// ErrNoMatchingSubscribers indica PUBACK com código 0x10: o broker aceitou a mensagem, mas não havia
// ninguém para recebê-la, então ela foi descartada. Para a telemetria isso não é entrega.
var ErrNoMatchingSubscribers = errors.New("PUBACK 0x10: nenhum assinante recebeu a mensagem")

const reasonNoMatchingSubscribers = 0x10

func (l *link) Publish(ctx context.Context, topic string, payload []byte) error {
	resp, err := l.cm.Publish(ctx, &paho.Publish{Topic: topic, QoS: 1, Payload: payload})
	if err != nil {
		return err
	}
	if resp == nil {
		return nil
	}
	return routingError(topic, resp.ReasonCode)
}

// routingError diz se o PUBACK confirma o roteamento. O autopaho só trata códigos >= 0x80 como erro, então
// 0x10 chegaria como sucesso. Só a telemetria exige assinante: a ACL só deixa o grupo $share/bridge
// assiná-la, de modo que 0x10 significa que nenhuma réplica da bridge estava assinada (por exemplo logo
// depois de o EMQX reiniciar, que perde as assinaturas em memória). Estado e LWT podem não ter assinante.
func routingError(topic string, reason byte) error {
	if reason == reasonNoMatchingSubscribers && strings.HasSuffix(topic, "/telemetria") {
		return ErrNoMatchingSubscribers
	}
	return nil
}

// Kill corta a conexão sem DISCONNECT (o broker publica o Last Will) e impede a reconexão.
func (l *link) Kill() {
	l.cm.TerminateConnectionForTest()
	l.cancel()
}

func (l *link) Close(ctx context.Context) {
	dctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	_ = l.cm.Disconnect(dctx)
	l.cancel()
}
