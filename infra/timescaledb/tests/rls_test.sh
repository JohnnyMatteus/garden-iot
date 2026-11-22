#!/usr/bin/env bash
# Verifica idempotência e isolamento por tenant (RLS) no TimescaleDB em execução (`make up`).
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../../.." && pwd)"
DB=${POSTGRES_DB:-garden}
OWNER=${POSTGRES_USER:-garden_owner}
run() { # usuário senha sql
  docker compose --project-directory "$ROOT" exec -T -e PGPASSWORD="$2" timescaledb \
    psql -h localhost -U "$1" -d "$DB" -v ON_ERROR_STOP=1 -tA -c "$3"
}
fail() { echo "FALHOU: $*" >&2; exit 1; }
expect() { [[ "$2" == "$3" ]] || fail "$1: esperado '$3', obtido '$2'"; echo "ok: $1"; }

OWNER_PW=${POSTGRES_PASSWORD:-dev-only-change-me}
WRITER_PW=${TELEMETRY_WRITER_PASSWORD:-dev-only-change-me}
OPS_PW=${TELEMETRY_OPS_PASSWORD:-dev-only-change-me}

run "$OWNER" "$OWNER_PW" "TRUNCATE telemetry.reading" >/dev/null
for t in rlstest1 rlstest2; do
  run "$OWNER" "$OWNER_PW" "DROP ROLE IF EXISTS tenant_$t" >/dev/null
  run "$OWNER" "$OWNER_PW" "SELECT telemetry.create_tenant_role('$t', 'pw-$t')" >/dev/null
done

insert() { # tenant
  run svc_telemetry_writer "$WRITER_PW" "INSERT INTO telemetry.reading
    (tenant_id, garden_id, node_id, sensor_id, kind, read_at, sequence, value, unit)
    SELECT '$1', 'h1', 'n1', 'soil-1', 'soil_moisture_vwc_pct', now() - (g || ' seconds')::interval, 10, 50, '%'
    FROM generate_series(1, 5) g ON CONFLICT DO NOTHING"
}

insert rlstest1 >/dev/null
insert rlstest2 >/dev/null
expect "5 leituras por tenant gravadas" "$(run "$OWNER" "$OWNER_PW" "SELECT count(*) FROM telemetry.reading WHERE tenant_id = 'rlstest1'")" "5"

# Instante fixo para provar o ON CONFLICT:
run svc_telemetry_writer "$WRITER_PW" "INSERT INTO telemetry.reading
  (tenant_id, garden_id, node_id, sensor_id, kind, read_at, sequence, value, unit)
  VALUES ('rlstest1','h1','n1','soil-1','soil_moisture_vwc_pct','2026-01-01T00:00:00Z',1,1,'%')
  ON CONFLICT DO NOTHING" >/dev/null
run svc_telemetry_writer "$WRITER_PW" "INSERT INTO telemetry.reading
  (tenant_id, garden_id, node_id, sensor_id, kind, read_at, sequence, value, unit)
  VALUES ('rlstest1','h1','n1','soil-1','soil_moisture_vwc_pct','2026-01-01T00:00:00Z',1,1,'%')
  ON CONFLICT DO NOTHING" >/dev/null
expect "reprocessamento não duplica" "$(run "$OWNER" "$OWNER_PW" "SELECT count(*) FROM telemetry.reading WHERE read_at = '2026-01-01T00:00:00Z'")" "1"

expect "tenant 1 só vê o próprio" "$(run tenant_rlstest1 pw-rlstest1 "SELECT string_agg(DISTINCT tenant_id, ',') FROM telemetry.reading")" "rlstest1"
expect "tenant 2 só vê o próprio" "$(run tenant_rlstest2 pw-rlstest2 "SELECT string_agg(DISTINCT tenant_id, ',') FROM telemetry.reading")" "rlstest2"
expect "operação vê os dois" "$(run svc_grafana "$OPS_PW" "SELECT count(DISTINCT tenant_id) FROM telemetry.reading")" "2"

if run tenant_rlstest1 pw-rlstest1 "DELETE FROM telemetry.reading" >/dev/null 2>&1; then fail "tenant não pode apagar"; fi
echo "ok: tenant sem permissão de escrita"

run "$OWNER" "$OWNER_PW" "TRUNCATE telemetry.reading" >/dev/null
for t in rlstest1 rlstest2; do run "$OWNER" "$OWNER_PW" "DROP ROLE tenant_$t" >/dev/null; done
echo "RLS ok"
