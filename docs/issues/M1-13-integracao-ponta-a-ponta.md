# M1-13 · Integração ponta a ponta com Testcontainers

- **core:** false
- **curso:** fc4-tests-testcontainers
- **subagente:** testador
- **depende de:** M1-09, M1-10, M1-11
- **estimativa:** 1 dia

## Objetivo

Testar o caminho simulador → EMQX (mTLS) → bridge → Kafka → persistência → TimescaleDB com containers reais (Kafka, EMQX, TimescaleDB; seção 11), incluindo duplicadas, fora de ordem e isolamento de tenant.

## Critérios de aceite

```gherkin
Funcionalidade: Pipeline de telemetria ponta a ponta

  Cenário: Leitura chega ao banco
    Dado os containers e um nó simulado
    Quando o nó publica um lote
    Então as leituras aparecem no TimescaleDB em menos de 10 segundos

  Cenário: Duplicadas e fora de ordem
    Dado o simulador injetando 10% de duplicadas e lotes fora de ordem
    Quando 100 lotes são publicados
    Então o banco tem exatamente as leituras dos 100 lotes distintos

  Cenário: Broker reinicia
    Dado o EMQX reiniciado no meio do teste
    Quando os nós reconectam
    Então nenhum lote confirmado é perdido

  Cenário: Roda no CI
    Dado um PR
    Então a suíte de integração passa num job próprio
```

## Arquivos prováveis

- `tests/integration/pipeline_test.go`
- `.github/workflows/ci.yml`

## Notas

- O limite de 10 segundos vem do SLO de leituras da seção 10.
