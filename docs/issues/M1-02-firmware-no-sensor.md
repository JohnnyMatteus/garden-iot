# M1-02 · Firmware do nó sensor: leitura, lote e publicação MQTT

- **core:** false
- **curso:** —
- **subagente:** implementador (C, ESP-IDF)
- **depende de:** M1-01
- **estimativa:** 1 dia

## Objetivo

Implementar o ciclo do nó sensor da seção 4.1 (acorda, lê, publica em lote, volta ao deep sleep) com publicação MQTT 5 QoS 1 em `horta/{tenant}/{horta}/{no}/telemetria`, timestamp por SNTP, sequência monotônica por nó e versão do firmware no payload.

## Critérios de aceite

```gherkin
Funcionalidade: Firmware do nó sensor

  Cenário: Lote publicado
    Dado o nó simulado no Wokwi com sensores de solo, ar e luz
    Quando o ciclo de leitura roda
    Então um lote com todas as leituras é publicado com QoS 1 no tópico de telemetria do nó
    E cada lote traz versão do firmware, sequência e timestamp SNTP em UTC

  Cenário: Sequência monotônica
    Dado três ciclos seguidos, com reinício do nó entre o segundo e o terceiro
    Então as sequências publicadas são estritamente crescentes

  Cenário: Last Will
    Dado o nó conectado
    Então ele registrou Last Will em horta/{tenant}/{horta}/{no}/lwt

  Cenário: Sem credencial em texto
    Dado o binário do firmware
    Então nenhuma senha ou chave privada aparece em texto no código-fonte
```

## Arquivos prováveis

- `firmware/main/sensor_node.c`
- `firmware/main/telemetry_payload.c|h`
- `firmware/diagram.json`, `firmware/wokwi.toml`

## Notas

- O formato do payload MQTT não está definido no design doc. Esta issue propõe um formato versionado simples (ex.: JSON compacto ou CBOR com `fw_version`); ele vira o contrato da v1 do `DevicePayloadTranslator` (M1-08).
- Vento, chuva e nível/vazão (pulsos) ficam para quando houver nó de vento e chuva simulado; o M1 cobre solo, ar e luz. Ver pontos em aberto no README.
- O nó usa certificado de dispositivo (M1-05); no Wokwi, o armazenamento seguro do certificado é simplificado e isso precisa estar documentado.
