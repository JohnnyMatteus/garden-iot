package dedup_test

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/JohnnyMatteus/garden-iot/services/bridge/internal/application"
	"github.com/JohnnyMatteus/garden-iot/services/bridge/internal/infrastructure/dedup"
)

func contract(t *testing.T, d application.Deduplicator, advance func(time.Duration)) {
	ctx := context.Background()
	ok, err := d.Claim(ctx, "a")
	require.NoError(t, err)
	assert.True(t, ok)

	ok, _ = d.Claim(ctx, "a")
	assert.False(t, ok, "segunda reserva do mesmo id")

	ok, _ = d.Claim(ctx, "b")
	assert.True(t, ok, "id diferente")

	require.NoError(t, d.Release(ctx, "a"))
	ok, _ = d.Claim(ctx, "a")
	assert.True(t, ok, "após release")

	advance(2 * time.Minute)
	ok, _ = d.Claim(ctx, "b")
	assert.True(t, ok, "após a janela")
}

func TestMemory(t *testing.T) {
	m := dedup.NewMemory(time.Minute)
	clock := time.Now()
	dedup.SetClock(m, func() time.Time { return clock })
	contract(t, m, func(d time.Duration) { clock = clock.Add(d) })
}

func TestRedis(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	contract(t, dedup.NewRedis(client, time.Minute), mr.FastForward)
}
