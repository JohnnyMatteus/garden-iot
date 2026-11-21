# 0007. ACL por versão de firmware para payloads de dispositivo (G-07)

- **Status:** Proposto
- **Data:** 2026-11-21
- **Id no design doc:** G-07 (seção 13; detalhe na seção 3.3)

## Contexto

Os nós mandam leituras brutas (ADC, pulsos, lux) num formato que muda com o firmware, e a frota em campo nunca está toda na mesma versão (OTA gradual, nós offline). O domínio precisa de grandezas físicas com calibração por canteiro.

## Decisão

O `DevicePayloadTranslator` converte o payload de cada versão de firmware em grandezas de domínio (umidade volumétrica %, L/min, m/s, W/m²) usando a calibração do sensor. Payload de firmware antigo continua aceito. Os tradutores têm testes de contrato por versão de firmware (seção 11). A mesma ideia de ACL vale para clima (`WeatherTranslator`, Open-Meteo) e visão (`VisionTranslator`).

## Alternativas consideradas

- O design doc não registra alternativas para esta decisão. A preencher pelo autor antes de aceitar.

## Consequências

- Atualizar firmware não quebra o backend; versões antigas convivem.
- Um tradutor e um conjunto de fixtures por versão de firmware para manter.
- O payload precisa trazer a versão do firmware.
