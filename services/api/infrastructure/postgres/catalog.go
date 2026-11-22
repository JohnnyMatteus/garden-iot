package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"

	"github.com/JohnnyMatteus/garden-iot/services/api/domain"
)

func collect[T any](rows pgx.Rows, scan func(pgx.Rows) (T, error)) ([]T, error) {
	defer rows.Close()
	out := []T{}
	for rows.Next() {
		v, err := scan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func (d *DB) ListGardens(ctx context.Context, tenant string) (out []domain.Garden, err error) {
	err = d.asTenant(ctx, tenant, func(tx pgx.Tx) error {
		rows, qerr := tx.Query(ctx, `SELECT id, name, created_at FROM garden.garden ORDER BY id`)
		if qerr != nil {
			return qerr
		}
		out, qerr = collect(rows, func(r pgx.Rows) (g domain.Garden, e error) {
			e = r.Scan(&g.ID, &g.Name, &g.CreatedAt)
			return
		})
		return qerr
	})
	return out, mapErr(err)
}

func (d *DB) GetGarden(ctx context.Context, tenant, id string) (g domain.Garden, err error) {
	err = d.asTenant(ctx, tenant, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `SELECT id, name, created_at FROM garden.garden WHERE id = $1`, id).Scan(&g.ID, &g.Name, &g.CreatedAt)
	})
	return g, mapErr(err)
}

func (d *DB) CreateGarden(ctx context.Context, tenant string, g domain.Garden) (out domain.Garden, err error) {
	err = d.asTenant(ctx, tenant, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `INSERT INTO garden.garden (tenant_id, id, name) VALUES ($1, $2, $3)
			RETURNING id, name, created_at`, tenant, g.ID, g.Name).Scan(&out.ID, &out.Name, &out.CreatedAt)
	})
	return out, mapErr(err)
}

func (d *DB) ListBeds(ctx context.Context, tenant, gardenID string) (out []domain.Bed, err error) {
	err = d.asTenant(ctx, tenant, func(tx pgx.Tx) error {
		rows, qerr := tx.Query(ctx, `SELECT id, garden_id, name, crop, irrigation FROM garden.bed WHERE garden_id = $1 ORDER BY id`, gardenID)
		if qerr != nil {
			return qerr
		}
		out, qerr = collect(rows, func(r pgx.Rows) (b domain.Bed, e error) {
			e = r.Scan(&b.ID, &b.GardenID, &b.Name, &b.Crop, &b.Irrigation)
			return
		})
		return qerr
	})
	return out, mapErr(err)
}

func (d *DB) GetBed(ctx context.Context, tenant, gardenID, bedID string) (b domain.Bed, err error) {
	err = d.asTenant(ctx, tenant, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `SELECT id, garden_id, name, crop, irrigation FROM garden.bed WHERE garden_id = $1 AND id = $2`,
			gardenID, bedID).Scan(&b.ID, &b.GardenID, &b.Name, &b.Crop, &b.Irrigation)
	})
	return b, mapErr(err)
}

func (d *DB) CreateBed(ctx context.Context, tenant string, b domain.Bed) (domain.Bed, error) {
	err := d.asTenant(ctx, tenant, func(tx pgx.Tx) error {
		_, e := tx.Exec(ctx, `INSERT INTO garden.bed (tenant_id, garden_id, id, name, crop, irrigation) VALUES ($1, $2, $3, $4, $5, $6)`,
			tenant, b.GardenID, b.ID, b.Name, b.Crop, string(b.Irrigation))
		return e
	})
	return b, mapErr(err)
}

const nodeCols = `id, garden_id, kind, bed_id, fw_version, last_seen_at`

func scanNode(r pgx.Row) (n domain.Node, err error) {
	err = r.Scan(&n.ID, &n.GardenID, &n.Kind, &n.BedID, &n.FwVersion, &n.LastSeenAt)
	return
}

func (d *DB) ListNodes(ctx context.Context, tenant, gardenID string) (out []domain.Node, err error) {
	err = d.asTenant(ctx, tenant, func(tx pgx.Tx) error {
		rows, qerr := tx.Query(ctx, `SELECT `+nodeCols+` FROM garden.node WHERE garden_id = $1 ORDER BY id`, gardenID)
		if qerr != nil {
			return qerr
		}
		out, qerr = collect(rows, func(r pgx.Rows) (domain.Node, error) { return scanNode(r) })
		return qerr
	})
	return out, mapErr(err)
}

func (d *DB) GetNode(ctx context.Context, tenant, gardenID, nodeID string) (n domain.Node, err error) {
	err = d.asTenant(ctx, tenant, func(tx pgx.Tx) error {
		var e error
		n, e = scanNode(tx.QueryRow(ctx, `SELECT `+nodeCols+` FROM garden.node WHERE garden_id = $1 AND id = $2`, gardenID, nodeID))
		return e
	})
	return n, mapErr(err)
}

func (d *DB) CreateNode(ctx context.Context, tenant string, n domain.Node) (domain.Node, error) {
	err := d.asTenant(ctx, tenant, func(tx pgx.Tx) error {
		if _, e := tx.Exec(ctx, `INSERT INTO garden.node (tenant_id, garden_id, id, kind, bed_id) VALUES ($1, $2, $3, $4, $5)`,
			tenant, n.GardenID, n.ID, string(n.Kind), n.BedID); e != nil {
			return e
		}
		_, e := tx.Exec(ctx, `INSERT INTO garden.node_state (tenant_id, garden_id, node_id) VALUES ($1, $2, $3)`, tenant, n.GardenID, n.ID)
		return e
	})
	return n, mapErr(err)
}
