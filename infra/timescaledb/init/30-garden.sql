-- Catálogo, gêmeo digital, alertas e comandos usados pela API (ADR 0011).
-- Mesmo isolamento por tenant da telemetria (ADR 0010): papéis tenant_<id> via RLS.
CREATE SCHEMA garden;
GRANT USAGE ON SCHEMA garden TO telemetry_tenant;

CREATE TABLE garden.garden (
    tenant_id  text NOT NULL,
    id         text NOT NULL CHECK (id ~ '^[a-z0-9-]{1,32}$'),
    name       text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, id)
);

CREATE TABLE garden.bed (
    tenant_id  text NOT NULL,
    garden_id  text NOT NULL,
    id         text NOT NULL CHECK (id ~ '^[a-z0-9-]{1,32}$'),
    name       text NOT NULL,
    crop       text NOT NULL DEFAULT '',
    irrigation text NOT NULL CHECK (irrigation IN ('drip', 'sprinkler')),
    PRIMARY KEY (tenant_id, garden_id, id),
    FOREIGN KEY (tenant_id, garden_id) REFERENCES garden.garden (tenant_id, id)
);

CREATE TABLE garden.node (
    tenant_id    text NOT NULL,
    garden_id    text NOT NULL,
    id           text NOT NULL CHECK (id ~ '^[a-z0-9-]{1,32}$'),
    kind         text NOT NULL CHECK (kind IN ('sensor', 'actuator')),
    bed_id       text,
    fw_version   text NOT NULL DEFAULT '',
    last_seen_at timestamptz,
    PRIMARY KEY (tenant_id, garden_id, id),
    FOREIGN KEY (tenant_id, garden_id) REFERENCES garden.garden (tenant_id, id),
    FOREIGN KEY (tenant_id, garden_id, bed_id) REFERENCES garden.bed (tenant_id, garden_id, id)
);

-- Gêmeo digital: desejado (escrito pela API) e reportado (vindo do nó por MQTT).
CREATE TABLE garden.node_state (
    tenant_id text NOT NULL,
    garden_id text NOT NULL,
    node_id   text NOT NULL,
    desired   jsonb,
    reported  jsonb,
    PRIMARY KEY (tenant_id, garden_id, node_id),
    FOREIGN KEY (tenant_id, garden_id, node_id) REFERENCES garden.node (tenant_id, garden_id, id)
);

CREATE TABLE garden.alert (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       text NOT NULL,
    garden_id       text NOT NULL,
    node_id         text,
    severity        text NOT NULL CHECK (severity IN ('info', 'warning', 'critical')),
    kind            text NOT NULL,
    message         text NOT NULL,
    raised_at       timestamptz NOT NULL DEFAULT now(),
    acknowledged_at timestamptz,
    FOREIGN KEY (tenant_id, garden_id) REFERENCES garden.garden (tenant_id, id)
);
CREATE INDEX alert_garden_idx ON garden.alert (tenant_id, garden_id, raised_at DESC);

CREATE TABLE garden.command (
    id               uuid PRIMARY KEY,
    tenant_id        text NOT NULL,
    garden_id        text NOT NULL,
    node_id          text NOT NULL,
    action           text NOT NULL CHECK (action IN ('open_valve')),
    duration_seconds integer NOT NULL CHECK (duration_seconds BETWEEN 1 AND 1800),
    status           text NOT NULL CHECK (status IN ('pending', 'sent', 'confirmed', 'rejected', 'expired', 'failed')),
    requested_by     text NOT NULL,
    created_at       timestamptz NOT NULL,
    expires_at       timestamptz NOT NULL,
    FOREIGN KEY (tenant_id, garden_id, node_id) REFERENCES garden.node (tenant_id, garden_id, id)
);
CREATE INDEX command_open_idx ON garden.command (tenant_id, garden_id, node_id) WHERE status IN ('pending', 'sent');

DO $$
DECLARE t text;
BEGIN
    FOREACH t IN ARRAY ARRAY['garden', 'bed', 'node', 'node_state', 'alert', 'command'] LOOP
        EXECUTE format('ALTER TABLE garden.%I ENABLE ROW LEVEL SECURITY', t);
        EXECUTE format('ALTER TABLE garden.%I FORCE ROW LEVEL SECURITY', t);
        EXECUTE format('CREATE POLICY tenant_rw ON garden.%I TO telemetry_tenant
                        USING (tenant_id = telemetry.current_tenant())
                        WITH CHECK (tenant_id = telemetry.current_tenant())', t);
        EXECUTE format('CREATE POLICY ops_read ON garden.%I FOR SELECT TO telemetry_ops USING (true)', t);
    END LOOP;
END $$;

-- Tenant lê tudo do catálogo; escreve onde a API o faz em nome dele (o papel de aplicação é validado na API).
GRANT SELECT ON ALL TABLES IN SCHEMA garden TO telemetry_tenant, telemetry_ops;
GRANT INSERT ON garden.garden, garden.bed, garden.node, garden.node_state, garden.command TO telemetry_tenant;
GRANT UPDATE (desired) ON garden.node_state TO telemetry_tenant;
GRANT UPDATE (acknowledged_at) ON garden.alert TO telemetry_tenant;

-- Papel de serviço da API: a conexão base escreve estado reportado, alertas e expiração
-- de comandos entre tenants, e assume tenant_<id> (SET ROLE) para as requisições dos usuários.
CREATE ROLE svc_api_rw NOLOGIN;
GRANT USAGE ON SCHEMA garden, telemetry TO svc_api_rw;
GRANT SELECT ON ALL TABLES IN SCHEMA garden TO svc_api_rw;
GRANT INSERT, UPDATE ON garden.node_state, garden.alert TO svc_api_rw;
GRANT UPDATE (status) ON garden.command TO svc_api_rw;
GRANT UPDATE (last_seen_at, fw_version) ON garden.node TO svc_api_rw;
GRANT SELECT ON telemetry.reading TO svc_api_rw;
CREATE POLICY svc_api_all ON garden.node_state TO svc_api_rw USING (true) WITH CHECK (true);
CREATE POLICY svc_api_all ON garden.alert TO svc_api_rw USING (true) WITH CHECK (true);
CREATE POLICY svc_api_all ON garden.command TO svc_api_rw USING (true);
CREATE POLICY svc_api_all ON garden.node TO svc_api_rw USING (true);
CREATE POLICY svc_api_all ON garden.garden TO svc_api_rw USING (true);
CREATE POLICY svc_api_all ON garden.bed TO svc_api_rw USING (true);
CREATE POLICY svc_api_read ON telemetry.reading FOR SELECT TO svc_api_rw USING (true);

-- create_tenant_role passa a permitir que a API assuma o papel (sem herdar privilégios dele).
CREATE OR REPLACE FUNCTION telemetry.create_tenant_role(p_tenant text, p_password text) RETURNS void
    LANGUAGE plpgsql
    AS $$
BEGIN
    IF p_tenant !~ '^[a-z0-9-]{1,32}$' THEN
        RAISE EXCEPTION 'tenant inválido: %', p_tenant;
    END IF;
    EXECUTE format('CREATE ROLE %I LOGIN PASSWORD %L IN ROLE telemetry_tenant', 'tenant_' || p_tenant, p_password);
    EXECUTE format('GRANT %I TO svc_api WITH INHERIT FALSE, SET TRUE', 'tenant_' || p_tenant);
END
$$;
