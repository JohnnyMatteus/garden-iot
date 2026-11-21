# M1-12 · Painel Grafana provisionado

- **core:** false
- **curso:** lt-tech-metrics
- **subagente:** devops
- **depende de:** M1-10
- **estimativa:** ½ dia

## Objetivo

Provisionar por arquivo um painel Grafana com fonte TimescaleDB mostrando nós online, leituras por nó e sensor e atraso ponta a ponta (sensor → TimescaleDB), métricas da seção 10.

## Critérios de aceite

```gherkin
Funcionalidade: Painel de telemetria

  Cenário: Painel provisionado
    Dado o ambiente local recém-criado
    Quando abro o Grafana
    Então o painel "Telemetria" existe sem configuração manual

  Cenário: Leituras visíveis
    Dado o simulador rodando com 10 nós
    Então o painel mostra as séries de umidade do solo, temperatura, umidade do ar e luz por nó

  Cenário: Atraso ponta a ponta
    Dado leituras gravadas
    Então o painel mostra o p99 de (instante de gravação − instante da leitura)

  Cenário: Nó offline
    Dado um nó que parou de publicar
    Então ele sai da contagem de nós online após o intervalo configurado
```

## Arquivos prováveis

- `deploy/grafana/provisioning/datasources/*.yaml`
- `deploy/grafana/dashboards/telemetria.json`

## Notas

- Lag dos consumidores Kafka e comandos confirmados vs. expirados (seção 10) entram quando houver exporter e comandos (M2, M4).
