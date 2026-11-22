#!/usr/bin/env bash
# Provisiona um dispositivo: gera a chave privada localmente, assina o CSR na CA do Vault
# e entrega certificado e chave. A chave privada nunca passa pelo Vault nem é logada.
#
# Uso: infra/vault/provision.sh <tenant> <horta> <no> [diretório-de-saída]
#      infra/vault/provision.sh --revoke <tenant> <horta> <no>
set -euo pipefail

ID_RE='^[a-z0-9-]{1,32}$'
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
export VAULT_ADDR=http://127.0.0.1:8200
TOKEN="${VAULT_DEV_ROOT_TOKEN_ID:-dev-only-change-me}"

vault_cli() { docker compose --project-directory "$ROOT" exec -T -e VAULT_ADDR="$VAULT_ADDR" -e VAULT_TOKEN="$TOKEN" vault vault "$@"; }

revoke=false
if [[ "${1:-}" == "--revoke" ]]; then revoke=true; shift; fi
[[ $# -ge 3 ]] || { echo "uso: $0 [--revoke] <tenant> <horta> <no> [saida]" >&2; exit 2; }
tenant=$1 horta=$2 no=$3
out=${4:-$ROOT/infra/pki/out/devices}
for part in "$tenant" "$horta" "$no"; do
  [[ $part =~ $ID_RE ]] || { echo "identificador inválido: $part (use $ID_RE)" >&2; exit 2; }
done
cn="$tenant/$horta/$no"
base="$out/$tenant-$horta-$no"

if $revoke; then
  serial=$(openssl x509 -in "$base.pem" -noout -serial | cut -d= -f2 | sed 's/../&:/g; s/:$//')
  vault_cli write pki/revoke serial_number="$serial" >/dev/null
  echo "revogado: $cn (serial $serial)"
  exit 0
fi

mkdir -p "$out"
umask 077
openssl ecparam -name prime256v1 -genkey -noout -out "$base.key"
csr=$(openssl req -new -key "$base.key" -subj "/CN=$cn")
printf '%s\n' "$csr" | vault_cli write -field=certificate pki/sign/device \
  common_name="$cn" csr=- \
  uri_sans="spiffe://garden-iot/tenant/$tenant/horta/$horta/no/$no" >"$base.pem"
echo "provisionado: $cn -> $base.pem / $base.key"
