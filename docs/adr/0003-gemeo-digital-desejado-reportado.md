# 0003. Estado desejado vs. reportado (gêmeo digital) para todo atuador (G-03)

- **Status:** Proposto
- **Data:** 2026-11-21
- **Id no design doc:** G-03 (seção 13; detalhe na seção 7)

## Contexto

Comandos para válvulas e bomba podem se perder, chegar atrasados ou duplicados, e o nó pode estar offline. Uma válvula presa aberta é um risco listado (seção 15). O sistema precisa saber o que pediu e o que de fato aconteceu.

## Decisão

Todo atuador tem um gêmeo digital no contexto Comandos com estado desejado e estado reportado. O serviço de comandos grava o desejado (via outbox), envia o comando com `id`, `acao`, `duracao` e `expira_em` (MQTT 5 com Correlation Data, Response Topic e Message Expiry Interval), e só considera o estado alterado quando o nó confirma em `cmd/resposta` e reporta em `estado`. Comando sem confirmação em T segundos gera `ComandoExpirou`, nova tentativa limitada e depois alerta. No nó, comando com id já executado é ignorado e reconfirmado. A rega é uma saga (abrir válvula → confirmar vazão → fechar por tempo ou volume → registrar), com ação compensatória de fechar tudo em qualquer falha.

## Alternativas consideradas

- O design doc não registra alternativas para esta decisão. A preencher pelo autor antes de aceitar.

## Consequências

- Divergência entre desejado e reportado é visível e alertável.
- Comando atrasado não executa (expiração) e duplicado não executa duas vezes (idempotência no nó).
- Saga e gêmeo digital são núcleo do autor (modo par, seção 14) e entram no M4.
