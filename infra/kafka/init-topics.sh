#!/bin/sh
# Cria os tópicos do M1 (retenção do design doc, seção 5.2). Idempotente.
set -eu

BOOTSTRAP="${KAFKA_BOOTSTRAP:?}"
KT=/opt/kafka/bin/kafka-topics.sh

# nome:retenção em ms (um argumento por tópico)
set -- "telemetria.bruta.v1:604800000"
for entry in "$@"; do
  name="${entry%%:*}"
  retention="${entry##*:}"
  "$KT" --bootstrap-server "$BOOTSTRAP" --create --if-not-exists \
    --topic "$name" --partitions 3 --replication-factor 1 \
    --config "retention.ms=$retention"
done
