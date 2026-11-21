# M1-09 · Bridge MQTT → Kafka em Go

- **core:** false
- **curso:** fc4-msg-kafka
- **subagente:** implementador (Go)
- **depende de:** M1-06, M1-07; ADR 0001
- **estimativa:** 1 dia

## Objetivo

Assinar `horta/+/+/+/telemetria` no EMQX com a credencial da bridge e publicar cada lote em `telemetria.bruta.v1` com produtor idempotente, chave `no_id` e deduplicação por `(no_id, sequencia)` (seção 5.2).

## Critérios de aceite

```gherkin
Funcionalidade: Bridge MQTT → Kafka

  Cenário: Lote encaminhado
    Dado um lote publicado pelo nó N com sequência 10
    Quando a bridge recebe
    Então uma mensagem telemetria.bruta.v1 com chave N e sequência 10 é produzida, validada no Schema Registry

  Cenário: Duplicada descartada
    Dado o mesmo lote (N, 10) entregue duas vezes pelo MQTT (QoS 1)
    Quando a bridge recebe as duas
    Então só uma mensagem chega ao Kafka

  Cenário: Fora de ordem aceito
    Dado os lotes (N, 12) e depois (N, 11)
    Quando a bridge recebe
    Então as duas mensagens chegam ao Kafka

  Cenário: Ack só depois do Kafka
    Dado o Kafka indisponível
    Quando chega um lote
    Então a bridge não confirma a mensagem MQTT até o Kafka aceitar

  Cenário: Mensagem inválida
    Dado um payload que não decodifica
    Quando a bridge recebe
    Então a mensagem é descartada com log estruturado e métrica de inválidas por motivo
```

## Arquivos prováveis

- `services/bridge/` (domain, application, infrastructure/mqtt, infrastructure/kafka)
- `services/bridge/*_test.go`

## Notas

- O design doc manda inválidas para a DLQ (seção 5.2), mas o plano de entrega põe a DLQ no M2. No M1 a bridge só descarta com log e métrica; ver pontos em aberto.
- A janela da deduplicação (memória, Redis ou outro armazenamento) não está no design doc.
