#!/bin/bash
# Papéis de login dos serviços, com senha vinda do ambiente (nunca em SQL versionado).
set -euo pipefail
psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" \
  -v writer_pw="$TELEMETRY_WRITER_PASSWORD" -v ops_pw="$TELEMETRY_OPS_PASSWORD" -v api_pw="$GARDEN_API_PASSWORD" <<'SQL'
CREATE ROLE svc_telemetry_writer LOGIN PASSWORD :'writer_pw' IN ROLE telemetry_writer;
CREATE ROLE svc_grafana LOGIN PASSWORD :'ops_pw' IN ROLE telemetry_ops;
CREATE ROLE svc_api LOGIN PASSWORD :'api_pw' IN ROLE svc_api_rw;
SQL
