# Payload de telemetria do firmware, v1

O contrato oficial é [`contracts/mqtt/README.md`](../../contracts/mqtt/README.md) (fw, seq, ts,
`r[]` com `id`, `k`, `v`). Este firmware o implementa sem desvios, com duas extensões aditivas, que
os consumidores v1 ignoram ou tratam como campo opcional:

| Extensão | Onde | Motivo |
| --- | --- | --- |
| `dropped` | topo do objeto | lotes descartados por fila offline cheia desde o último lote (M1-03) |
| `k = "water_level_ok"`, `v` 0 ou 1 | item de `r[]` | nível de água (M1-02); o contrato lista só 4 grandezas |

Sensores e ids usados: `soil-1` (`soil_adc`, mediana de 9 leituras do ADC de 12 bits), `air-1`
(`air_temp_c`, `air_rh_pct`), `light-1` (`light_lux`), `water-1` (`water_level_ok`). Sensor que
falha no ciclo não aparece em `r[]`. `ts` é 0 quando o SNTP ainda não sincronizou.

Os exemplos de referência estão em `test/golden/v1/` e são gerados pelo teste do encoder em C; o
teste de contrato do `DevicePayloadTranslator` em Go os lê, então mudar o formato sem atualizar os
dois lados quebra o CI.

Last Will: `horta/{tenant}/{horta}/{no}/lwt`, QoS 1, corpo `{"online":false}`.
