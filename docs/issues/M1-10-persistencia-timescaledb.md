# M1-10 · Persistência da telemetria no TimescaleDB

- **core:** false
- **curso:** fc4-sql-postgres
- **subagente:** implementador (Go)
- **depende de:** M1-01, M1-07, M1-08
- **estimativa:** 1 dia

## Objetivo

Consumir `telemetria.bruta.v1`, converter cada leitura com o `DevicePayloadTranslator` e gravar numa hypertable do TimescaleDB com tenant, horta, nó, sensor, instante, valor, unidade e flag de qualidade, de forma idempotente.

## Critérios de aceite

```gherkin
Funcionalidade: Persistência da telemetria

  Cenário: Leituras gravadas
    Dado uma mensagem com 5 leituras do nó N
    Quando o consumidor processa
    Então 5 linhas estão na hypertable com o instante original da leitura

  Cenário: Reprocessamento idempotente
    Dado a mesma mensagem consumida de novo após reinício
    Quando o consumidor processa
    Então nenhuma linha é duplicada

  Cenário: Offset depois do commit
    Dado uma falha no banco no meio do lote
    Então o offset não avança e o lote é reprocessado

  Cenário: Isolamento por tenant
    Dado leituras dos tenants T1 e T2
    Quando consulto pelo papel do tenant T1
    Então só vejo leituras de T1
```

## Arquivos prováveis

- `services/telemetry-store/` (consumer, translator, repository)
- `services/telemetry-store/migrations/*.sql`

## Notas

- As leituras ficam por nó e sensor; a associação a canteiro (`canteiro_id`) depende do contexto Cultivo (M3).
- Como o isolamento por tenant é verificado no banco (RLS, esquema por tenant ou filtro na aplicação) não está no design doc; a issue propõe RLS.
