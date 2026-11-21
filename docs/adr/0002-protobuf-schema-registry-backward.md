# 0002. Protobuf + Schema Registry com compatibilidade BACKWARD (G-02)

- **Status:** Proposto
- **Data:** 2026-11-21
- **Id no design doc:** G-02 (seção 13; detalhe na seção 5.2)

## Contexto

Os tópicos Kafka têm vários produtores e consumidores que evoluem em ritmos diferentes, e o firmware dos nós em campo é atualizado aos poucos (payload antigo continua chegando, ADR 0007). Sem contrato, uma mudança de campo quebra consumidores em produção. Schemas versionados são um dos padrões que o projeto pode reaproveitar do kafka-telemetry.

## Decisão

As mensagens dos tópicos Kafka usam Protobuf registrado no Schema Registry com compatibilidade BACKWARD. Os contratos ficam versionados no repositório, os tópicos carregam a versão no nome (`*.v1`) e a compatibilidade é verificada no CI (seção 11, testes de contrato).

## Alternativas consideradas

- O design doc não registra alternativas para esta decisão. A preencher pelo autor antes de aceitar.

## Consequências

- Consumidores novos leem mensagens antigas; mudança incompatível exige tópico com nova versão.
- Mais uma peça de infraestrutura (Schema Registry) e geração de código Protobuf em Go.
- O firmware em C não precisa falar Protobuf: a conversão do payload do nó acontece na fronteira (ADR 0007). O formato do payload MQTT não está definido no design doc.
