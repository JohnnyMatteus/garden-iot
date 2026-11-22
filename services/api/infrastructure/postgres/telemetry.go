package postgres

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/JohnnyMatteus/garden-iot/services/api/application"
	"github.com/JohnnyMatteus/garden-iot/services/api/domain"
)

const readingCols = `node_id, sensor_id, kind, read_at, value, unit, quality`

func scanReading(r pgx.Rows) (rd domain.Reading, err error) {
	err = r.Scan(&rd.NodeID, &rd.SensorID, &rd.Kind, &rd.ReadAt, &rd.Value, &rd.Unit, &rd.Quality)
	return
}

func (d *DB) History(ctx context.Context, tenant string, q application.ReadingQuery) (out []domain.Reading, err error) {
	err = d.asTenant(ctx, tenant, func(tx pgx.Tx) error {
		// NULL desliga o filtro opcional; o plano ainda usa o índice (tenant, node, kind, read_at).
		rows, qerr := tx.Query(ctx, `SELECT `+readingCols+` FROM telemetry.reading
			WHERE garden_id = $1 AND node_id = $2
			  AND ($3::text IS NULL OR kind = $3)
			  AND ($4::timestamptz IS NULL OR read_at >= $4)
			  AND ($5::timestamptz IS NULL OR read_at <= $5)
			ORDER BY read_at DESC LIMIT $6`,
			q.GardenID, q.NodeID, nullText(q.Kind), nullTime(q.From), nullTime(q.To), q.Limit)
		if qerr != nil {
			return qerr
		}
		out, qerr = collect(rows, scanReading)
		return qerr
	})
	return out, mapErr(err)
}

func (d *DB) Latest(ctx context.Context, tenant, gardenID, kind string, since time.Time) (rd *domain.Reading, err error) {
	err = d.asTenant(ctx, tenant, func(tx pgx.Tx) error {
		rows, qerr := tx.Query(ctx, `SELECT `+readingCols+` FROM telemetry.reading
			WHERE garden_id = $1 AND kind = $2 AND read_at >= $3 ORDER BY read_at DESC LIMIT 1`, gardenID, kind, since)
		if qerr != nil {
			return qerr
		}
		list, qerr := collect(rows, scanReading)
		if qerr == nil && len(list) == 1 {
			rd = &list[0]
		}
		return qerr
	})
	return rd, mapErr(err)
}

func (d *DB) IngestedAfter(ctx context.Context, tenant, gardenID string, after time.Time, limit int) (out []application.IngestedReading, err error) {
	err = d.asTenant(ctx, tenant, func(tx pgx.Tx) error {
		rows, qerr := tx.Query(ctx, `SELECT `+readingCols+`, ingested_at FROM telemetry.reading
			WHERE garden_id = $1 AND ingested_at > $2 ORDER BY ingested_at LIMIT $3`, gardenID, after, limit)
		if qerr != nil {
			return qerr
		}
		out, qerr = collect(rows, func(r pgx.Rows) (ir application.IngestedReading, e error) {
			e = r.Scan(&ir.NodeID, &ir.SensorID, &ir.Kind, &ir.ReadAt, &ir.Value, &ir.Unit, &ir.Quality, &ir.IngestedAt)
			return
		})
		return qerr
	})
	return out, mapErr(err)
}

func nullText(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func nullTime(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	return &t
}
