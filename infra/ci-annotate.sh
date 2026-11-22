#!/usr/bin/env bash
# Diagnóstico de CI: transforma a falha da stack compose em até 5 anotações ::error, legíveis sem
# login no Actions. Nunca imprime ambiente nem configuração resolvida do compose (repositório público).
# Uso: infra/ci-annotate.sh <título> <arquivo-com-a-saída-do-passo> [argumentos do docker compose...]
set -uo pipefail

title=${1:?título}
step_log=${2:?arquivo de log}
shift 2

emitted=0
max=5

emit() {
  local text
  text=$(printf '%s' "$1" | sed 's/\x1b\[[0-9;]*[A-Za-z]//g')
  text=${text//'%'/'%25'}
  text=${text//$'\r'/'%0D'}
  text=${text//$'\n'/'%0A'}
  echo "::error title=${title}::${text:0:3500}"
  emitted=$((emitted + 1))
}

if [[ -s $step_log ]]; then
  emit "fim da saída do passo:"$'\n'"$(tail -n 25 "$step_log")"
fi

states=$(docker compose "$@" ps -a --format '{{.Service}} state={{.State}} health={{.Health}} exit={{.ExitCode}}' 2>&1 || true)
if [[ -n $states ]] && ((emitted < max)); then
  emit "estado dos serviços:"$'\n'"$states"
fi

# Serviços que não ficaram saudáveis ou que terminaram com erro: fim do log de cada um.
while read -r svc rest; do
  ((emitted < max)) || break
  [[ -n $svc ]] || continue
  case $rest in
    *health=unhealthy* | *health=starting* | *state=restarting* | *state=exited\ health=\ exit=[1-9]* | *state=dead*) ;;
    *) continue ;;
  esac
  emit "log de ${svc}:"$'\n'"$(docker compose "$@" logs --no-color --tail 15 "$svc" 2>&1)"
done <<<"$states"

exit 0
