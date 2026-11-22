# Contrato MQTT dos nós

Tópicos (design doc, seção 5.1). `{tenant}`, `{horta}` e `{no}` são `[a-z0-9-]{1,32}`; a identidade do certificado do nó (CN `tenant/horta/no`) tem de casar com o tópico, e o broker aplica isso na ACL.

| Tópico | QoS | Quem publica | Observação |
| --- | --- | --- | --- |
| `horta/{tenant}/{horta}/{no}/telemetria` | 1 | nó | lote de leituras (payload v1 abaixo) |
| `horta/{tenant}/{horta}/{no}/estado` | 1, retained | nó | estado reportado |
| `horta/{tenant}/{horta}/{no}/cmd` | 1 | serviço de comandos | assinado só pelo nó (M4) |
| `horta/{tenant}/{horta}/{no}/cmd/resposta` | 1 | nó | confirmação (M4) |
| `horta/{tenant}/{horta}/{no}/lwt` | 1 | nó (Last Will) | nó offline |

## Payload de telemetria, firmware v1

JSON compacto em UTF-8, um objeto por lote. Este é o contrato de entrada do `DevicePayloadTranslator` v1.

```json
{
  "fw": "1.0.0",
  "seq": 10,
  "ts": 1763726400,
  "r": [
    {"id": "soil-1", "k": "soil_adc", "v": 2100},
    {"id": "air-1", "k": "air_temp_c", "v": 23.5},
    {"id": "air-1", "k": "air_rh_pct", "v": 61.2},
    {"id": "light-1", "k": "light_lux", "v": 42000}
  ]
}
```

| Campo | Tipo | Regra |
| --- | --- | --- |
| `fw` | string | versão semântica do firmware; escolhe o tradutor |
| `seq` | inteiro >= 0 | estritamente crescente por nó, sobrevive a reinício |
| `ts` | inteiro | segundos Unix UTC (SNTP) em que o lote foi lido; vira `read_at` |
| `r[].id` | string | identificador do sensor dentro do nó (chave da calibração) |
| `r[].k` | string | tipo de grandeza bruta: `soil_adc`, `air_temp_c`, `air_rh_pct`, `light_lux` |
| `r[].v` | número | valor bruto na unidade de `k` |

Campos desconhecidos devem ser ignorados pelos consumidores (evolução compatível). Vento, chuva, nível e vazão entram em versão posterior do firmware.

## Grandezas normalizadas

Saída do `DevicePayloadTranslator` e valor da coluna `kind` da telemetria. A fonte única no código é `contracts/quantity`; persistência, API, painéis Grafana e o serviço de irrigação só usam estes nomes (um teste em `contracts/quantity` falha se divergirem). Note que `air_rh_pct` e `light_lux` são tipos **brutos** do payload do firmware, não grandezas normalizadas.

| Grandeza (`kind`) | Unidade | Origem |
| --- | --- | --- |
| `soil_moisture_vwc_pct` | % | `soil_adc` calibrado (seco 0%, saturado 100%) |
| `air_temp_c` | °C | `air_temp_c` |
| `air_humidity_pct` | % | `air_rh_pct` |
| `illuminance_lux` | lux | `light_lux` |
| `solar_radiation_w_m2` | W/m² | estimada a partir de `light_lux` (qualidade `estimado`) |
| `water_level_ok` | 0/1 | `water_level_ok` |

## Nó atuador (válvula e vazão)

O nó atuador assina `cmd` e publica `estado` e `cmd/resposta`. O serviço de comandos (API) e o serviço de irrigação publicam comandos em MQTT 5 com Correlation Data igual ao `id` do comando e Message Expiry.

Comando (`.../cmd`, QoS 1):

```json
{"id": "01J...", "action": "open_valve", "duration_s": 60, "expires_at": 1763726400}
```

`action` é `open_valve` ou `close_valve`; `duration_s` é o tempo máximo (o firmware fecha sozinho, ADR 0004); o nó descarta comando com `expires_at` vencido.

Estado reportado (`.../estado`, QoS 1, retained), a cada mudança e periodicamente:

```json
{"valve": "open", "flow_lpm": 1.5, "ts": 1763726400}
```

| Campo | Tipo | Regra |
| --- | --- | --- |
| `valve` | string | `open` ou `closed` |
| `flow_lpm` | número ou ausente | vazão medida em L/min; ausente se não há medidor |
| `ts` | inteiro | segundos Unix UTC |

Resposta ao comando (`.../cmd/resposta`, QoS 1):

```json
{"id": "01J...", "accepted": true}
```

`id` repete o do comando (e o Correlation Data, que tem precedência). `accepted: false` indica recusa pelo intertravamento do firmware.
