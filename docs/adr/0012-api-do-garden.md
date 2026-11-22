# 0012. Serviço `api` como fronteira para web e comandos

- **Status:** Proposto
- **Data:** 2026-11-21
- **Id no design doc:** não existe; adicionada a pedido do autor (o design doc não define API para o front-end nem para comandos)

## Contexto

O front-end Vue e os operadores precisam consultar hortas, canteiros, nós, histórico, estado do nó e alertas, ver leituras ao vivo e pedir rega manual. O design doc prevê "Serviço de comandos" e regras de segurança (seções 6.4 e 7), mas nenhuma API. Ninguém além dos serviços pode falar MQTT ou Kafka, e o agente de IA não aciona atuador (ADR 0006).

## Decisão

Um serviço Go `services/api`, hexagonal, expõe REST (contrato em `contracts/openapi/garden-api.yaml`) e SSE.

- Autenticação JWT; claims `tenant`, `sub` e `role` (dono, operador, leitor). O tenant do token escolhe o papel de banco (ADR 0010).
- Leituras e histórico: SELECT no TimescaleDB com RLS. Ao vivo: SSE alimentado por consulta periódica (2 s) a `ingested_at` com o papel do tenant. A API não consome Kafka no M1: o caminho Kafka alimenta o banco pelo consumidor de persistência, e ler do banco reaproveita o `DevicePayloadTranslator` e o RLS sem duplicar a tradução do payload.
- Estado do nó (gêmeo digital): desejado e reportado em tabelas próprias; reportado vem de `estado` e `cmd/resposta` via MQTT, desejado é gravado pela API.
- Rega manual: o caso de uso passa pela política de segurança do domínio (papel, reservatório, geada, vento, limite de duração) e só então publica `horta/{tenant}/{horta}/{no}/cmd` em MQTT 5 com Correlation Data (id do comando), Response Topic e Message Expiry Interval. Intertravamentos finais continuam no firmware (ADR 0004). A API nunca aciona atuador diretamente, apenas enfileira um comando que o nó pode recusar.
- A API tem credencial MQTT própria (`svc:api`), com ACL limitada a publicar em `cmd` e assinar `estado` e `cmd/resposta`.

## Como conversa com MQTT e Kafka

| Sistema | Direção | O quê |
| --- | --- | --- |
| MQTT (EMQX) | publica | `horta/{t}/{h}/{n}/cmd` QoS 1, Correlation Data = id do comando, Response Topic, Message Expiry |
| MQTT (EMQX) | assina | `.../estado` (gêmeo reportado) e `.../cmd/resposta` (confirmação) |
| Kafka | nenhuma | telemetria chega ao banco pela bridge e pela persistência; a API só lê o banco |
| TimescaleDB | lê e escreve | leituras (leitura), catálogo, estado, alertas e comandos (RLS por tenant) |

## Alternativas consideradas

- API consumindo `telemetria.bruta.v1` direto para o SSE: menor latência, mas duplica o tradutor de payload e a regra de tenant; fica como evolução se o polling pesar.

- Front-end falando direto com o banco ou com MQTT/WebSocket: expõe credenciais e pula a política de segurança.
- Colocar a API dentro do serviço de irrigação: mistura leitura de painel com decisão de rega e atrasa o M1.
- GraphQL: mais flexível, mas o contrato OpenAPI com testes de contrato é mais simples de verificar no CI.

## Consequências

- Mais um serviço e uma credencial MQTT a provisionar.
- O comando é assíncrono: a API responde 202 com o id e o cliente acompanha por SSE ou consulta.
- O contrato OpenAPI é validado no CI (lint) e o servidor é testado contra ele.
