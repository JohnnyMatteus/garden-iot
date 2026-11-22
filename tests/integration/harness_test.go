//go:build integration

// Pacote de integração ponta a ponta (M1-13). Exige o ambiente local no ar (`make up`, ou o job do CI):
// EMQX com mTLS, Kafka, Schema Registry e TimescaleDB. Os serviços sob teste (bridge, persist e
// fleet-sim) são compilados e executados como processos, falando com a infraestrutura real.
package integration_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

var (
	mqttURL    = env("IT_MQTT_URL", "tls://localhost:8883")
	kafkaAddr  = env("IT_KAFKA_BROKERS", "localhost:29092")
	registry   = env("IT_SCHEMA_REGISTRY_URL", "http://localhost:8081")
	pgHost     = env("IT_PG_ADDR", "localhost:5432")
	pgDB       = env("IT_PG_DB", "garden")
	pgOwnerPwd = env("IT_PG_OWNER_PASSWORD", "dev-only-change-me")
	pgOpsPwd   = env("IT_PG_OPS_PASSWORD", "dev-only-change-me")
)

func repoRoot(t *testing.T) string {
	root, err := filepath.Abs("../..")
	require.NoError(t, err)
	return root
}

func pkiDir(t *testing.T) string {
	return env("IT_PKI_DIR", filepath.Join(repoRoot(t), "infra/pki/out"))
}

func randomID(prefix string) string {
	b := make([]byte, 3)
	_, _ = rand.Read(b)
	return prefix + hex.EncodeToString(b)
}

// build compila um comando Go do módulo dado e devolve o caminho do binário.
func build(t *testing.T, module, pkg string) string {
	t.Helper()
	out := filepath.Join(t.TempDir(), filepath.Base(pkg))
	cmd := exec.Command("go", "build", "-o", out, pkg)
	cmd.Dir = filepath.Join(repoRoot(t), module)
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build de %s: %v\n%s", pkg, err, b)
	}
	return out
}

type process struct {
	cmd *exec.Cmd
	out *bytes.Buffer
}

func start(t *testing.T, bin string, env []string, args ...string) *process {
	t.Helper()
	p := &process{cmd: exec.Command(bin, args...), out: &bytes.Buffer{}}
	p.cmd.Env = append(os.Environ(), env...)
	p.cmd.Stdout, p.cmd.Stderr = p.out, p.out
	require.NoError(t, p.cmd.Start())
	t.Cleanup(func() {
		_ = p.cmd.Process.Signal(os.Interrupt)
		done := make(chan struct{})
		go func() { _ = p.cmd.Wait(); close(done) }()
		select {
		case <-done:
		case <-time.After(10 * time.Second):
			_ = p.cmd.Process.Kill()
		}
		if t.Failed() {
			t.Logf("saída de %s:\n%s", filepath.Base(bin), p.out.String())
		}
	})
	return p
}

// run executa até o fim (usado no simulador, que termina sozinho).
func (p *process) wait(t *testing.T) {
	t.Helper()
	require.NoError(t, p.cmd.Wait(), "saída:\n%s", p.out.String())
}

type pipeline struct{ pki string }

// O ACL só permite a assinatura compartilhada $share/bridge, então todo bridge do ambiente (inclusive o
// container do compose) divide o mesmo grupo. A sessão MQTT dura 1h depois do desligamento (é o que
// protege as mensagens em QoS 1 num reinício). Com um client id novo por teste, a sessão do teste
// anterior ficava no EMQX como assinante do grupo e retinha a sua parcela das mensagens sem que
// ninguém as lesse. Um client id fixo faz o bridge do próximo teste retomar a mesma sessão.
const bridgeClientID = "bridge-integration-test"

// Endereços fixos para a leitura de métricas no diagnóstico de falha.
const (
	bridgeHTTP  = "127.0.0.1:19102"
	persistHTTP = "127.0.0.1:19103"
)

// startPipeline sobe bridge e persist com um grupo de consumo novo.
func startPipeline(t *testing.T) pipeline {
	t.Helper()
	pki := pkiDir(t)
	if _, err := os.Stat(filepath.Join(pki, "bridge.pem")); err != nil {
		t.Skipf("PKI ausente em %s (rode make up): %v", pki, err)
	}
	bridgeBin := build(t, "services/bridge", "./cmd/bridge")
	persistBin := build(t, "services/bridge", "./cmd/persist")

	start(t, bridgeBin, []string{
		"BRIDGE_MQTT_URL=" + mqttURL, "BRIDGE_MQTT_CLIENT_ID=" + bridgeClientID, "BRIDGE_MQTT_SHARE_GROUP=bridge",
		"BRIDGE_TLS_CA=" + filepath.Join(pki, "ca.pem"), "BRIDGE_TLS_CERT=" + filepath.Join(pki, "bridge.pem"),
		"BRIDGE_TLS_KEY=" + filepath.Join(pki, "bridge.key"),
		"BRIDGE_KAFKA_BROKERS=" + kafkaAddr, "BRIDGE_SCHEMA_REGISTRY_URL=" + registry, "BRIDGE_HTTP_ADDR=" + bridgeHTTP,
	})
	start(t, persistBin, []string{
		"PERSIST_KAFKA_BROKERS=" + kafkaAddr, "PERSIST_KAFKA_GROUP=" + randomID("persist-"),
		"PERSIST_DATABASE_URL=" + writerDSN(), "PERSIST_HTTP_ADDR=" + persistHTTP,
	})
	time.Sleep(3 * time.Second) // tempo para as assinaturas ficarem ativas
	return pipeline{pki: pki}
}

func writerDSN() string {
	return fmt.Sprintf("postgres://svc_telemetry_writer:%s@%s/%s", env("IT_PG_WRITER_PASSWORD", "dev-only-change-me"), pgHost, pgDB)
}

// simulate roda o simulador de frota até terminar.
func (p pipeline) simulate(t *testing.T, args ...string) *process {
	t.Helper()
	sim := build(t, "tools/fleet-sim", "./cmd/fleet-sim")
	base := []string{"-broker", mqttURL, "-ca", filepath.Join(p.pki, "ca.pem"),
		"-cert", filepath.Join(p.pki, "fleet-sim.pem"), "-key", filepath.Join(p.pki, "fleet-sim.key")}
	return start(t, sim, nil, append(base, args...)...)
}

func pool(t *testing.T, user, password string) *pgxpool.Pool {
	t.Helper()
	pl, err := pgxpool.New(context.Background(), fmt.Sprintf("postgres://%s:%s@%s/%s", user, password, pgHost, pgDB))
	require.NoError(t, err)
	t.Cleanup(pl.Close)
	return pl
}

func countReadings(ctx context.Context, pl *pgxpool.Pool, tenant string) (int, error) {
	var n int
	err := pl.QueryRow(ctx, `SELECT count(*) FROM telemetry.reading WHERE tenant_id = $1`, tenant).Scan(&n)
	return n, err
}

// readingsPerBatch é quanto o perfil "sunny" gera por lote depois da ACL:
// solo, temperatura, umidade, lux, radiação estimada e nível de água.
const readingsPerBatch = 6

func eventuallyCount(t *testing.T, pl *pgxpool.Pool, tenant string, want int, within time.Duration) {
	t.Helper()
	ok := assert.Eventually(t, func() bool {
		n, err := countReadings(context.Background(), pl, tenant)
		return err == nil && n == want
	}, within, 250*time.Millisecond, "leituras de %s no banco", tenant)
	if !ok {
		diagnose(t, pl, tenant, want)
		t.FailNow()
	}
}

// diagnose diz onde as mensagens pararam: banco, bridge (MQTT para Kafka), persist (Kafka para banco)
// e assinantes no EMQX. Não imprime ambiente, credenciais nem chaves.
func diagnose(t *testing.T, pl *pgxpool.Pool, tenant string, want int) {
	t.Helper()
	n, err := countReadings(context.Background(), pl, tenant)
	t.Logf("diagnóstico: banco tem %d de %d leituras esperadas de %s (erro: %v)", n, want, tenant, err)
	t.Logf("diagnóstico: métricas do bridge:\n%s", metricLines(bridgeHTTP, "bridge_messages_", "bridge_processing_"))
	t.Logf("diagnóstico: métricas do persist:\n%s", metricLines(persistHTTP, "persist_"))
	root := repoRoot(t)
	for _, args := range [][]string{{"subscriptions", "list"}, {"clients", "list"}} {
		cmd := append([]string{"compose", "--project-directory", root, "--profile", "core", "exec", "-T", "emqx", "/opt/emqx/bin/emqx", "ctl"}, args...)
		out, _ := exec.Command("docker", cmd...).CombinedOutput()
		t.Logf("diagnóstico: emqx ctl %s:\n%s", args[0], limit(string(out), 3000))
	}
}

func metricLines(addr string, prefixes ...string) string {
	c := &http.Client{Timeout: 3 * time.Second}
	resp, err := c.Get("http://" + addr + "/metrics")
	if err != nil {
		return "indisponível: " + err.Error()
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	var keep []string
	for _, l := range strings.Split(string(body), "\n") {
		for _, p := range prefixes {
			if strings.HasPrefix(l, p) {
				keep = append(keep, l)
			}
		}
	}
	return limit(strings.Join(keep, "\n"), 3000)
}

func limit(s string, n int) string {
	if len(s) > n {
		return s[:n] + "..."
	}
	return s
}
