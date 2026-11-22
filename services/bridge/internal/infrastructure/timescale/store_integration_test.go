//go:build integration

package timescale_test

import (
	"context"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/JohnnyMatteus/garden-iot/services/bridge/internal/infrastructure/timescale"
	"github.com/JohnnyMatteus/garden-iot/services/bridge/internal/readings"
)

func setup(t *testing.T) *pgxpool.Pool {
	ctx := context.Background()
	schema, err := filepath.Abs("../../../../../infra/timescaledb/init/10-schema.sql")
	require.NoError(t, err)

	ctr, err := postgres.Run(ctx, "timescale/timescaledb:2.17.2-pg16",
		postgres.WithDatabase("garden"), postgres.WithUsername("owner"), postgres.WithPassword("pw"),
		postgres.WithInitScripts(schema),
		testcontainers.WithWaitStrategy(wait.ForLog("database system is ready to accept connections").WithOccurrence(2).WithStartupTimeout(90*time.Second)))
	require.NoError(t, err)
	t.Cleanup(func() { _ = ctr.Terminate(ctx) })

	owner, err := ctr.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)
	op, err := pgxpool.New(ctx, owner)
	require.NoError(t, err)
	t.Cleanup(op.Close)
	_, err = op.Exec(ctx, `CREATE ROLE svc_writer LOGIN PASSWORD 'w' IN ROLE telemetry_writer`)
	require.NoError(t, err)

	cfg := op.Config().ConnConfig.Copy()
	wp, err := pgxpool.New(ctx, "postgres://svc_writer:w@"+cfg.Host+":"+strconv.Itoa(int(cfg.Port))+"/garden?sslmode=disable")
	require.NoError(t, err)
	t.Cleanup(wp.Close)
	return wp
}

func TestSaveIsIdempotent(t *testing.T) {
	pool := setup(t)
	store := timescale.NewStore(pool)
	at := time.Unix(1732190000, 0).UTC()
	rows := []readings.Reading{
		{Tenant: "t1", Garden: "h1", Node: "n1", Sensor: "soil", Kind: "soil_moisture_vwc_pct", ReadAt: at, Sequence: 1, Value: 50, Unit: "pct", Quality: "ok"},
		{Tenant: "t1", Garden: "h1", Node: "n1", Sensor: "air", Kind: "air_temp", ReadAt: at, Sequence: 1, Value: 23.5, Unit: "celsius", Quality: "ok"},
	}
	require.NoError(t, store.Save(context.Background(), rows))
	require.NoError(t, store.Save(context.Background(), rows), "reprocessar o mesmo lote")

	var n int
	require.NoError(t, pool.QueryRow(context.Background(), `SELECT count(*) FROM telemetry.reading`).Scan(&n))
	assert.Equal(t, 2, n)
}
