# 0001. MQTT 5 na borda e Kafka no backend, ligados por bridge própria em Go (G-01)

- **Status:** Proposto
- **Data:** 2026-11-21
- **Id no design doc:** G-01 (seção 13; detalhe nas seções 3.1 e 5)

## Contexto

Os nós ESP32 são dispositivos com pouca memória, muitos a bateria e com conexão instável; precisam de um protocolo leve, com QoS, Last Will e request/response para comandos. O backend precisa de retenção, reprocessamento, agregação em janela e vários consumidores independentes (streams, irrigação, persistência). O garden-iot é um projeto próprio, ao lado do kafka-telemetry, e pode reaproveitar padrões dele (schemas versionados, garantias de entrega) sem depender dele.

## Decisão

Os dispositivos falam MQTT 5 com o EMQX (mTLS + ACL por dispositivo; tópicos `horta/{tenant}/{horta}/{no}/...`, seção 5.1). Uma bridge própria em Go leva as mensagens do MQTT para o Kafka: produtor idempotente, deduplicação por `(no_id, sequencia)` e mensagens inválidas para DLQ com motivo. O backend processa a partir dos tópicos Kafka da seção 5.2. Um broker MQTT local no gateway de borda (opcional) aplica regras de segurança offline.

## Alternativas consideradas

- O design doc não registra alternativas para esta decisão (ex.: conector pronto em vez de bridge própria). A preencher pelo autor antes de aceitar.

## Consequências

- Cada lado usa o protocolo adequado: leve na borda, durável e reprocessável no backend.
- A bridge é código próprio a manter, testar (duplicadas, fora de ordem) e observar (lag).
- Dois brokers para operar (EMQX e Kafka) e, opcionalmente, um terceiro na borda.
