# M1-05 · PKI de dispositivos no Vault e provisionamento

- **core:** false
- **curso:** fc4-sec-owasp-vault
- **subagente:** implementador (Go) + devops
- **depende de:** M1-01
- **estimativa:** 1 dia

## Objetivo

Criar a CA própria no Vault e um comando de provisionamento que emite um certificado individual por dispositivo (seção 5.1), com o identificador do tenant, da horta e do nó no certificado, e registra o `Dispositivo` no contexto Dispositivos.

## Critérios de aceite

```gherkin
Funcionalidade: Provisionamento de dispositivo

  Cenário: Certificado individual
    Dado um tenant T, uma horta H e um nó N
    Quando rodo o provisionamento
    Então recebo certificado e chave privada assinados pela CA do Vault, com T, H e N identificáveis no certificado
    E DispositivoProvisionado é registrado

  Cenário: Chave privada não fica no servidor
    Dado um provisionamento concluído
    Então a chave privada do dispositivo não fica gravada em banco nem em log

  Cenário: Revogação
    Dado um dispositivo provisionado
    Quando revogo o certificado
    Então o EMQX recusa a próxima conexão dele (verificado em M1-06)

  Cenário: Serviços com credencial própria
    Dado a bridge e o simulador
    Então cada serviço tem certificado próprio, distinto dos dispositivos
```

## Arquivos prováveis

- `deploy/vault/` (configuração do PKI em modo dev)
- `services/devices/cmd/provision/`
- `services/devices/domain/`

## Notas

- O formato da identidade no certificado (CN, SAN) não está no design doc; a escolha precisa casar com a ACL do EMQX (M1-06).
- Secure boot e criptografia de flash (seção 9) ficam para o hardware real (M6).
