//go:build integration

// Testes de segurança do EMQX (M1-06). Exigem `make up` e `make seed`.
package broker_test

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/eclipse/paho.golang/paho"
)

var (
	pkiDir   = filepath.Join("..", "..", "pki", "out")
	brokerAt = envOr("EMQX_ADDR", "localhost:8883")
)

func envOr(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}

func tlsConfig(t *testing.T, certBase string) *tls.Config {
	t.Helper()
	ca, err := os.ReadFile(filepath.Join(pkiDir, "ca.pem"))
	if err != nil {
		t.Fatal(err)
	}
	pool := x509.NewCertPool()
	pool.AppendCertsFromPEM(ca)
	cfg := &tls.Config{RootCAs: pool, ServerName: "localhost", MinVersion: tls.VersionTLS12}
	if certBase != "" {
		pair, err := tls.LoadX509KeyPair(certBase+".pem", certBase+".key")
		if err != nil {
			t.Fatal(err)
		}
		cfg.Certificates = []tls.Certificate{pair}
	}
	return cfg
}

func device(tenant, garden, node string) string {
	return filepath.Join(pkiDir, "devices", fmt.Sprintf("%s-%s-%s", tenant, garden, node))
}

func service(name string) string { return filepath.Join(pkiDir, name) }

// connect abre MQTT 5 sobre mTLS; devolve erro quando o broker recusa em qualquer camada.
func connect(ctx context.Context, t *testing.T, certBase string) (*paho.Client, error) {
	t.Helper()
	conn, err := tls.DialWithDialer(&net.Dialer{Timeout: 5 * time.Second}, "tcp", brokerAt, tlsConfig(t, certBase))
	if err != nil {
		return nil, err
	}
	c := paho.NewClient(paho.ClientConfig{Conn: conn})
	ack, err := c.Connect(ctx, &paho.Connect{
		ClientID:   fmt.Sprintf("acltest-%d", time.Now().UnixNano()),
		KeepAlive:  30,
		CleanStart: true,
	})
	if err != nil {
		return nil, err
	}
	if ack.ReasonCode >= 0x80 {
		return nil, fmt.Errorf("connack %#x", ack.ReasonCode)
	}
	return c, nil
}

func mustConnect(ctx context.Context, t *testing.T, certBase string) *paho.Client {
	t.Helper()
	c, err := connect(ctx, t, certBase)
	if err != nil {
		t.Fatalf("conexão com %s: %v", certBase, err)
	}
	t.Cleanup(func() { _ = c.Disconnect(&paho.Disconnect{ReasonCode: 0}) })
	return c
}

// Com deny_action=ignore o broker devolve 0x87 no PUBACK; a queda da conexão também conta como negado.
func publishDenied(ctx context.Context, c *paho.Client, topic string) bool {
	resp, err := c.Publish(ctx, &paho.Publish{Topic: topic, QoS: 1, Payload: []byte(`{}`)})
	return err != nil || resp.ReasonCode >= 0x80
}

func subscribeDenied(ctx context.Context, c *paho.Client, filter string) bool {
	ack, err := c.Subscribe(ctx, &paho.Subscribe{Subscriptions: []paho.SubscribeOptions{{Topic: filter, QoS: 1}}})
	return err != nil || len(ack.Reasons) == 0 || ack.Reasons[0] >= 0x80
}

func provision(t *testing.T, args ...string) {
	t.Helper()
	out, err := exec.Command(filepath.Join("..", "..", "vault", "provision.sh"), args...).CombinedOutput()
	if err != nil {
		t.Fatalf("provision %v: %v\n%s", args, err, out)
	}
}

func ctxFor(t *testing.T) context.Context {
	c, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)
	return c
}

func TestAnonymousClientIsRefused(t *testing.T) {
	if c, err := connect(ctxFor(t), t, ""); err == nil {
		_ = c.Disconnect(&paho.Disconnect{})
		t.Fatal("conexão sem certificado de cliente foi aceita")
	}
}

func TestNodePublishesOnlyOnItsOwnTopics(t *testing.T) {
	c := mustConnect(ctxFor(t), t, device("demo", "horta-1", "n1"))
	if publishDenied(ctxFor(t), c, "horta/demo/horta-1/n1/telemetria") {
		t.Fatal("publicação no próprio tópico foi negada")
	}
	if !publishDenied(ctxFor(t), c, "horta/demo/horta-1/n2/telemetria") {
		t.Fatal("publicação no tópico de outro nó foi aceita")
	}
}

func TestTenantIsolation(t *testing.T) {
	provision(t, "other", "horta-9", "n1")
	c := mustConnect(ctxFor(t), t, device("other", "horta-9", "n1"))
	if !subscribeDenied(ctxFor(t), c, "horta/demo/#") {
		t.Fatal("assinatura de outro tenant foi aceita")
	}
	if !subscribeDenied(ctxFor(t), c, "horta/#") {
		t.Fatal("assinatura curinga ampla foi aceita")
	}
	if subscribeDenied(ctxFor(t), c, "horta/other/horta-9/n1/cmd") {
		t.Fatal("assinatura do próprio cmd foi negada")
	}
}

func TestBridgeScope(t *testing.T) {
	c := mustConnect(ctxFor(t), t, service("bridge"))
	if subscribeDenied(ctxFor(t), c, "horta/+/+/+/telemetria") {
		t.Fatal("bridge não consegue assinar a telemetria")
	}
	if !publishDenied(ctxFor(t), c, "horta/demo/horta-1/n1/cmd") {
		t.Fatal("bridge publicou em cmd")
	}
}

func TestRevokedCertificateIsRefused(t *testing.T) {
	provision(t, "demo", "horta-1", "revogado")
	base := device("demo", "horta-1", "revogado")
	c := mustConnect(ctxFor(t), t, base)
	_ = c.Disconnect(&paho.Disconnect{})

	provision(t, "--revoke", "demo", "horta-1", "revogado")
	// O EMQX atualiza a CRL a cada refresh_interval (1m no ambiente local).
	deadline := time.Now().Add(150 * time.Second)
	for time.Now().Before(deadline) {
		c, err := connect(ctxFor(t), t, base)
		if err != nil {
			return
		}
		_ = c.Disconnect(&paho.Disconnect{})
		time.Sleep(5 * time.Second)
	}
	t.Fatal("certificado revogado continuou sendo aceito")
}
