# 0005. Irrigação por FAO-56 corrigida pelo sensor de umidade (G-05)

- **Status:** Proposto
- **Data:** 2026-11-21
- **Id no design doc:** G-05 (seção 13; detalhe na seção 6)

## Contexto

Sensores baratos de umidade do solo têm leitura instável (seção 15), e só o modelo agronômico não enxerga o que acontece no canteiro. O projeto quer decisões de rega explicáveis com um modelo conhecido.

## Decisão

A necessidade hídrica vem da evapotranspiração de referência FAO-56 (Penman-Monteith, seção 6.1), com `ETc = Kc × ET₀` por cultura e estágio fenológico e balanço hídrico por canteiro (seção 6.2). O gatilho é o déficit passar do limite de água facilmente disponível ou a umidade medida cair abaixo do limite calibrado. A medida do sensor corrige o modelo: o sensor tem prioridade quando está saudável; o modelo assume quando o sensor está anômalo.

## Alternativas consideradas

- O design doc não registra alternativas para esta decisão. A preencher pelo autor antes de aceitar.

## Consequências

- Decisão auditável e testável com os valores de referência do exemplo da FAO-56 (seção 11).
- Depende de detecção de anomalia por sensor (M2) e de calibração por canteiro.
- ET₀ e balanço hídrico são núcleo do autor (modo par, seção 14) e entram no M2 e M3.
