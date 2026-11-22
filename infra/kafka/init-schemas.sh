#!/bin/sh
# Fixa compatibilidade BACKWARD por subject (ADR 0002) e registra o schema Protobuf do
# contrato. Subject = <tópico>-value (TopicNameStrategy). Idempotente.
set -eu

SR="${SCHEMA_REGISTRY_URL:?}"
PROTO_DIR=/contracts/proto

register() { # subject arquivo-proto
  subject=$1 file=$2
  curl -fsS -X PUT "$SR/config/$subject" \
    -H 'Content-Type: application/vnd.schemaregistry.v1+json' \
    -d '{"compatibility":"BACKWARD"}' >/dev/null
  schema=$(sed 's/\\/\\\\/g; s/"/\\"/g' "$file" | awk '{printf "%s\\n", $0}')
  curl -fsS -X POST "$SR/subjects/$subject/versions" \
    -H 'Content-Type: application/vnd.schemaregistry.v1+json' \
    -d "{\"schemaType\":\"PROTOBUF\",\"schema\":\"$schema\"}" >/dev/null
  echo "registrado: $subject"
}

register telemetria.bruta.v1-value "$PROTO_DIR/telemetria/bruta/v1/telemetria_bruta.proto"
