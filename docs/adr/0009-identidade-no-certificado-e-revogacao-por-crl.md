# 0009. Identidade no CN do certificado e revogação por CRL

- **Status:** Proposto
- **Data:** 2026-11-21
- **Id no design doc:** decisão de implementação do M1-05/M1-06 (seções 5.1 e 9)

## Contexto

O design doc exige mTLS por dispositivo e ACL por dispositivo, mas não define onde está a identidade (CN, SAN) nem como a revogação chega ao broker. A ACL do EMQX só enxerga `${username}`, `${clientid}` e `${cn}` como variáveis, sem separar partes.

## Decisão

- CN do dispositivo = `tenant/horta/no`, cada parte `[a-z0-9-]{1,32}`. O EMQX usa o CN como username (`mqtt.peer_cert_as_username = cn`) e a ACL escreve `horta/${username}/telemetria`. O formato é imposto por regex na própria ACL, o que impede curinga e injeção de segmento. O certificado leva também um URI SAN `spiffe://garden-iot/tenant/T/horta/H/no/N` para leitura por humanos e ferramentas.
- Serviços usam CN `svc:<nome>` (`svc:bridge`, `svc:fleet-sim`), com regras próprias e escopo mínimo na ACL.
- A chave privada do dispositivo nasce no provisionamento (CSR gerado fora do Vault); o Vault só assina o CSR.
- Revogação por CRL: o Vault publica a CRL (`/v1/pki/crl/pem`, endereço embutido no certificado) e o EMQX a consulta (`enable_crl_check`), com atualização a cada minuto no ambiente local.

## Alternativas consideradas

- OCSP: resposta imediata, mas o Vault não oferece OCSP responder nesta versão de forma simples.
- Identidade só no SAN: o EMQX não a expõe à ACL sem plugin.
- Autorização por banco de dados (tabela de dispositivos): mais flexível, mas adiciona dependência no caminho de conexão.

## Consequências

- Revogar vale no próximo refresh da CRL, não instantaneamente (janela de até 1 minuto local).
- Com `auto_rebuild=true` (necessário para a CRL não expirar) o Vault não regenera a CRL ao revogar. Toda revogação precisa ser seguida de `pki/crl/rotate`, como faz `infra/vault/provision.sh --revoke`; revogar por outro caminho sem girar só vale no rebuild periódico (até cerca de 23 h).
- Trocar tenant/horta de um nó exige novo certificado.
- Se a CRL ficar indisponível o broker recusa conexões novas (falha fechada).
