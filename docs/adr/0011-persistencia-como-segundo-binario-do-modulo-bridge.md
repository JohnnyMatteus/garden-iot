# 0011. Consumidor Kafka → TimescaleDB como segundo binário do módulo da bridge (M1-10)

- **Status:** Proposto
- **Data:** 2026-11-21

## Contexto

A persistência (M1-10) lê `telemetria.bruta.v1` e grava leituras em grandezas de domínio, o que exige o `DevicePayloadTranslator` (ADR 0007). Esse pacote fica em `services/bridge/internal/acl`, e o `internal` do Go impede que outro módulo o importe.

## Decisão

A persistência é o binário `services/bridge/cmd/persist`, no mesmo módulo Go da bridge, com sua própria imagem, grupo de consumo, configuração (`PERSIST_*`) e métricas. O caso de uso (`internal/readings`) depende de portas (normalizador, armazenamento, DLQ); o adaptador de banco usa `INSERT ... ON CONFLICT DO NOTHING` em lote sobre a chave natural da tabela, o que torna a gravação idempotente por `(nó, sensor, grandeza, instante, sequência)`. O offset só é confirmado após a gravação (at-least-once). Lotes que a ACL não converte vão para `dlq.telemetria.bruta.v1`.

## Alternativas consideradas

- Módulo `services/persist` separado: exigiria mover a ACL para um módulo compartilhado, sem ganho no M1.
- Persistir dentro do processo da bridge: acopla a ingestão MQTT à escrita no banco e impede escalar os dois separadamente.

## Consequências

- Um módulo, dois binários, deploys independentes; a ACL é compartilhada sem publicar módulo.
- Se o M2 trouxer outros consumidores que precisem da ACL, ela passa a um módulo próprio.
- A calibração do sensor vem de configuração no M1 (`PERSIST_SOIL_ADC_*`); por canteiro, só com o contexto Cultivo (M3).
