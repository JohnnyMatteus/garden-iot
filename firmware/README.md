# Firmware do nó sensor

ESP32 com ESP-IDF (C). Ciclo: acorda, lê solo, ar, luz e nível de água, publica um lote JSON por
MQTT 5 (QoS 1, mTLS) e volta ao deep sleep. Sem rede, o lote vai para uma fila em flash e sai
na próxima conexão, com a sequência e o timestamp originais. Formato do payload:
[docs/payload-v1.md](docs/payload-v1.md) (implementa `contracts/mqtt/README.md`).

## Estrutura

| Caminho | O quê |
| --- | --- |
| `components/garden_core/` | lógica pura (payload, fila offline, mediana); testável no host |
| `main/` | ciclo do nó e drivers: sensores, Wi-Fi/SNTP, MQTT, certificados, NVS, deep sleep |
| `test/host/` | testes unitários (Unity) que rodam no PC, sem ESP-IDF |
| `test/golden/v1/` | payloads de referência, compartilhados com o tradutor Go |
| `partitions.csv` | `devcert` (certificados do dispositivo) e `offbuf` (fila offline) |
| `diagram.json`, `wokwi.toml` | simulação no Wokwi |

## Comandos

ESP-IDF não é instalado na máquina; o build usa a imagem oficial em versão fixada (a mesma do CI).

```bash
make -C firmware test    # testes unitários no host (gcc + cmake)
make -C firmware build   # build do firmware em Docker (espressif/idf:v5.4.1), alvo esp32
make -C firmware wokwi   # build com sdkconfig.defaults.wokwi, para simulação
```

## Segurança e credenciais

- Nenhuma senha ou chave privada está no código ou na imagem. CA, certificado e chave do nó ficam
  na partição `devcert`, gravados no provisionamento (M1-05). Sem certificado o nó se recusa a
  conectar, a menos que `CONFIG_GARDEN_ALLOW_INSECURE_MQTT=y`, ligado só em
  `sdkconfig.defaults.wokwi`.
- Wokwi: o simulador não faz o provisionamento de PKI, então a simulação roda sem TLS contra um
  broker público de teste. A validação de mTLS fica no ambiente local (M1-06) e no simulador de
  frota. Isso é uma simplificação consciente.
- Wi-Fi: a senha em `CONFIG_GARDEN_WIFI_PASSWORD` serve só para desenvolvimento; em campo ela virá
  do provisionamento.

## Sensores

| Grandeza | Simulação (Wokwi) | Hardware de referência |
| --- | --- | --- |
| Solo | potenciômetro no GPIO34 (ADC1) | capacitivo, mesmo pino |
| Temperatura e umidade do ar | DHT22 no GPIO4 | SHT31 ou BME280 (I²C), driver no M6 |
| Luz | BH1750 em I²C (21/22) | BH1750 |
| Nível de água | chave no GPIO27 | boia |

Vento, chuva e vazão ficam para o nó de vento e chuva (ver README das issues).

## Robustez

- Fila offline: 32 setores de 4 KiB, 8 slots de 512 B. Cheia, apaga o setor mais antigo e o
  contador `dropped` segue no lote seguinte. Capacidade útil de 248 a 256 lotes.
- Sequência persistida no NVS antes de publicar e conferida contra a fila ao subir.
- Watchdog de tarefa (`CONFIG_GARDEN_WDT_TIMEOUT_S`) com pânico e reinício; as esperas de rede
  realimentam o watchdog.
- OTA assinado com rollback e intertravamentos de atuador não fazem parte do M1.
