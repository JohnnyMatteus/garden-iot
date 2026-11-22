# 0013. Confirmação de entrega da telemetria e reinício do broker

- **Status:** Proposto (a parte B pede decisão do autor)
- **Data:** 2026-11-22

## Contexto

O teste `TestBrokerRestartLosesNothing` reinicia o EMQX no meio da simulação e perdeu 90 de 1800 leituras (15 lotes), com o banco em 1710. Dois mecanismos de perda, confirmados em fonte (quanto cada um pesou naquele run não foi medido):

- **A. PUBACK 0x10.** O EMQX responde ao publicador com reason code `0x10` (No matching subscribers) quando o roteamento não encontra nenhum assinante. Em `emqx_channel.erl` (v5.8.6): `puback_reason_code(PacketId, Msg, [] = PubRes)` devolve `?RC_NO_MATCHING_SUBSCRIBERS` e `?RC_NO_MATCHING_SUBSCRIBERS` é `16#10` (https://raw.githubusercontent.com/emqx/emqx/v5.8.6/apps/emqx/src/emqx_channel.erl, https://raw.githubusercontent.com/emqx/emqx/v5.8.6/apps/emqx/include/emqx_mqtt.hrl). O EMQX 5.8 guarda sessões e assinaturas na memória do nó; depois do reinício, os publicadores reconectam antes de a bridge assinar de novo (no CI, a assinatura voltou 6 s depois da queda). O paho.golang v0.23.0 só trata `ReasonCode >= 0x80` como erro (`publishQoS12`, https://raw.githubusercontent.com/eclipse-paho/paho.golang/v0.23.0/paho/client.go), e o simulador descartava a resposta: o lote era dado como entregue e perdido.
- **B. Estado em memória do broker.** O PUBACK do EMQX confirma o roteamento, não a leitura pela bridge. A mensagem aceita fica na fila ou nas inflight da sessão, em memória, e some se o nó reiniciar antes de a bridge confirmá-la. A documentação do EMQX descreve sessões regulares como "state is lost when the EMQX node restarts" (https://docs.emqx.com/en/emqx/latest/durability/durability_introduction.html). Sessões duráveis existem na 5.8.6 open source em nó único: `emqx_durable_storage` é app comum às duas edições e o backend `emqx_ds_builtin_local` entra no perfil CE (`apps/emqx_ds_backends`). A doc da 5.8 abre dizendo que o recurso é Enterprise, mas mais adiante descreve que "in the open-source edition, session data is only stored on the current node" e é restaurado quando o nó volta (https://github.com/emqx/emqx-docs/blob/release-5.8/en_US/guides/durability/durability_introduction.md). O que é só Enterprise é a réplica (`emqx_ds_builtin_raft`) e a assinatura compartilhada durável (`emqx_ds_shared_sub`, BSL), que a doc da 5.8 ainda dá como não suportada. A bridge usa `$share/bridge/...`, então ligar sessões duráveis sem mudar a assinatura não a protege.

No firmware, o `esp-mqtt` do ESP-IDF v5.4.1 (submódulo no commit `cac1552e62b0474c162547b7cce345d7cd1aecfe`) só escreve o reason code do PUBACK no log (`esp_mqtt5_parse_puback`, `ESP_LOGI`) e não o expõe em `MQTT_EVENT_PUBLISHED` (https://raw.githubusercontent.com/espressif/esp-mqtt/cac1552e62b0474c162547b7cce345d7cd1aecfe/mqtt5_client.c). A versão atual do esp-mqtt expõe `event->reason_code` (https://raw.githubusercontent.com/espressif/esp-mqtt/master/mqtt5_client.c); não verifiquei em qual versão do ESP-IDF isso chega.

## Decisão

**Parte A (implementada).** O publicador só considera entregue uma publicação de telemetria quando o PUBACK confirma roteamento: `tools/fleet-sim/internal/publisher` passa a tratar `0x10` em `/telemetria` como falha (`ErrNoMatchingSubscribers`), e a repetição existente (`fleet.publish`, janela `-retry`) reenvia até a bridge reassinar. Só a telemetria exige assinante: estado e LWT podem não ter nenhum.

A premissa é a ACL: só `svc:bridge`, pelo grupo `$share/bridge`, pode assinar `horta/+/+/+/telemetria` (`infra/emqx/acl.conf`; `svc:api` e `svc:irrigation` assinam apenas `estado` e `cmd/resposta`; dispositivos assinam só o próprio `cmd`). Por isso `0x10` equivale a "nenhuma réplica da bridge assinada". Se outro assinante passar a casar esse tópico (um painel, por exemplo), `0x10` deixa de proteger e esta decisão precisa ser revista. Os publicadores `api` e `irrigation` (comandos) têm outra semântica: `0x10` ali significa nó sem assinatura de `cmd`, tratada pelo gêmeo digital (ADR 0003), e não foi alterada.

**Parte B (pendente de decisão do autor).** Com a assinatura compartilhada atual, uma mensagem aceita e ainda não confirmada pela bridge não sobrevive ao reinício do nó. Opções:

1. **Sessão durável no open source, com assinatura não compartilhada.** `durable_sessions.enable`, volume de dados e nome de nó estáveis, bridge com client id fixo e filtro sem `$share`, ACL ajustada. Mantém licença e firmware; como a assinatura sobrevive ao restart, cobre também o 0x10 no aparelho. Custos: uma única bridge ativa (escalar exige particionar os filtros) e durabilidade limitada ao disco do nó. Não foi exercitada.
2. **Confirmação fim a fim pela aplicação.** A bridge confirma ao nó depois de gravar no Kafka (por exemplo, `.../telemetria/ack` com o `seq`) e o nó só libera o lote do buffer com esse ack. É a garantia mais forte e mantém as réplicas. Custos: tópico e ACL novos, mudança no firmware e no simulador.
3. **EMQX Enterprise.** Traz réplica e o app de assinatura compartilhada durável, com troca de licença e de imagem. Suporte e maturidade na 5.8 não verificados.
4. **Aceitar a janela e documentar.** Não atende "confirmado nunca se perde".

Recomendação: 1 agora, por resolver o restart sem mudar firmware nem licença, e 2 quando a bridge precisar de réplicas ou o broker precisar sobreviver à perda do disco.

## Garantia e limite reais

- **Garantido agora:** o simulador não dá por entregue telemetria que o broker descartou por falta de assinante; a bridge e o persist são at-least-once com idempotência no banco (ADR 0011). No simulador, isso fecha a janela entre o reinício do broker e a nova assinatura da bridge, enquanto durar o `-retry`.
- **Não garantido:** mensagens que o EMQX já roteou para a sessão da bridge e ainda não foram confirmadas por ela, se o nó reiniciar nesse intervalo. O teste de reinício pode continuar apontando essa perda, e ela não deve ser "resolvida" com timeout maior nem esperando a bridge no teste.
- **Firmware:** como o esp-mqtt fixado não expõe o reason code, o nó não detecta `0x10` e apagaria o lote do buffer offline mesmo quando a bridge estava fora. Sem trocar a versão do ESP-IDF ou adotar a opção 1 ou a 2, o firmware tem a mesma perda que o simulador tinha. Não alterei o firmware, que não pude compilar neste ambiente.

## Consequências

- O comportamento do simulador agora é testado por `routing_test.go` apenas na lógica de decisão; o caso completo (restart do EMQX) só o CI exercita.
- O que sobrar da perda no teste de reinício aponta para a parte B e deve virar decisão do autor, não ajuste de teste.
