# 0010. Isolamento por tenant com RLS no TimescaleDB

- **Status:** Proposto
- **Data:** 2026-11-21
- **Id no design doc:** decisão de implementação do M1-10 (seção 9, multitenant)

## Contexto

O design doc exige tenant em todas as tabelas e isolamento verificado por testes, sem dizer como o banco aplica isso.

## Decisão

Row-Level Security na hypertable `telemetry.reading`. Cada tenant tem um papel de login `tenant_<id>` (membro de `telemetry_tenant`) e a política compara `tenant_id` com o papel corrente. O consumidor usa `svc_telemetry_writer` (escreve em qualquer tenant, o tenant vem validado da mensagem) e o Grafana usa `svc_grafana` (leitura de todos os tenants). A API (ADR 0011) abre a transação com `SET LOCAL ROLE tenant_<id>` derivado do JWT, de modo que um erro de filtro na aplicação não vaza dados.

## Alternativas consideradas

- Filtro `WHERE tenant_id` só na aplicação: simples, mas um esquecimento vaza dados.
- Esquema ou banco por tenant: isolamento forte, custo operacional alto para muitos tenants pequenos.

## Consequências

- Compressão nativa do TimescaleDB não é usada no M1 (incompatível com RLS nesta versão).
- Um papel de banco por tenant; criação via `telemetry.create_tenant_role`.
- Teste automatizado em `infra/timescaledb/tests/rls_test.sh`.
