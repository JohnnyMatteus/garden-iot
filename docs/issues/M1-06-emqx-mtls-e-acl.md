# M1-06 · EMQX com mTLS e ACL por dispositivo e por serviço

- **core:** false
- **curso:** fc4-sec-authz
- **subagente:** devops + testador
- **depende de:** M1-05
- **estimativa:** 1 dia

## Objetivo

Configurar o EMQX (MQTT 5) para aceitar só conexões mTLS com certificados da CA do Vault, sem acesso anônimo, e com ACL em que cada nó só publica nos seus tópicos e só assina o seu `cmd`, e cada serviço tem escopo mínimo (seções 5.1 e 9).

## Critérios de aceite

```gherkin
Funcionalidade: Segurança do broker

  Cenário: Conexão anônima recusada
    Dado um cliente sem certificado
    Quando tenta conectar
    Então o EMQX recusa

  Cenário: Nó publica só nos seus tópicos
    Dado o nó N da horta H do tenant T conectado
    Quando publica em horta/T/H/N/telemetria
    Então a mensagem é aceita
    Quando publica em horta/T/H/OUTRO/telemetria
    Então a publicação é negada

  Cenário: Isolamento entre tenants
    Dado um nó do tenant T1
    Quando tenta assinar horta/T2/#
    Então a assinatura é negada

  Cenário: Escopo da bridge
    Dado a credencial da bridge
    Então ela assina horta/+/+/+/telemetria e não publica em cmd

  Cenário: Certificado revogado
    Dado um certificado revogado no Vault
    Quando o dispositivo tenta conectar
    Então o EMQX recusa
```

## Arquivos prováveis

- `deploy/emqx/emqx.conf`, `deploy/emqx/acl.conf`
- `tests/broker/acl_test.go`

## Notas

- O mecanismo de checagem de revogação (CRL ou OCSP) não está no design doc; decisão do autor.
- Tenant em todos os tópicos desde o M1 (seção 9, multitenant).
