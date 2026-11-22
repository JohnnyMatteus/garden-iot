# CLAUDE.md

Plataforma IoT de cuidado de hortas: edge (ESP32) para MQTT 5 (EMQX) para Kafka para TimescaleDB, mais API Go. Leia `docs/design-doc.md`, `docs/adr/` e `docs/issues/`.

## Regras

- Domínio fictício; sem segredos reais. Senhas de dev só como `dev-only-change-me`. `.env` fora do git.
- Docs em português do Brasil; identificadores de código em inglês.
- Contratos entre partes ficam em `contracts/` (Protobuf, contrato MQTT, OpenAPI). Mudança de contrato: `make contracts-lint contracts-breaking`; nunca renumerar nem reutilizar campo Protobuf.
- Serviços Go com arquitetura hexagonal: `domain/`, `application/`, `infrastructure/`, `interfaces/`. Domínio não importa infraestrutura.
- Bibliotecas consolidadas em vez de código à mão; comentários só com o porquê.
- O agente de IA e a API nunca acionam atuador diretamente: só publicam comando que passa pela política de segurança (ADR 0004, 0006, 0011).
- Identidade MQTT: CN do certificado (`tenant/horta/no` ou `svc:<nome>`), ver ADR 0009.
- Máquina compartilhada: localmente só lint e testes unitários; integração roda no CI.

## Comandos

```bash
make up | down | ps | logs    # ambiente local (profiles core, observability, tools)
make seed                     # provisiona nós de exemplo
make lint | test              # unitários
make test-integration         # exige make up
make contracts-gen            # regenera contracts/gen/go (buf + protoc-gen-go v1.36.5)
```

Módulos Go: `contracts`, `services/*`, `tools/fleet-sim`, `infra/tests`. Serviços usam `replace github.com/JohnnyMatteus/garden-iot/contracts => ../../contracts`.

## Fluxo de trabalho

- Branch `feat/*`, Conventional Commits em português, commit pequeno, CI verde antes de promover (workflow `Promote` leva `feat/*` verde para a `main`).
- Workflow de CI se chama exatamente `CI` (o `Promote` depende do nome).
- Toda decisão relevante vira ADR em `docs/adr/`.
