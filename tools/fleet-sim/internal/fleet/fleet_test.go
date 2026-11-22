package fleet_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/JohnnyMatteus/garden-iot/tools/fleet-sim/internal/fleet"
	"github.com/JohnnyMatteus/garden-iot/tools/fleet-sim/internal/profile"
)

type msg struct {
	topic string
	seq   uint32
}

type recorder struct {
	mu     sync.Mutex
	msgs   []msg
	killed int
	wills  []string
}

type link struct{ r *recorder }

func (r *recorder) Dial(_ context.Context, _ string, willTopic string, _ []byte) (fleet.Link, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.wills = append(r.wills, willTopic)
	return link{r}, nil
}

func (l link) Publish(_ context.Context, topic string, payload []byte) error {
	var p struct{ Seq uint32 }
	_ = json.Unmarshal(payload, &p)
	l.r.mu.Lock()
	defer l.r.mu.Unlock()
	l.r.msgs = append(l.r.msgs, msg{topic, p.Seq})
	return nil
}
func (l link) Kill()               { l.r.mu.Lock(); l.r.killed++; l.r.mu.Unlock() }
func (link) Close(context.Context) {}

func run(t *testing.T, cfg fleet.Config, specs []fleet.NodeSpec) *recorder {
	rec := &recorder{}
	cfg.Interval, cfg.Speed, cfg.Start = time.Minute, 1e9, time.Unix(1732190000, 0)
	require.NoError(t, fleet.New(cfg, rec, slog.New(slog.NewTextHandler(io.Discard, nil))).Run(context.Background(), specs))
	return rec
}

func specs(t *testing.T, name string, n int) []fleet.NodeSpec {
	ps, err := profile.Load()
	require.NoError(t, err)
	var out []fleet.NodeSpec
	for i := 0; i < n; i++ {
		out = append(out, fleet.NodeSpec{Tenant: "t1", Garden: "h1", Node: string(rune('a' + i)), Profile: ps[name]})
	}
	return out
}

func TestSmallFleetPublishesGrowingSequences(t *testing.T) {
	rec := run(t, fleet.Config{Seed: 1, Ticks: 5}, specs(t, "sunny", 10))
	assert.Len(t, rec.msgs, 50)
}

func TestNodeDropKillsLinkAndSkipsBatches(t *testing.T) {
	rec := run(t, fleet.Config{Seed: 1, Ticks: 8}, specs(t, "node-drops", 1))
	assert.Equal(t, 1, rec.killed)
	assert.Len(t, rec.wills, 2, "reconecta depois de cair")
	assert.Len(t, rec.msgs, 6, "dois ticks sem publicar")
}

func TestDuplicatesAndReorderAreInjected(t *testing.T) {
	rec := run(t, fleet.Config{Seed: 5, Ticks: 100, DuplicateRate: 0.2, ReorderRate: 0.2}, specs(t, "sunny", 1))

	counts := map[uint32]int{}
	inOrder := true
	var last uint32
	for _, m := range rec.msgs {
		counts[m.seq]++
		if m.seq < last {
			inOrder = false
		}
		last = m.seq
	}
	assert.Len(t, counts, 100, "nenhum lote se perde")
	dups := 0
	for _, c := range counts {
		if c > 1 {
			dups++
		}
	}
	assert.Positive(t, dups)
	assert.False(t, inOrder)

	keys := make([]int, 0)
	for k := range counts {
		keys = append(keys, int(k))
	}
	sort.Ints(keys)
	assert.Equal(t, 1, keys[0])
}
