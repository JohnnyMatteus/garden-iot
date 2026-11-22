-- Schema de telemetria do garden-iot (M1-10). Executado pelo entrypoint do Postgres
-- na criação do volume, como dono do banco (POSTGRES_USER).
CREATE EXTENSION IF NOT EXISTS timescaledb;

CREATE SCHEMA telemetry;

-- Uma linha por leitura convertida para grandeza de domínio.
CREATE TABLE telemetry.reading (
    tenant_id   text             NOT NULL,
    garden_id   text             NOT NULL,
    node_id     text             NOT NULL,
    sensor_id   text             NOT NULL,
    kind        text             NOT NULL,
    read_at     timestamptz      NOT NULL,
    sequence    bigint           NOT NULL,
    value       double precision NOT NULL,
    unit        text             NOT NULL,
    quality     text             NOT NULL DEFAULT 'ok' CHECK (quality IN ('ok', 'estimated', 'suspect')),
    ingested_at timestamptz      NOT NULL DEFAULT now(),
    -- Chave natural: reprocessar o mesmo lote não duplica linhas (INSERT ... ON CONFLICT DO NOTHING).
    PRIMARY KEY (tenant_id, garden_id, node_id, sensor_id, kind, read_at, sequence)
);

SELECT create_hypertable('telemetry.reading', by_range('read_at', INTERVAL '1 day'));

CREATE INDEX reading_ingested_at_idx ON telemetry.reading (ingested_at DESC);
CREATE INDEX reading_node_kind_idx ON telemetry.reading (tenant_id, node_id, kind, read_at DESC);

-- Isolamento por tenant com RLS (ADR 0010).
ALTER TABLE telemetry.reading ENABLE ROW LEVEL SECURITY;
ALTER TABLE telemetry.reading FORCE ROW LEVEL SECURITY;

-- Papéis de aplicação. Papéis de tenant são membros de telemetry_tenant e se chamam tenant_<id>.
CREATE ROLE telemetry_writer NOLOGIN;
CREATE ROLE telemetry_ops NOLOGIN;
CREATE ROLE telemetry_tenant NOLOGIN;

GRANT USAGE ON SCHEMA telemetry TO telemetry_writer, telemetry_ops, telemetry_tenant;
GRANT INSERT, SELECT ON telemetry.reading TO telemetry_writer;
GRANT SELECT ON telemetry.reading TO telemetry_ops, telemetry_tenant;

CREATE FUNCTION telemetry.current_tenant() RETURNS text
    LANGUAGE sql STABLE PARALLEL SAFE
    AS $$ SELECT regexp_replace(current_user, '^tenant_', '') $$;

-- Escrita do consumidor: atravessa tenants (o tenant vem da mensagem, validado antes).
CREATE POLICY writer_all ON telemetry.reading TO telemetry_writer USING (true) WITH CHECK (true);
-- Operação (Grafana): leitura de todos os tenants, sem escrita.
CREATE POLICY ops_read ON telemetry.reading FOR SELECT TO telemetry_ops USING (true);
-- Tenant: só as próprias linhas.
CREATE POLICY tenant_read ON telemetry.reading FOR SELECT TO telemetry_tenant
    USING (tenant_id = telemetry.current_tenant());

-- Cria o papel de login de um tenant (usado no onboarding e nos testes).
CREATE FUNCTION telemetry.create_tenant_role(p_tenant text, p_password text) RETURNS void
    LANGUAGE plpgsql
    AS $$
BEGIN
    IF p_tenant !~ '^[a-z0-9-]{1,32}$' THEN
        RAISE EXCEPTION 'tenant inválido: %', p_tenant;
    END IF;
    EXECUTE format('CREATE ROLE %I LOGIN PASSWORD %L IN ROLE telemetry_tenant', 'tenant_' || p_tenant, p_password);
END
$$;
REVOKE ALL ON FUNCTION telemetry.create_tenant_role(text, text) FROM PUBLIC;
