package mqtt_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/url"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/eclipse/paho.golang/autopaho"
	"github.com/eclipse/paho.golang/paho"
	mochi "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/hooks/auth"
	"github.com/mochi-mqtt/server/v2/listeners"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	bridgemqtt "github.com/JohnnyMatteus/garden-iot/services/bridge/internal/infrastructure/mqtt"
)

type recorder struct {
	mu       sync.Mutex
	payloads []string
	failures atomic.Int32
}

func (r *recorder) Handle(_ context.Context, topic string, payload []byte) error {
	if r.failures.Add(-1) >= 0 {
		return errors.New("kafka indisponível")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.payloads = append(r.payloads, topic+"|"+string(payload))
	return nil
}

func (r *recorder) got() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.payloads...)
}

func startBroker(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := l.Addr().String()
	require.NoError(t, l.Close())

	srv := mochi.New(&mochi.Options{Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	require.NoError(t, srv.AddHook(new(auth.AllowHook), nil))
	require.NoError(t, srv.AddListener(listeners.NewTCP(listeners.Config{ID: "t", Address: addr})))
	go func() { _ = srv.Serve() }()
	t.Cleanup(func() { _ = srv.Close() })
	require.Eventually(t, func() bool {
		c, err := net.Dial("tcp", addr)
		if err == nil {
			_ = c.Close()
		}
		return err == nil
	}, 3*time.Second, 20*time.Millisecond)
	return "tcp://" + addr
}

func publish(t *testing.T, broker, topic, payload string) {
	t.Helper()
	u, _ := url.Parse(broker)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cm, err := autopaho.NewConnection(ctx, autopaho.ClientConfig{
		ServerUrls: []*url.URL{u}, KeepAlive: 20, CleanStartOnInitialConnection: true,
		ClientConfig: paho.ClientConfig{ClientID: "pub"},
	})
	require.NoError(t, err)
	require.NoError(t, cm.AwaitConnection(ctx))
	_, err = cm.Publish(ctx, &paho.Publish{Topic: topic, QoS: 1, Payload: []byte(payload)})
	require.NoError(t, err)
	_ = cm.Disconnect(ctx)
}

func runSubscriber(t *testing.T, broker string, h bridgemqtt.Handler) {
	t.Helper()
	sub := bridgemqtt.NewSubscriber(bridgemqtt.Config{
		BrokerURL: broker, ClientID: "bridge-test", ShareGroup: "bridge", TopicFilter: "horta/+/+/+/telemetria",
		SessionExpiry: time.Minute, RetryBackoff: 20 * time.Millisecond,
	}, h, slog.New(slog.NewTextHandler(io.Discard, nil)))
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { _ = sub.Run(ctx); close(done) }()
	t.Cleanup(func() { cancel(); <-done })
	select {
	case <-sub.Ready():
	case <-time.After(10 * time.Second):
		t.Fatal("assinatura não ficou ativa")
	}
}

func TestReceivesThroughSharedSubscription(t *testing.T) {
	broker := startBroker(t)
	rec := &recorder{}
	runSubscriber(t, broker, rec)

	publish(t, broker, "horta/t1/h1/N/telemetria", `{"seq":10}`)

	assert.Eventually(t, func() bool { return len(rec.got()) == 1 }, 5*time.Second, 50*time.Millisecond)
	assert.Equal(t, []string{`horta/t1/h1/N/telemetria|{"seq":10}`}, rec.got())
}

func TestRetriesUntilHandlerSucceeds(t *testing.T) {
	broker := startBroker(t)
	rec := &recorder{}
	rec.failures.Store(3)
	runSubscriber(t, broker, rec)

	publish(t, broker, "horta/t1/h1/N/telemetria", `{"seq":1}`)

	assert.Eventually(t, func() bool { return len(rec.got()) == 1 }, 5*time.Second, 50*time.Millisecond)
}

func TestSharedFilter(t *testing.T) {
	assert.Equal(t, "$share/g/horta/+/+/+/telemetria", bridgemqtt.SharedFilter("g", "horta/+/+/+/telemetria"))
}
