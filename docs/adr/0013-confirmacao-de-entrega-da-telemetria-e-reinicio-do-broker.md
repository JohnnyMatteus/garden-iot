# 0013. Confirmação de entrega da telemetria e reinício do broker

- **Status:** Proposto (a parte B pede decisão do autor)
- **Data:** 2026-11-22

## Contexto

O teste `TestBrokerRestartLosesNothing` reinicia o EMQX no meio da simulação e perdeu 90 de 1800 leituras (15 lotes), com o banco em 1710. Duas causas independentes, ambas confirmadas em fonte:

- **A. PUBACK 0x10.** O EMQX responde ao publicador com reason code `0x10` (No matching subscribers) quando o roteamento não encontra nenhum assinante. Em `emqx_channel.erl` (v5.8.6): `puback_reason_code(PacketId, Msg, [] = PubRes)` devolve `?RC_NO_MATCHING_SUBSCRIBERS` e `?RC_NO_MATCHING_SUBSCRIBERS` é `16#10` (https://raw.githubusercontent.com/emqx/emqx/v5.8.6/apps/emqx/src/emqx_channel.erl, https://raw.githubusercontent.com/emqx/emqx/v5.8.6/apps/emqx/include/emqx_mqtt.hrl). O EMQX 5.8 guarda sessões e assinaturas na memória do nó; depois do reinício, os publicadores reconectam antes de a bridge assinar de novo (no CI, a assinatura voltou 6 s depois da queda). O paho.golang v0.23.0 só trata `ReasonCode >= 0x80` como erro (`publishQoS12`, https://raw.githubusercontent.com/eclipse-paho/paho.golang/v0.23.0/paho/client.go), e o simulador descartava a resposta: o lote era dado como entregue e perdido.
- **B. Estado em memória do broker.** O PUBACK do EMQX confirma o roteamento, não a leitura pela bridge. A mensagem aceita fica na fila ou nas inflight da sessão, em memória, e some se o nó reiniciar antes de a bridge confirmá-la. A documentação do EMQX descreve sessões regulares como "state is lost when the EMQX node restarts" (https://docs.emqx.com/en/emqx/latest/durability/durability_introduction.html). Sessões duráveis, que resolveriam isso, são "an EMQX Enterprise feature" e, no 5.8, "does not yet support the persistence of shared subscription sessions" (https://docs.emqx.com/en/emqx/v5.8/develop/durability_introduction.html). A bridge usa `$share/bridge/...` e a imagem do projeto é a open source `emqx/emqx:5.8.6`.

No firmware, o `esp-mqtt` do ESP-IDF v5.4.1 (submódulo no commit `cac1552e62b0474c162547b7cce345d7cd1aecfe`) só escreve o reason code do PUBACK no log (`esp_mqtt5_parse_puback`, `ESP_LOGI`) e não o expõe em `MQTT_EVENT_PUBLISHED` (https://raw.githubusercontent.com/espressif/esp-mqtt/cac1552e62b0474c162547b7cce345d7cd1aecfe/mqtt5_client.c). A versão atual do esp-mqtt expõe `event->reason_code` (https://raw.githubusercontent.com/espressif/esp-mqtt/master/mqtt5_client.c); não verifiquei em qual versão do ESP-IDF isso chega.

## Decisão

**Parte A (implementada).** O publicador só considera entregue uma publicação de telemetria quando o PUBACK confirma roteamento: `tools/fleet-sim/internal/publisher` passa a tratar `0x10` em `/telemetria` como falha (`ErrNoMatchingSubscribers`), e a repetição existente (`fleet.publish`, janela `-retry`) reenvia até a bridge reassinar. Só a telemetria exige assinante: estado e LWT podem não ter nenhum.

A premissa é a ACL: só `svc:bridge`, pelo grupo `$share/bridge`, pode assinar `horta/+/+/+/telemetria` (`infra/emqx/acl.conf`; `svc:api` e `svc:irrigation` assinam apenas `estado` e `cmd/resposta`; dispositivos assinam só o próprio `cmd`). Por isso `0x10` equivale a "nenhuma réplica da bridge assinada". Se outro assinante passar a casar esse tópico (um painel, por exemplo), `0x10` deixa de proteger e esta decisão precisa ser revista. Os publicadores `api` e `irrigation` (comandos) têm outra semântica: `0x10` ali significa nó sem assinatura de `cmd`, tratada pelo gêmeo digital (ADR 0003), e não foi alterada.

**Parte B (pendente de decisão do autor).** Com o EMQX open source 5.8 não há como garantir que uma mensagem já aceita e ainda não confirmada pela bridge sobreviva ao reinício do nó. Opções:

1. **Confirmação fim a fim pela aplicação.** A bridge só confirma ao nó depois de gravar no Kafka (por exemplo, publicando `.../telemetria/ack` com o `seq`); o nó só libera o lote do buffer offline com esse ack. Mantém a licença e dá a garantia completa (at-least-once; a deduplicação já existe). Custos: tópico e ACL novos, mudança no firmware e no simulador, e dois caminhos de ack para manter.
2. **EMQX Enterprise com sessões duráveis.** Resolve a perda em memória, mas no 5.8 não cobre assinatura compartilhada; teria que ser uma única bridge com sessão durável (sem escalar réplicas) ou uma versão mais nova que suporte o caso (a documentação "latest" descreve assinaturas compartilhadas sobre o armazenamento durável; não verifiquei versão, requisitos nem licença). Implica troca de licença e de imagem.
3. **Aceitar a janela e documentar.** A perda fica restrita às mensagens em memória no instante do reinício. Não atende "confirmado nunca se perde".

A recomendação é a opção 1, por manter a licença e a arquitetura do ADR 0001.

## Garantia e limite reais

- **Garantido agora:** o simulador não dá por entregue telemetria que o broker descartou por falta de assinante; a bridge e o persist são at-least-once com idempotência no banco (ADR 0011). Isso elimina a janela entre o reinício do broker e a nova assinatura da bridge.
- **Não garantido:** mensagens que o EMQX já roteou para a sessão da bridge e ainda não foram confirmadas por ela, se o nó reiniciar nesse intervalo. O teste de reinício pode continuar apontando essa perda, e ela não deve ser "resolvida" com timeout maior nem esperando a bridge no teste.
- **Firmware:** como o esp-mqtt fixado não expõe o reason code, o nó não detecta `0x10` e apagaria o lote do buffer offline mesmo quando a bridge estava fora. Sem trocar a versão do ESP-IDF ou adotar a opção 1, o firmware tem a mesma perda que o simulador tinha. Não alterei o firmware, que não pude compilar neste ambiente.

## Consequências

- O comportamento do simulador agora é testado por `routing_test.go` apenas na lógica de decisão; o caso completo (restart do EMQX) só o CI exercita.
- O que sobrar da perda no teste de reinício aponta para a parte B e deve virar decisão do autor, não ajuste de teste.
