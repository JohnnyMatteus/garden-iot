# Roteiro: regras de segurança e intertravamento (núcleo `regras-de-seguranca`)

Código: `services/irrigation/domain/safety`. Design doc seção 6.4; ADR 0004 (intertravamento no firmware) e ADR 0006 (nada aciona atuador sem passar pelas regras).

## 1. Princípios

- O pacote é puro: recebe contexto e lâmina, devolve parecer. Sem E/S, sem relógio.
- Três ações: `allow`, `postpone` (tenta de novo no próximo ciclo), `block` (precisa de atenção humana, gera alerta). A mais restritiva vence, e todas as violações voltam no parecer para a decisão ser explicável.
- Dado ausente não bloqueia por si só (previsão fora do ar, nó sem anemômetro), mas **limite vencido bloqueia**. Exceção deliberada: vazão esperada inválida falha fechada.
- O firmware repete o essencial (tempo máximo de válvula, ADR 0004); estas regras são a primeira camada, não a única.

## 2. Tabela de decisão

| Regra | Condição | Ação | Alerta |
| --- | --- | --- | --- |
| `reservoir` | reservatório `low` | block | sim |
| `frost` | temperatura medida ou prevista < 2 °C | block | sim |
| `wind` | aspersão e vento > 5 m/s (medido; previsão só sem anemômetro) | postpone | não |
| `rain_forecast` | chuva prevista ≥ 5 mm na janela | postpone | não |
| `rain_forecast` (leve) | 0 < chuva prevista < 5 mm | allow, lâmina − chuva efetiva | não |
| `min_depth` | lâmina final < 2 mm | postpone | não |
| `flow_mismatch` | vazão medida fora de ±30% da esperada | block (a saga fecha a válvula) | sim |

Fronteiras: 2 °C exatos não é geada; 5 m/s exatos ainda rega; ±30% exatos é aceitável (há folga de ponto flutuante no código).

## 3. Casos conferidos à mão

- 4 mm previstos, lâmina 10 mm: chuva efetiva 4 − 2 = 2 mm, lâmina 8 mm.
- 4 mm previstos, lâmina 3,5 mm: 3,5 − 2 = 1,5 < 2 mm, adia por `min_depth`.
- Vazão esperada 2,0 L/min: aceitável de 1,4 a 2,6; 2,7 é vazamento (+35%); 1,3 é entupimento (−35%).
- Aspersão, vento 8 m/s, 0 °C, reservatório baixo: block com as três violações listadas.

## 3b. Perguntas para a revisão em par

1. Reservatório desconhecido (nó sem boia) deve liberar ou bloquear? Hoje libera; o risco é a bomba rodar a seco.
2. Geada prevista a 11 h deve bloquear uma rega agora? Hoje sim, porque a janela é de 12 h. Isso é conservador demais para cultura tolerante?
3. A regra de vento ignora previsão quando existe medição local. E se o anemômetro travar? (O detector de sensor travado do M2 deve zerar `WindMS`.)
4. Regas manuais passam por aqui também (design doc). Existe um caminho de "forçar" que deveria ser negado? A resposta esperada é não.
