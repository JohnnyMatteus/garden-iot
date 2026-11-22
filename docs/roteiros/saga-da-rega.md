# Roteiro: saga da rega (núcleo `saga-da-rega`)

Código: `services/irrigation/application/saga.go`. Design doc seção 7; ADR 0003 (desejado × reportado), 0004 e 0006.

## 1. Passos

1. **Abrir.** Grava o estado desejado `open` *antes* de publicar `open_valve`; a porta `Actuator.Open` só volta após o `cmd/resposta` com `accepted: true`. Depois espera o nó reportar `open` (estado recente).
2. **Confirmar vazão.** Espera `SettleTime` e passa a conferir cada leitura: nó com estado velho, válvula que não está aberta, vazão ausente ou fora de ±30% da esperada (regra `flow_mismatch`) falham a saga.
3. **Fechar por tempo ou volume.** O volume é a integral da vazão reportada no tempo (L/min × minutos entre amostras). Fecha no que vier primeiro: `Duration` ou `TargetL`.
4. **Registrar.** Um `Record` com litros reais e duração, também quando a saga foi compensada.

## 2. Compensação

Qualquer falha nos passos 1 a 3 (ou no fechamento) executa o mesmo caminho: desejado `closed`, `close_valve` com até 3 tentativas, confirmação de `closed` reportado, alerta e registro. Pontos que merecem leitura atenta:

- A compensação roda mesmo se o `Open` falhou, porque a falha pode ser só da resposta e a válvula ter aberto (comando entregue, resposta perdida).
- Roda com `context.WithoutCancel`: desligar o serviço no meio de uma rega não pode deixar a válvula aberta.
- Se nem a compensação consegue confirmar "fechada", o alerta sobe como `critical` e `Run` devolve erro além do `Outcome`. Nesse caso o firmware (tempo máximo de válvula, ADR 0004) é a última defesa.
- Request inválido (duração ou vazão esperada ≤ 0) falha antes de abrir qualquer coisa.

## 3. Desejado × reportado

`Twin.SetDesired` é gravado antes do comando; `Twin.Reported` é o que o nó publicou em `estado`. A saga trata divergência persistente como falha: desejado `open` com reportado `closed` (o firmware fechou por intertravamento) compensa e alerta, em vez de tentar reabrir sozinha.

## 4. Casos de teste e o que provam

| Teste | Prova |
| --- | --- |
| caminho feliz | abre, fecha por tempo, 2 L/min × 55 s ≈ 1,83 L registrados, sem alerta |
| fecha por volume | com alvo de 1 L fecha antes dos 60 s |
| abertura recusada | compensa e alerta mesmo sem abertura confirmada |
| vazamento (+75%) e entupimento (−80%) | fecha, alerta e registra os litros já aplicados |
| nó mudo | alerta crítico, erro explícito |
| fechamento com 2 falhas | 3ª tentativa funciona, sem alerta |
| válvula presa aberta | alerta crítico e erro |
| contexto cancelado | ainda fecha a válvula |
| request inválido | nunca publica `open_valve` |

## 5. Perguntas para a revisão em par

1. A idempotência no nó (comando com id repetido é reconfirmado) cobre o reenvio de `Open` após timeout? Hoje `Open` não reenvia: a saga compensa.
2. `StaleAfter` de 30 s combina com o período de publicação do `estado` do firmware?
3. Integrar a vazão por amostragem subestima volume se o nó reporta devagar. Vale um medidor de pulsos acumulado no firmware?
4. Quem fecha a válvula se o próprio processo do serviço morre sem `defer`? (Tempo máximo no firmware, ADR 0004.)
