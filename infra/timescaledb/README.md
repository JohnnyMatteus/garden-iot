# TimescaleDB

Persistência da telemetria (M1-10). Esquema criado pelo entrypoint do Postgres a partir de `init/` na criação do volume (`make clean && make up` recria).

- `telemetry.reading`: hypertable particionada por `read_at` (1 dia). Chave natural `(tenant_id, garden_id, node_id, sensor_id, kind, read_at, sequence)`; o consumidor usa `INSERT ... ON CONFLICT DO NOTHING`, então reprocessar um lote não duplica linhas e o offset Kafka só avança depois do commit.
- `kind` normalizado (saída do `DevicePayloadTranslator` v1): `soil_moisture_vwc_pct` (%), `air_temp_c` (°C), `air_humidity_pct` (%), `illuminance_lux` (lux), `solar_radiation_w_m2`, `water_level_ok`. `quality`: `ok`, `estimated`, `suspect`.
- `ingested_at` (padrão `now()`) mede o atraso ponta a ponta no painel Grafana.
- Isolamento por tenant com RLS (ADR 0010): papéis `tenant_<id>` (membros de `telemetry_tenant`) só veem o próprio tenant; `svc_telemetry_writer` grava; `svc_grafana` lê todos os tenants.
- Criar papel de tenant: `SELECT telemetry.create_tenant_role('acme', '<senha>');`
- Teste: `infra/timescaledb/tests/rls_test.sh` (stack no ar).

Fora do M1: compressão (incompatível com RLS nesta versão), agregados contínuos e retenção (M2).
