package application_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/JohnnyMatteus/garden-iot/services/bridge/internal/application"
	"github.com/JohnnyMatteus/garden-iot/services/bridge/internal/domain"
)

const topicN = "horta/t1/h1/N/telemetria"

type fakeTranslator struct{}

func (fakeTranslator) Translate(tp domain.Topic, payload []byte) (domain.Telemetry, error) {
	var in struct {
		Seq uint64 `json:"seq"`
		Fw  string `json:"fw"`
	}
	if err := json.Unmarshal(payload, &in); err != nil {
		return domain.Telemetry{}, domain.NewInvalid(domain.ReasonDecode, err)
	}
	return domain.Telemetry{Tenant: tp.Tenant, Horta: tp.Horta, NoID: tp.NoID, Sequence: in.Seq, FwVersion: in.Fw,
		ReadAt: time.Now(), Payload: payload}, nil
}

type fakeKafka struct {
	mu   sync.Mutex
	sent []domain.Telemetry
	err  error
}

func (f *fakeKafka) Publish(_ context.Context, t domain.Telemetry) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return f.err
	}
	f.sent = append(f.sent, t)
	return nil
}

type fakeDLQ struct{ got []domain.Rejected }

func (f *fakeDLQ) PublishRejected(_ context.Context, r domain.Rejected) error {
	f.got = append(f.got, r)
	return nil
}

type fakeDedup struct{ seen map[string]bool }

func (f *fakeDedup) Claim(_ context.Context, id string) (bool, error) {
	if f.seen[id] {
		return false, nil
	}
	f.seen[id] = true
	return true, nil
}
func (f *fakeDedup) Release(_ context.Context, id string) error { delete(f.seen, id); return nil }

type fakeMetrics struct {
	forwarded, duplicates int
	invalid               map[domain.Reason]int
}

func (m *fakeMetrics) Forwarded(string)        { m.forwarded++ }
func (m *fakeMetrics) Duplicate()              { m.duplicates++ }
func (m *fakeMetrics) Invalid(r domain.Reason) { m.invalid[r]++ }
func (m *fakeMetrics) Failed(string)           {}
func (m *fakeMetrics) Latency(float64)         {}

func setup() (*application.Handler, *fakeKafka, *fakeDLQ, *fakeMetrics) {
	k, d, m := &fakeKafka{}, &fakeDLQ{}, &fakeMetrics{invalid: map[domain.Reason]int{}}
	h := application.NewHandler(fakeTranslator{}, k, d, &fakeDedup{seen: map[string]bool{}}, m, slog.New(slog.NewTextHandler(io.Discard, nil)))
	return h, k, d, m
}

func batch(seq uint64) []byte {
	b, _ := json.Marshal(map[string]any{"seq": seq, "fw": "1.0.0"})
	return b
}

func TestForwardsBatch(t *testing.T) {
	h, k, _, m := setup()
	require.NoError(t, h.Handle(context.Background(), topicN, batch(10)))
	require.Len(t, k.sent, 1)
	assert.Equal(t, "N", k.sent[0].NoID)
	assert.EqualValues(t, 10, k.sent[0].Sequence)
	assert.False(t, k.sent[0].ReceivedAt.IsZero())
	assert.Equal(t, 1, m.forwarded)
}

func TestDropsDuplicate(t *testing.T) {
	h, k, _, m := setup()
	require.NoError(t, h.Handle(context.Background(), topicN, batch(10)))
	require.NoError(t, h.Handle(context.Background(), topicN, batch(10)))
	assert.Len(t, k.sent, 1)
	assert.Equal(t, 1, m.duplicates)
}

func TestAcceptsOutOfOrder(t *testing.T) {
	h, k, _, _ := setup()
	require.NoError(t, h.Handle(context.Background(), topicN, batch(12)))
	require.NoError(t, h.Handle(context.Background(), topicN, batch(11)))
	assert.Len(t, k.sent, 2)
}

func TestKafkaDownDoesNotAckAndRedeliveryIsNotDuplicate(t *testing.T) {
	h, k, _, _ := setup()
	k.err = errors.New("broker indisponível")
	require.Error(t, h.Handle(context.Background(), topicN, batch(10)))

	k.err = nil
	require.NoError(t, h.Handle(context.Background(), topicN, batch(10)))
	assert.Len(t, k.sent, 1)
}

func TestInvalidGoesToDLQWithReason(t *testing.T) {
	cases := map[string]struct {
		topic   string
		payload []byte
		reason  domain.Reason
	}{
		"payload":  {topicN, []byte("não é json"), domain.ReasonDecode},
		"tópico":   {"horta/t1/telemetria", batch(1), domain.ReasonTopic},
		"wildcard": {"horta/t1/h1/+/telemetria", batch(1), domain.ReasonTopic},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			h, k, d, m := setup()
			require.NoError(t, h.Handle(context.Background(), c.topic, c.payload))
			assert.Empty(t, k.sent)
			require.Len(t, d.got, 1)
			assert.Equal(t, c.reason, d.got[0].Reason)
			assert.Equal(t, c.payload, d.got[0].Payload)
			assert.Equal(t, 1, m.invalid[c.reason])
		})
	}
}

type spoofTranslator struct{}

func (spoofTranslator) Translate(domain.Topic, []byte) (domain.Telemetry, error) {
	return domain.Telemetry{Tenant: "t2", Horta: "h1", NoID: "N", Sequence: 1}, nil
}

func TestPayloadCannotOverrideTopicIdentity(t *testing.T) {
	k, d, m := &fakeKafka{}, &fakeDLQ{}, &fakeMetrics{invalid: map[domain.Reason]int{}}
	h := application.NewHandler(spoofTranslator{}, k, d, &fakeDedup{seen: map[string]bool{}}, m, slog.New(slog.NewTextHandler(io.Discard, nil)))
	require.NoError(t, h.Handle(context.Background(), topicN, batch(1)))
	assert.Empty(t, k.sent)
	assert.Equal(t, domain.ReasonIdentity, d.got[0].Reason)
}
