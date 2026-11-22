# Roteiro: ET₀ e balanço hídrico (núcleo `et0-e-balanco-hidrico`)

Roteiro de estudo e de revisão em par do código em `services/irrigation/domain/agronomy`. Referências: design doc seção 6 e ADR 0005.

## 1. O que o código promete

1. Dado o clima do dia (Tmin, Tmax, UR mín/máx, vento, radiação solar, latitude, altitude, dia do ano), `ET0` devolve a evapotranspiração de referência em mm/dia (FAO-56, eq. 6, G = 0).
2. `Crop.Kc` e `Balance.Apply` transformam ET₀ em necessidade da cultura (ETc = Kc × ET₀) e acumulam o déficit do canteiro.
3. `Balance.NeedsWater` dispara quando o déficit alcança a água facilmente disponível (RAW).

## 2. Ordem de cálculo (e o porquê de cada passo)

| Passo | Fórmula | Por quê |
| --- | --- | --- |
| Vento a 2 m | u₂ = u_z × 4,87 / ln(67,8 z − 5,42) | a fórmula foi calibrada para 2 m; anemômetro de horta costuma estar mais alto |
| eₛ(T) | 0,6108 exp(17,27 T / (T + 237,3)) | pressão de saturação; a média das extremas (Tmax, Tmin) é mais fiel que eₛ da média |
| eₐ | [eₛ(Tmin) UR_max + eₛ(Tmax) UR_min] / 200 | pressão real de vapor, a partir das duas umidades extremas |
| Δ | 4098 eₛ(Tmed) / (Tmed + 237,3)² | inclinação da curva de saturação, na temperatura média |
| γ | 0,000665 P, com P = 101,3 ((293 − 0,0065 z)/293)^5,26 | constante psicrométrica depende da altitude |
| Ra, Rso | eq. 21 e 37 | radiação de céu limpo; só serve para limitar a razão Rs/Rso a 1 e estimar a perda de onda longa |
| Rn | (1 − 0,23) Rs − Rnl | saldo de radiação; albedo 0,23 da grama de referência |
| ET₀ | numerador radiativo + aerodinâmico, dividido por Δ + γ(1 + 0,34 u₂) | Penman-Monteith |

A radiação vem do sensor como `solar_radiation_w_m2` (estimada a partir de lux, qualidade `estimado`). A conversão da média diária é MJ/m²/dia = W/m² × 0,0864 (`WattsToMJPerDay`).

## 3. Conferência à mão: exemplo 18 da FAO-56 (Uccle, 6 de julho)

Entradas: Tmin 12,3 °C, Tmax 21,5 °C, UR 63–84%, vento 10 km/h a 10 m, Rs 22,07 MJ/m²/dia, latitude 50,8° N, altitude 100 m, dia 187.

| Grandeza | Valor |
| --- | --- |
| Tmed | 16,9 °C |
| eₛ(Tmax) / eₛ(Tmin) / média | 2,564 / 1,431 / 1,997 kPa |
| eₐ | (1,431 × 0,84 + 2,564 × 0,63) / 2 = 1,409 kPa |
| eₛ − eₐ | 0,589 kPa |
| Δ | 0,122 kPa/°C |
| P / γ | 100,1 kPa / 0,0666 kPa/°C |
| u₂ | 2,778 × 4,87 / ln(678 − 5,42) = 2,078 m/s |
| Ra / Rso | 41,09 / 30,90 MJ/m²/dia |
| Rns | 0,77 × 22,07 = 16,99 |
| Rnl | 4,903e-9 × 7,09e9 (média de Tmax⁴ e Tmin⁴ em K) × (0,34 − 0,14 √1,409 = 0,174) × (1,35 × 0,714 − 0,35 = 0,614) = 3,71 |
| Rn | 13,28 MJ/m²/dia |

Numerador: 0,408 × 0,122 × 13,28 = 0,662; γ × 900 / (16,9 + 273) × u₂ × (eₛ − eₐ) = 0,2067 × 2,078 × 0,589 = 0,253; soma 0,915.
Denominador: 0,122 + 0,0666 × (1 + 0,34 × 2,078) = 0,2357.
**ET₀ = 0,915 / 0,2357 = 3,88 mm/dia** (a FAO publica 3,9). Os testes aceitam 3,9 ± 0,05 e 3,88 ± 0,01.

Outras âncoras nos testes: eₛ(20 °C) = 2,338 kPa e Δ(20 °C) = 0,145 (Anexo 2 da FAO-56).

## 4. Balanço hídrico: conferência do exemplo semanal

Tomate, solo franco (θcc 0,27; θpm 0,13), Zr 0,70 m:

- TAW = 1000 × (0,27 − 0,13) × 0,70 = **98 mm**
- RAW = p × TAW = 0,40 × 98 = **39,2 mm** (limite de gatilho)
- Kc intermediário 1,15, então ETc = 1,15 × ET₀.

Chuva efetiva: descontam-se 2 mm de interceptação (8 mm de chuva → 6 mm efetivos). O déficit é limitado a [0, TAW]: abaixo de 0 é percolação e não vira crédito.

Sequência no teste `TestWaterBalanceWeek`: 4,60 → 10,35 → 8,95 → 10,85 → 0. Oito dias de ET₀ 4,5 sem chuva dão 8 × 5,175 = 41,4 mm ≥ 39,2, o que dispara a rega com lâmina de 41,4 mm; em 2 m² são 82,8 L.

## 5. Perguntas para a revisão em par

1. Por que limitar Rs/Rso a 1? O que acontece com a perda de onda longa sem esse limite num sensor de luz que satura?
2. UR média em vez de UR mín/máx mudaria ET₀ em quanto, no exemplo 18?
3. A interceptação fixa de 2 mm é boa para chuva fina e rega por aspersão? Onde trocar por um modelo diário (USDA-SCS)?
4. O sensor de solo tem prioridade quando saudável (ADR 0005). O que impede o modelo de acumular erro durante dias em que o sensor está anômalo? (Resposta esperada: a calibração do déficit pela medida quando o sensor volta, tratada no planejador.)
5. Qual é o efeito de Kc de desenvolvimento interpolado linearmente num estágio de duração real desconhecida (`progress`)?
