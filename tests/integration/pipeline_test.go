//go:build integration

package integration_test

import (
	"context"
	"os/exec"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Leitura chega ao banco em menos de 10 s (SLO do design doc, seção 10).
func TestReadingReachesDatabaseWithinSLO(t *testing.T) {
	p := startPipeline(t)
	ops := pool(t, "svc_grafana", pgOpsPwd)
	tenant := randomID("it")

	started := time.Now()
	p.simulate(t, "-tenant", tenant, "-nodes", "1", "-ticks", "1", "-seed", "1").wait(t)

	eventuallyCount(t, ops, tenant, readingsPerBatch, 10*time.Second)
	t.Logf("atraso ponta a ponta: %s", time.Since(started))
}

// 10% de duplicadas e lotes fora de ordem: o banco tem exatamente os 100 lotes distintos.
func TestDuplicatesAndOutOfOrder(t *testing.T) {
	p := startPipeline(t)
	ops := pool(t, "svc_grafana", pgOpsPwd)
	tenant := randomID("it")

	p.simulate(t, "-tenant", tenant, "-nodes", "1", "-ticks", "100", "-speed", "6000",
		"-duplicates", "0.1", "-reorder", "0.1", "-seed", "7").wait(t)

	eventuallyCount(t, ops, tenant, 100*readingsPerBatch, 30*time.Second)
	// espera mais um pouco: duplicadas atrasadas não podem aumentar a contagem
	time.Sleep(3 * time.Second)
	n, err := countReadings(context.Background(), ops, tenant)
	require.NoError(t, err)
	assert.Equal(t, 100*readingsPerBatch, n)
}

// O EMQX reinicia no meio do teste; nenhum lote confirmado se perde.
func TestBrokerRestartLosesNothing(t *testing.T) {
	p := startPipeline(t)
	ops := pool(t, "svc_grafana", pgOpsPwd)
	tenant := randomID("it")

	sim := p.simulate(t, "-tenant", tenant, "-nodes", "3", "-ticks", "100", "-speed", "600", "-seed", "3")
	time.Sleep(2 * time.Second)
	out, err := exec.Command("docker", "compose", "--project-directory", repoRoot(t), "--profile", "core", "restart", "emqx").CombinedOutput()
	require.NoError(t, err, string(out))
	sim.wait(t)

	eventuallyCount(t, ops, tenant, 3*100*readingsPerBatch, 60*time.Second)
}

// Cada tenant só enxerga as próprias leituras (RLS), mesmo com os dois no mesmo pipeline.
func TestTenantIsolation(t *testing.T) {
	p := startPipeline(t)
	ops := pool(t, "svc_grafana", pgOpsPwd)
	owner := pool(t, "garden_owner", pgOwnerPwd)
	a, b := randomID("ita"), randomID("itb")

	for _, tn := range []string{a, b} {
		_, err := owner.Exec(context.Background(), `SELECT telemetry.create_tenant_role($1, 'pw')`, tn)
		require.NoError(t, err)
		p.simulate(t, "-tenant", tn, "-nodes", "2", "-ticks", "3", "-seed", "1").wait(t)
		eventuallyCount(t, ops, tn, 2*3*readingsPerBatch, 15*time.Second)
	}

	asA := pool(t, "tenant_"+a, "pw")
	na, err := countReadings(context.Background(), asA, a)
	require.NoError(t, err)
	assert.Equal(t, 2*3*readingsPerBatch, na)
	nb, err := countReadings(context.Background(), asA, b)
	require.NoError(t, err)
	assert.Zero(t, nb, "tenant A não pode ver leituras do tenant B")
}
