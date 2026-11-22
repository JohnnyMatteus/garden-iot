#!/usr/bin/env bash
# Espera todos os containers com healthcheck do projeto ficarem saudáveis (timeout em segundos).
set -euo pipefail
timeout=${1:-240}
deadline=$((SECONDS + timeout))
while ((SECONDS < deadline)); do
  pending=$(docker ps --filter "label=com.docker.compose.project=${COMPOSE_PROJECT_NAME:-garden-iot}" \
    --format '{{.Names}} {{.Status}}' | grep -E '\((health: starting|unhealthy)\)' || true)
  [[ -z $pending ]] && { echo "todos saudáveis"; exit 0; }
  sleep 3
done
echo "timeout; pendentes:" >&2
echo "$pending" >&2
exit 1
