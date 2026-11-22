// Package timescale grava leituras em telemetry.reading de forma idempotente.
package timescale

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/JohnnyMatteus/garden-iot/services/bridge/internal/readings"
)

type Store struct{ pool *pgxpool.Pool }

func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

// insert usa unnest para gravar o lote inteiro num único comando; a chave natural da tabela
// faz o reprocessamento (redelivery do Kafka) não duplicar linhas.
const insert = `
INSERT INTO telemetry.reading (tenant_id, garden_id, node_id, sensor_id, kind, read_at, sequence, value, unit, quality)
SELECT * FROM unnest($1::text[], $2::text[], $3::text[], $4::text[], $5::text[], $6::timestamptz[], $7::bigint[], $8::float8[], $9::text[], $10::text[])
ON CONFLICT DO NOTHING`

func (s *Store) Save(ctx context.Context, rows []readings.Reading) error {
	n := len(rows)
	var (
		tenant, garden, node, sensor, kind, unit, quality = make([]string, n), make([]string, n), make([]string, n), make([]string, n), make([]string, n), make([]string, n), make([]string, n)
		readAt                                            = make([]time.Time, n)
		seq                                               = make([]int64, n)
		value                                             = make([]float64, n)
	)
	for i, r := range rows {
		tenant[i], garden[i], node[i], sensor[i], kind[i], unit[i], quality[i] = r.Tenant, r.Garden, r.Node, r.Sensor, r.Kind, r.Unit, r.Quality
		readAt[i], seq[i], value[i] = r.ReadAt, int64(r.Sequence), r.Value
	}
	if _, err := s.pool.Exec(ctx, insert, tenant, garden, node, sensor, kind, readAt, seq, value, unit, quality); err != nil {
		return fmt.Errorf("insert de %d leituras: %w", n, err)
	}
	return nil
}
