# M1-07 · Contrato Protobuf `telemetria.bruta.v1` no Schema Registry

- **core:** false
- **curso:** fc4-msg-kafka
- **subagente:** implementador (Go)
- **depende de:** M1-01; ADR 0002
- **estimativa:** ½ dia

## Objetivo

Definir em Protobuf a mensagem do tópico `telemetria.bruta.v1` (leitura bruta + metadados de firmware, chave `no_id`, retenção de 7 dias; seção 5.2), registrá-la no Schema Registry com compatibilidade BACKWARD e verificar a compatibilidade no CI.

## Critérios de aceite

```gherkin
Funcionalidade: Contrato da telemetria bruta

  Cenário: Mensagem definida
    Dado contracts/proto/telemetria/bruta/v1/
    Então a mensagem tem tenant, horta, no_id, sequencia, fw_version, instante da leitura, instante de recebimento e o payload bruto

  Cenário: Tópico criado
    Dado o ambiente local
    Então telemetria.bruta.v1 existe com retenção de 7 dias e o subject está registrado com compatibilidade BACKWARD

  Cenário: Mudança incompatível barrada
    Dado um PR que remove um campo obrigatório da mensagem
    Quando o CI roda a checagem de compatibilidade
    Então o job falha
```

## Arquivos prováveis

- `contracts/proto/telemetria/bruta/v1/telemetria_bruta.proto`
- `deploy/kafka/topics.yaml`
- `.github/workflows/ci.yml` (checagem de compatibilidade)

## Notas

- O plano de entrega põe "schemas" no M2. Como a bridge do M1 precisa produzir alguma mensagem, esta issue antecipa só o contrato de `telemetria.bruta.v1`; os demais tópicos ficam no M2. Ver pontos em aberto no README.
