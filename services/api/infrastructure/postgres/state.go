package postgres

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/JohnnyMatteus/garden-iot/services/api/domain"
)

type actuatorJSON struct {
	Valve   string    `json:"valve"`
	FlowLpm *float64  `json:"flowLpm,omitempty"`
	At      time.Time `json:"at"`
}

func decodeState(raw []byte) (*domain.ActuatorState, error) {
	if raw == nil {
		return nil, nil
	}
	var j actuatorJSON
	if err := json.Unmarshal(raw, &j); err != nil {
		return nil, err
	}
	return &domain.ActuatorState{Valve: domain.Valve(j.Valve), FlowLpm: j.FlowLpm, At: j.At}, nil
}

func encodeState(s domain.ActuatorState) []byte {
	b, _ := json.Marshal(actuatorJSON{Valve: string(s.Valve), FlowLpm: s.FlowLpm, At: s.At})
	return b
}

func (d *DB) State(ctx context.Context, tenant, gardenID, nodeID string) (st domain.NodeState, err error) {
	st.NodeID = nodeID
	err = d.asTenant(ctx, tenant, func(tx pgx.Tx) error {
		var desired, reported []byte
		if e := tx.QueryRow(ctx, `SELECT desired, reported FROM garden.node_state WHERE garden_id = $1 AND node_id = $2`,
			gardenID, nodeID).Scan(&desired, &reported); e != nil {
			return e
		}
		var e error
		if st.Desired, e = decodeState(desired); e != nil {
			return e
		}
		st.Reported, e = decodeState(reported)
		return e
	})
	return st, mapErr(err)
}

func (d *DB) SetDesired(ctx context.Context, tenant, gardenID, nodeID string, s domain.ActuatorState) error {
	return mapErr(d.asTenant(ctx, tenant, func(tx pgx.Tx) error {
		tag, e := tx.Exec(ctx, `UPDATE garden.node_state SET desired = $3 WHERE garden_id = $1 AND node_id = $2`, gardenID, nodeID, encodeState(s))
		if e == nil && tag.RowsAffected() == 0 {
			return pgx.ErrNoRows
		}
		return e
	}))
}

// SetReported usa a conexão base (svc_api), pois quem reporta é o nó e não um usuário do tenant.
func (d *DB) SetReported(ctx context.Context, tenant, gardenID, nodeID string, s domain.ActuatorState) error {
	tag, err := d.pool.Exec(ctx, `UPDATE garden.node_state SET reported = $4 WHERE tenant_id = $1 AND garden_id = $2 AND node_id = $3`,
		tenant, gardenID, nodeID, encodeState(s))
	if err == nil && tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return mapErr(err)
}

func (d *DB) States(ctx context.Context, tenant, gardenID string) (out []domain.NodeState, err error) {
	err = d.asTenant(ctx, tenant, func(tx pgx.Tx) error {
		rows, qerr := tx.Query(ctx, `SELECT node_id, desired, reported FROM garden.node_state WHERE garden_id = $1`, gardenID)
		if qerr != nil {
			return qerr
		}
		out, qerr = collect(rows, func(r pgx.Rows) (st domain.NodeState, e error) {
			var desired, reported []byte
			if e = r.Scan(&st.NodeID, &desired, &reported); e != nil {
				return
			}
			if st.Desired, e = decodeState(desired); e != nil {
				return
			}
			st.Reported, e = decodeState(reported)
			return
		})
		return qerr
	})
	return out, mapErr(err)
}
