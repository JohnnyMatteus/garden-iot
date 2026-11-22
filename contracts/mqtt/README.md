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
