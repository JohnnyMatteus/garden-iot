# M1-01 · Esqueleto do repositório e ambiente local

- **core:** false
- **curso:** fc4-containers-deep
- **subagente:** devops
- **depende de:** ADRs 0001, 0008 aceitos
- **estimativa:** 1 dia

## Objetivo

Criar o repositório (firmware em ESP-IDF, serviços em Go, contratos Protobuf, infraestrutura) e um `docker compose` de desenvolvimento com EMQX, Kafka, Schema Registry, TimescaleDB, Grafana e Vault em modo dev, mais CI com build do firmware, lint e testes Go.

## Critérios de aceite

```gherkin
Funcionalidade: Esqueleto e ambiente local

  Cenário: Ambiente sobe com um comando
    Dado o repositório recém-clonado
    Quando "docker compose up -d" roda
    Então EMQX, Kafka, Schema Registry, TimescaleDB, Grafana e Vault ficam saudáveis (healthcheck)

  Cenário: Firmware compila no CI
    Dado um PR
    Quando o CI roda
    Então o firmware compila com ESP-IDF numa versão fixada e os testes Go passam em jobs separados

  Cenário: Estrutura hexagonal nos serviços Go
    Dado os serviços em services/
    Então cada um tem domain/, application/, infrastructure/ e interfaces/

  Cenário: Limites de recursos
    Dado o compose de desenvolvimento
    Então cada container tem limite de memória e roda sem privilégios
```

## Arquivos prováveis

- `firmware/` (projeto ESP-IDF)
- `services/` (Go: bridge, persistência, simulador)
- `contracts/proto/`
- `deploy/compose/docker-compose.yml`
- `.github/workflows/ci.yml`

## Notas

- O layout do repositório é uma suposição desta quebra; o design doc não o define.
- O projeto é independente do kafka-telemetry: pode reaproveitar padrões dele (schemas versionados, garantias de entrega), mas não importa código nem infraestrutura dele.
