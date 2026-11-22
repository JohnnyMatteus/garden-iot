# ADRs: garden-iot

Decisões iniciais do [design doc](../design-doc.md) (seção 13), no formato de `docs/adr/0000-template.md`. Todas com status **Proposto** até o autor aceitar. A numeração local (`NNNN`) segue a ordem dos ids do design doc (`G-NN`).

O garden-iot é um projeto próprio, ao lado do kafka-telemetry. Pode reaproveitar padrões dele (schemas versionados, garantias de entrega), mas não depende dele nem o migra.

| ADR | Id | Decisão | Afeta o M1 |
| --- | --- | --- | --- |
| [0001](0001-mqtt5-na-borda-kafka-no-backend.md) | G-01 | MQTT 5 na borda e Kafka no backend, ligados por bridge própria em Go | sim |
| [0002](0002-protobuf-schema-registry-backward.md) | G-02 | Protobuf + Schema Registry com compatibilidade BACKWARD | sim (só `telemetria.bruta.v1`) |
| [0003](0003-gemeo-digital-desejado-reportado.md) | G-03 | Estado desejado vs. reportado (gêmeo digital) para todo atuador | não (M4) |
| [0004](0004-intertravamentos-no-firmware.md) | G-04 | Intertravamentos de segurança no firmware, independentes da nuvem | não (M4) |
| [0005](0005-irrigacao-fao56-corrigida-pelo-sensor.md) | G-05 | Irrigação por FAO-56 corrigida pelo sensor de umidade | não (M2, M3) |
| [0006](0006-agente-sem-acesso-a-atuadores.md) | G-06 | Agente de IA sem acesso direto a atuadores; propostas com aprovação | não (M5) |
| [0007](0007-acl-por-versao-de-firmware.md) | G-07 | ACL por versão de firmware para payloads de dispositivo | sim |
| [0008](0008-wokwi-e-simulador-de-frota.md) | G-08 | Wokwi + simulador de frota como ambiente padrão de desenvolvimento | sim |
| [0009](0009-identidade-no-certificado-e-revogacao-por-crl.md) | impl. | Identidade no CN do certificado e revogação por CRL | sim |
| [0010](0010-isolamento-por-tenant-com-rls.md) | impl. | Isolamento por tenant com RLS no TimescaleDB | sim |
| [0011](0011-api-do-garden.md) | pedido do autor | Serviço `api` (REST, SSE, JWT) como fronteira para web e comandos | não |

Onde o design doc não registra alternativas, o ADR diz isso explicitamente em vez de inventar; o autor completa antes de aceitar.
