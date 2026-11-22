## O que muda e por quê

<!-- Uma ou duas frases. Link para a issue (ex.: M1-09). -->

## Área

- [ ] firmware
- [ ] services (bridge, irrigation, assistant)
- [ ] contracts (Protobuf ou MQTT)
- [ ] infra (compose, EMQX, Vault, Kafka, TimescaleDB, Grafana)
- [ ] tools (fleet-sim)
- [ ] docs

## Como testei

<!-- Comandos rodados (make lint, make test, make test-integration) e resultado. -->

## Checklist

- [ ] Contrato Protobuf: `buf lint` e `buf breaking` passam; nenhum campo renumerado ou reutilizado
- [ ] Sem segredo real (somente valores `dev-only-change-me` em exemplos)
- [ ] Mudança de comportamento tem teste
- [ ] Decisão relevante registrada em ADR (`docs/adr/`)
- [ ] README e CLAUDE.md atualizados se comandos ou layout mudaram
