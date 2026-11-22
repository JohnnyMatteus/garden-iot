package publisher_test

import (
	"context"
	"io"
	"log/slog"
	"net"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/eclipse/paho.golang/autopaho"
	"github.com/eclipse/paho.golang/paho"
	mochi "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/hooks/auth"
	"github.com/mochi-mqtt/server/v2/listeners"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/JohnnyMatteus/garden-iot/tools/fleet-sim/internal/publisher"
)

func startBroker(t *testing.T) string {
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

func TestKillTriggersLastWill(t *testing.T) {
	broker := startBroker(t)
	u, _ := url.Parse(broker)

	var mu sync.Mutex
	var got []string
	ctx := context.Background()
	watcher, err := autopaho.NewConnection(ctx, autopaho.ClientConfig{
		ServerUrls: []*url.URL{u}, KeepAlive: 20, CleanStartOnInitialConnection: true,
		ClientConfig: paho.ClientConfig{ClientID: "watch", OnPublishReceived: []func(paho.PublishReceived) (bool, error){
			func(pr paho.PublishReceived) (bool, error) {
				mu.Lock()
				defer mu.Unlock()
				got = append(got, pr.Packet.Topic+"|"+string(pr.Packet.Payload))
				return true, nil
			}}},
	})
	require.NoError(t, err)
	require.NoError(t, watcher.AwaitConnection(ctx))
	_, err = watcher.Subscribe(ctx, &paho.Subscribe{Subscriptions: []paho.SubscribeOptions{
		{Topic: "horta/+/+/+/#", QoS: 1}}})
	require.NoError(t, err)

	d, err := publisher.New(publisher.Config{BrokerURL: broker})
	require.NoError(t, err)
	link, err := d.Dial(ctx, "sim-n1", "horta/t1/h1/n1/lwt", []byte(`{"online":false}`))
	require.NoError(t, err)
	require.NoError(t, link.Publish(ctx, "horta/t1/h1/n1/telemetria", []byte(`{"seq":1}`)))
	link.Kill()

	assert.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(got) == 2
	}, 5*time.Second, 50*time.Millisecond)
	mu.Lock()
	defer mu.Unlock()
	assert.Contains(t, got, `horta/t1/h1/n1/telemetria|{"seq":1}`)
	assert.Contains(t, got, `horta/t1/h1/n1/lwt|{"online":false}`)
}
