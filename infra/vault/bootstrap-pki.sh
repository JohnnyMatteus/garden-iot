#!/bin/sh
# Configura a PKI de dispositivos no Vault (modo dev) e emite os certificados dos serviços.
# Idempotente: pode rodar de novo contra o mesmo Vault. Saída em /out (infra/pki/out no host).
#
# Identidade no certificado (ADR 0009):
#   dispositivo: CN = "<tenant>/<horta>/<no>"  (o EMQX usa o CN como username na ACL)
#   serviço:     CN = "svc:<nome>"
set -eu

OUT=/out
CRL_URL="${VAULT_CRL_URL:?}"

# Extrai um campo de string do JSON pretty-printed do Vault (sem jq na imagem).
json_field() {
  grep "\"$2\":" "$1" | head -n 1 | cut -d'"' -f4 | sed 's/\\n/\n/g'
}

issue() { # role cn arquivo-base [args extras...]
  role=$1 cn=$2 base=$3
  shift 3
  vault write -format=json "pki/issue/$role" common_name="$cn" "$@" >"/tmp/$base.json"
  json_field "/tmp/$base.json" certificate >"$OUT/$base.pem"
  json_field "/tmp/$base.json" private_key >"$OUT/$base.key"
  rm -f "/tmp/$base.json"
  chmod 644 "$OUT/$base.pem" "$OUT/$base.key"
  echo "emitido: $base ($cn)"
}

if ! vault secrets list | grep -q '^pki/'; then
  vault secrets enable -path=pki pki
  vault secrets tune -max-lease-ttl=87600h pki
  vault write -field=certificate pki/root/generate/internal \
    common_name="garden-iot dev CA" key_type=ec key_bits=256 ttl=87600h >"$OUT/ca.pem"
else
  vault read -field=certificate pki/cert/ca >"$OUT/ca.pem"
fi
chmod 644 "$OUT/ca.pem"

vault write pki/config/urls \
  issuing_certificates="${VAULT_ADDR}/v1/pki/ca" \
  crl_distribution_points="$CRL_URL"
vault write pki/config/crl expiry=24h auto_rebuild=true auto_rebuild_grace_period=1h

# Dispositivos: só autenticação de cliente. A chave privada nasce no provisionamento
# (CSR gerado fora do Vault; ver provision.sh) e nunca é gravada aqui.
vault write pki/roles/device \
  allow_any_name=true enforce_hostnames=false require_cn=true \
  server_flag=false client_flag=true key_type=ec key_bits=256 \
  ttl=720h max_ttl=720h

vault write pki/roles/service \
  allow_any_name=true enforce_hostnames=false require_cn=true \
  server_flag=false client_flag=true key_type=ec key_bits=256 \
  ttl=720h max_ttl=720h

vault write pki/roles/server \
  allow_any_name=true enforce_hostnames=false require_cn=true \
  server_flag=true client_flag=false allow_ip_sans=true key_type=ec key_bits=256 \
  ttl=720h max_ttl=720h

issue server emqx emqx alt_names=localhost,emqx ip_sans=127.0.0.1
issue service svc:bridge bridge
issue service svc:fleet-sim fleet-sim
issue service svc:api api
echo "PKI pronta em $OUT"
