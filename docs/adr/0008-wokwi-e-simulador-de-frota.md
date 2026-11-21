# 0008. Wokwi + simulador de frota como ambiente padrão de desenvolvimento (G-08)

- **Status:** Proposto
- **Data:** 2026-11-21
- **Id no design doc:** G-08 (seção 13; detalhe na seção 4.3)

## Contexto

A complexidade de hardware pode atrasar o projeto (seção 15), e testes de regressão de firmware, carga e caos não são viáveis só com placas físicas.

## Decisão

O desenvolvimento usa simulação por padrão: o Wokwi simula o ESP32 com Wi-Fi e MQTT, e o `wokwi-cli` roda no CI validando a saída serial (`--expect-text`); um simulador de frota em Go gera telemetria de N nós com perfis (dia ensolarado, chuva, vento forte, sensor travado, nó que cai) para testes de carga e de regras. Hardware real só no M6.

## Alternativas consideradas

- Desenvolver com hardware real desde o início: descartado pelo próprio design doc ("Wokwi e simulador primeiro; hardware real só no M6", seção 15).

## Consequências

- Firmware testável no CI sem placa; carga de 1.000 nós reproduzível (seção 11).
- A simulação não cobre tudo do hardware (ruído real de sensor, consumo de bateria); o M6 valida num canteiro real.
