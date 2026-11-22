package mqtt

import (
	"context"
	"encoding/json"
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

	"github.com/JohnnyMatteus/garden-iot/services/irrigation/application"
)

var ref = application.NodeRef{Tenant: "demo", Garden: "h1", Node: "n1"}

func TestDecodeState(t *testing.T) {
	now := time.Unix(1763726400, 0).UTC()
	st, err := decodeState([]byte(`{"valve":"open","flow_lpm":1.5,"ts":1763726000}`), now)
	require.NoError(t, err)
	assert.Equal(t, application.ValveOpen, st.Valve)
	assert.InDelta(t, 1.5, *st.FlowLpm, 1e-9)
	assert.Equal(t, time.Unix(1763726000, 0).UTC(), st.At)

	st, err = decodeState([]byte(`{"valve":"closed"}`), now)
	require.NoError(t, err)
	assert.Nil(t, st.FlowLpm)
	assert.Equal(t, now, st.At, "sem ts usa o instante de recebimento")

	_, err = decodeState([]byte(`{"valve":"meio"}`), now)
	assert.Error(t, err)
}

func TestDecodeResponsePrefersCorrelationData(t *testing.T) {
	id, ok, err := decodeResponse([]byte(`{"id":"corpo","accepted":true}`), []byte("corr"))
	require.NoError(t, err)
	assert.Equal(t, "corr", id)
	assert.True(t, ok)
	id, _, _ = decodeResponse([]byte(`{"id":"corpo","accepted":false}`), nil)
	assert.Equal(t, "corpo", id)
	_, _, err = decodeResponse([]byte(`{"accepted":true}`), nil)
	assert.Error(t, err)
}

func TestParseTopic(t *testing.T) {
	r, s, ok := parseTopic("horta/demo/h1/n1/cmd/resposta")
	assert.True(t, ok)
	assert.Equal(t, ref, r)
	assert.Equal(t, "cmd/resposta", s)
	_, _, ok = parseTopic("x/demo/h1/n1/estado")
	assert.False(t, ok)
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

// fakeNode é um nó atuador: assina cmd, responde e reporta o estado como o firmware faria.
func fakeNode(t *testing.T, broker string, accept bool, silent bool) (cmds func() []commandPayload) {
	t.Helper()
	var mu sync.Mutex
	var got []commandPayload
	u, _ := url.Parse(broker)
	var self atomic.Pointer[autopaho.ConnectionManager]
	ready := make(chan struct{})
	cm, err := autopaho.NewConnection(context.Background(), autopaho.ClientConfig{
		ServerUrls: []*url.URL{u}, KeepAlive: 30,
		OnConnectionUp: func(cm *autopaho.ConnectionManager, _ *paho.Connack) {
			self.Store(cm)
			_, _ = cm.Subscribe(context.Background(), &paho.Subscribe{Subscriptions: []paho.SubscribeOptions{{Topic: "horta/demo/h1/n1/cmd", QoS: 1}}})
			close(ready)
		},
		ClientConfig: paho.ClientConfig{ClientID: "node", OnPublishReceived: []func(paho.PublishReceived) (bool, error){
			func(pr paho.PublishReceived) (bool, error) {
				var c commandPayload
				_ = json.Unmarshal(pr.Packet.Payload, &c)
				mu.Lock()
				got = append(got, c)
				mu.Unlock()
				if silent {
					return true, nil
				}
				resp, _ := json.Marshal(responsePayload{ID: c.ID, Accepted: accept})
				_, _ = self.Load().Publish(context.Background(), &paho.Publish{Topic: "horta/demo/h1/n1/cmd/resposta", QoS: 1, Payload: resp,
					Properties: &paho.PublishProperties{CorrelationData: pr.Packet.Properties.CorrelationData}})
				if accept {
					v := "open"
					if c.Action == actionClose {
						v = "closed"
					}
					st, _ := json.Marshal(statePayload{Valve: v, TS: time.Now().Unix()})
					_, _ = self.Load().Publish(context.Background(), &paho.Publish{Topic: "horta/demo/h1/n1/estado", QoS: 1, Payload: st})
				}
				return true, nil
			}}},
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = cm.Disconnect(context.Background()) })
	<-ready
	return func() []commandPayload {
		mu.Lock()
		defer mu.Unlock()
		return append([]commandPayload(nil), got...)
	}
}

func connect(t *testing.T, broker string, timeout time.Duration) *Bus {
	t.Helper()
	b, err := Connect(context.Background(), Config{BrokerURL: broker, ClientID: "irrigation", ResponseTimeout: timeout},
		slog.New(slog.NewTextHandler(io.Discard, nil)))
	require.NoError(t, err)
	t.Cleanup(func() { _ = b.Disconnect(context.Background()) })
	time.Sleep(200 * time.Millisecond) // assinatura de estado/resposta concluída
	return b
}

func TestOpenCloseRoundTrip(t *testing.T) {
	broker := startBroker(t)
	cmds := fakeNode(t, broker, true, false)
	b := connect(t, broker, 3*time.Second)

	require.NoError(t, b.SetDesired(context.Background(), ref, application.ValveOpen))
	require.NoError(t, b.Open(context.Background(), ref, 90*time.Second))
	require.Eventually(t, func() bool {
		st, ok := b.Reported(context.Background(), ref)
		return ok && st.Valve == application.ValveOpen
	}, 2*time.Second, 20*time.Millisecond)

	require.NoError(t, b.Close(context.Background(), ref))
	require.Eventually(t, func() bool {
		st, _ := b.Reported(context.Background(), ref)
		return st.Valve == application.ValveClosed
	}, 2*time.Second, 20*time.Millisecond)

	got := cmds()
	require.Len(t, got, 2)
	assert.Equal(t, actionOpen, got[0].Action)
	assert.Equal(t, 90, got[0].DurationS)
	assert.Greater(t, got[0].ExpiresAt, time.Now().Unix())
	assert.Equal(t, actionClose, got[1].Action)
	d, _ := b.Desired(ref)
	assert.Equal(t, application.ValveOpen, d)
}

func TestOpenRefusedByNode(t *testing.T) {
	broker := startBroker(t)
	fakeNode(t, broker, false, false)
	b := connect(t, broker, 3*time.Second)
	err := b.Open(context.Background(), ref, time.Minute)
	assert.ErrorContains(t, err, "recusou")
}

func TestOpenTimesOutWithoutResponse(t *testing.T) {
	broker := startBroker(t)
	fakeNode(t, broker, true, true)
	b := connect(t, broker, 1*time.Second)
	err := b.Open(context.Background(), ref, time.Minute)
	assert.ErrorContains(t, err, "sem cmd/resposta")
}
