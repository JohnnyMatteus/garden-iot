//go:build integration

package postgres_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/JohnnyMatteus/garden-iot/services/api/application"
	"github.com/JohnnyMatteus/garden-iot/services/api/domain"
	"github.com/JohnnyMatteus/garden-iot/services/api/infrastructure/postgres"
)

func envOr(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}

// Exige `make up`. Prova que a API, conectada como svc_api e assumindo tenant_<id>, só enxerga o próprio tenant.
func TestTenantIsolationThroughRLS(t *testing.T) {
	ctx := context.Background()
	host := envOr("TIMESCALEDB_ADDR", "localhost:5432")
	owner, err := pgx.Connect(ctx, "postgres://garden_owner:"+envOr("POSTGRES_PASSWORD", "dev-only-change-me")+"@"+host+"/garden")
	if err != nil {
		t.Fatal(err)
	}
	defer owner.Close(ctx)

	const other = "apitest"
	_, _ = owner.Exec(ctx, "DROP ROLE IF EXISTS tenant_"+other)
	if _, err := owner.Exec(ctx, "SELECT telemetry.create_tenant_role($1, 'x')", other); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = owner.Exec(ctx, "DELETE FROM telemetry.reading WHERE tenant_id = $1", other)
		_, _ = owner.Exec(ctx, "DELETE FROM garden.node_state WHERE tenant_id = $1", other)
		_, _ = owner.Exec(ctx, "DELETE FROM garden.node WHERE tenant_id = $1", other)
		_, _ = owner.Exec(ctx, "DELETE FROM garden.garden WHERE tenant_id = $1", other)
		_, _ = owner.Exec(ctx, "DROP ROLE IF EXISTS tenant_"+other)
	})

	db, err := postgres.Connect(ctx, "postgres://svc_api:"+envOr("GARDEN_API_PASSWORD", "dev-only-change-me")+"@"+host+"/garden")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if _, err := db.CreateGarden(ctx, other, domain.Garden{ID: "horta-x", Name: "Outra"}); err != nil {
		t.Fatalf("criar horta do outro tenant: %v", err)
	}
	if _, err := db.CreateGarden(ctx, other, domain.Garden{ID: "horta-x", Name: "Outra"}); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("duplicada deveria dar conflito, veio %v", err)
	}
	gardens, err := db.ListGardens(ctx, other)
	if err != nil || len(gardens) != 1 || gardens[0].ID != "horta-x" {
		t.Fatalf("tenant %s vê %v (%v)", other, gardens, err)
	}
	demo, err := db.ListGardens(ctx, "demo")
	if err != nil {
		t.Fatal(err)
	}
	for _, g := range demo {
		if g.ID == "horta-x" {
			t.Fatal("tenant demo enxergou horta de outro tenant")
		}
	}
	if _, err := db.GetGarden(ctx, "demo", "horta-x"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("esperava não encontrado, veio %v", err)
	}

	now := time.Now().UTC()
	for _, tenant := range []string{"demo", other} {
		if _, err := owner.Exec(ctx, `INSERT INTO telemetry.reading (tenant_id, garden_id, node_id, sensor_id, kind, read_at, sequence, value, unit)
			VALUES ($1, $2, 'n1', 's1', 'apitest_kind', $3, 1, 7, 'u') ON CONFLICT DO NOTHING`, tenant, map[string]string{"demo": "horta-1", other: "horta-x"}[tenant], now); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() { _, _ = owner.Exec(ctx, "DELETE FROM telemetry.reading WHERE kind = 'apitest_kind'") })

	// Mesmo consultando a horta do outro tenant, o RLS não devolve nada.
	rs, err := db.History(ctx, "demo", application.ReadingQuery{GardenID: "horta-x", NodeID: "n1", Kind: "apitest_kind", Limit: 10})
	if err != nil || len(rs) != 0 {
		t.Fatalf("vazamento entre tenants: %v (%v)", rs, err)
	}
	rs, err = db.History(ctx, other, application.ReadingQuery{GardenID: "horta-x", NodeID: "n1", Kind: "apitest_kind", Limit: 10})
	if err != nil || len(rs) != 1 {
		t.Fatalf("tenant não vê as próprias leituras: %v (%v)", rs, err)
	}
}
