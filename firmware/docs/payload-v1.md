# Payload de telemetria do firmware, v1

Contrato entre o firmware (M1-02) e o `DevicePayloadTranslator` (M1-08). Vale para firmware 1.x.
O contrato Protobuf `telemetria.bruta.v1` (M1-07) embrulha este payload como "leitura bruta
mais metadados de firmware"; o JSON abaixo é o que o nó publica no MQTT.

Tópico: `horta/{tenant}/{horta}/{no}/telemetria`, QoS 1. O tenant, a horta e o nó vêm do tópico
(a ACL do broker impede um nó de publicar fora do seu), por isso não repetem no corpo.

Formato escolhido: JSON compacto (resposta ao ponto em aberto 1 do README das issues). Motivo: o
Wokwi, o EMQX e o simulador de frota o leem sem ferramenta extra, e um lote tem cerca de 170
bytes. CBOR ou Protobuf em C ficam como evolução se o consumo de bateria pedir.

| Campo | Tipo | Obrigatório | Descrição |
| --- | --- | --- | --- |
| `fw_version` | string | sim | semver do firmware; o tradutor é escolhido pela versão maior |
| `seq` | uint32 | sim | sequência monotônica por nó, sobrevive a reinício; chave de deduplicação com o nó |
| `ts` | int | sim | epoch UTC em segundos (SNTP); `0` quando o nó ainda não sincronizou o relógio |
| `dropped` | uint32 | sim | lotes descartados por buffer offline cheio desde o último lote enviado |
| `soil_adc` | int | não | mediana de 9 leituras do ADC de 12 bits do sensor capacitivo (0 a 4095) |
| `air_temp_c` | número | não | temperatura do ar em °C |
| `air_rh_pct` | número | não | umidade relativa do ar em % |
| `lux` | número | não | iluminância em lux |
| `water_level_ok` | bool | não | `true` quando há água no reservatório |

Sensor que falha no ciclo tem o campo omitido; o lote sai mesmo assim. Campos desconhecidos devem
ser ignorados por quem lê (compatível para frente).

Exemplo:

```json
{"fw_version":"1.0.0","seq":42,"ts":1732190000,"dropped":0,"soil_adc":2100,"air_temp_c":23.50,"air_rh_pct":61.25,"lux":12000.0,"water_level_ok":true}
```

Os exemplos de referência estão em `test/golden/v1/` e são gerados pelo teste do encoder em C; o
teste de contrato do tradutor em Go os lê, então mudar o formato sem atualizar os dois lados
quebra o CI.

## Last Will

Tópico `horta/{tenant}/{horta}/{no}/lwt`, QoS 1, corpo `{"online":false}`.

## Mudanças de versão

Mudança incompatível exige nova versão maior do firmware e novo tradutor registrado
(ADR 0007). Firmware 1.x antigo continua aceito.
