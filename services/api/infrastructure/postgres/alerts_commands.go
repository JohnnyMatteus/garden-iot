package postgres

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/JohnnyMatteus/garden-iot/services/api/domain"
)

func (d *DB) List(ctx context.Context, tenant, gardenID string, onlyOpen bool) (out []domain.Alert, err error) {
	err = d.asTenant(ctx, tenant, func(tx pgx.Tx) error {
		rows, qerr := tx.Query(ctx, `SELECT id::text, garden_id, node_id, severity, kind, message, raised_at, acknowledged_at
			FROM garden.alert WHERE garden_id = $1 AND (NOT $2 OR acknowledged_at IS NULL) ORDER BY raised_at DESC LIMIT 200`, gardenID, onlyOpen)
		if qerr != nil {
			return qerr
		}
		out, qerr = collect(rows, func(r pgx.Rows) (a domain.Alert, e error) {
			e = r.Scan(&a.ID, &a.GardenID, &a.NodeID, &a.Severity, &a.Kind, &a.Message, &a.RaisedAt, &a.AcknowledgedAt)
			return
		})
		return qerr
	})
	return out, mapErr(err)
}

func (d *DB) Acknowledge(ctx context.Context, tenant, gardenID, alertID string, at time.Time) error {
	return mapErr(d.asTenant(ctx, tenant, func(tx pgx.Tx) error {
		tag, e := tx.Exec(ctx, `UPDATE garden.alert SET acknowledged_at = $3 WHERE garden_id = $1 AND id::text = $2 AND acknowledged_at IS NULL`, gardenID, alertID, at)
		if e == nil && tag.RowsAffected() == 0 {
			return pgx.ErrNoRows
		}
		return e
	}))
}

func (d *DB) Create(ctx context.Context, tenant string, c domain.Command) error {
	return mapErr(d.asTenant(ctx, tenant, func(tx pgx.Tx) error {
		_, e := tx.Exec(ctx, `INSERT INTO garden.command (id, tenant_id, garden_id, node_id, action, duration_seconds, status, requested_by, created_at, expires_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
			c.ID, tenant, c.GardenID, c.NodeID, c.Action, c.DurationSeconds, string(c.Status), c.RequestedBy, c.CreatedAt, c.ExpiresAt)
		return e
	}))
}

func (d *DB) Get(ctx context.Context, tenant, gardenID, nodeID, commandID string) (c domain.Command, err error) {
	err = d.asTenant(ctx, tenant, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `SELECT id::text, garden_id, node_id, action, duration_seconds, status, requested_by, created_at, expires_at
			FROM garden.command WHERE garden_id = $1 AND node_id = $2 AND id::text = $3`, gardenID, nodeID, commandID).
			Scan(&c.ID, &c.GardenID, &c.NodeID, &c.Action, &c.DurationSeconds, &c.Status, &c.RequestedBy, &c.CreatedAt, &c.ExpiresAt)
	})
	return c, mapErr(err)
}

func (d *DB) HasOpen(ctx context.Context, tenant, gardenID, nodeID string) (open bool, err error) {
	err = d.asTenant(ctx, tenant, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM garden.command
			WHERE garden_id = $1 AND node_id = $2 AND status IN ('pending', 'sent') AND expires_at > now())`, gardenID, nodeID).Scan(&open)
	})
	return open, mapErr(err)
}

// SetStatus vem do fluxo do usuário (criação) e do MQTT (resposta do nó): usa a conexão base,
// restrita ao tenant informado, e nunca regride um comando já encerrado.
func (d *DB) SetStatus(ctx context.Context, tenant, commandID string, s domain.CommandStatus) error {
	tag, err := d.pool.Exec(ctx, `UPDATE garden.command SET status = $3
		WHERE tenant_id = $1 AND id::text = $2 AND status IN ('pending', 'sent')`, tenant, commandID, string(s))
	if err == nil && tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return mapErr(err)
}

func (d *DB) ExpireBefore(ctx context.Context, at time.Time) (int, error) {
	tag, err := d.pool.Exec(ctx, `UPDATE garden.command SET status = 'expired' WHERE status IN ('pending', 'sent') AND expires_at <= $1`, at)
	return int(tag.RowsAffected()), mapErr(err)
}

func (d *DB) Recent(ctx context.Context, tenant, gardenID, nodeID string, limit int) (out []domain.Command, err error) {
	err = d.asTenant(ctx, tenant, func(tx pgx.Tx) error {
		rows, qerr := tx.Query(ctx, `SELECT id::text, garden_id, node_id, action, duration_seconds, status, requested_by, created_at, expires_at
			FROM garden.command WHERE garden_id = $1 AND ($2::text IS NULL OR node_id = $2) ORDER BY created_at DESC LIMIT $3`,
			gardenID, nullText(nodeID), limit)
		if qerr != nil {
			return qerr
		}
		out, qerr = collect(rows, func(r pgx.Rows) (c domain.Command, e error) {
			e = r.Scan(&c.ID, &c.GardenID, &c.NodeID, &c.Action, &c.DurationSeconds, &c.Status, &c.RequestedBy, &c.CreatedAt, &c.ExpiresAt)
			return
		})
		return qerr
	})
	return out, mapErr(err)
}
