# M1-14 · Validação do critério de pronto do M1

- **core:** false
- **curso:** —
- **subagente:** testador
- **depende de:** M1-03, M1-04, M1-12, M1-13
- **estimativa:** ½ dia

## Objetivo

Conferir, num ambiente limpo, que o M1 (Telemetria simulada) está pronto: firmware no Wokwi, EMQX com mTLS e ACL, bridge para Kafka, TimescaleDB e painel Grafana, e registrar as evidências.

## Critérios de aceite

```gherkin
Funcionalidade: Critério de pronto do M1

  Cenário: Checklist do plano de entrega
    Dado o repositório na branch principal
    Quando sigo o README do zero
    Então o firmware roda no Wokwi e o job de firmware está verde
    E o simulador publica por mTLS, a ACL nega tópicos de outro nó e de outro tenant
    E as leituras chegam ao Kafka, ao TimescaleDB e ao painel Grafana

  Cenário: SLO de leitura medido
    Dado o simulador com 100 nós por 10 minutos
    Então 99% das leituras estão disponíveis no TimescaleDB em menos de 10 segundos
```

## Arquivos prováveis

- `docs/projects/garden-iot/issues/README.md` (seção de validação)

## Notas

- O design doc não tem critério de pronto explícito por marco; o checklist deriva da linha do M1 no plano de entrega e precisa da aprovação do autor.
