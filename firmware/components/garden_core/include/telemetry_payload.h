#pragma once

#include <stdbool.h>
#include <stddef.h>
#include <stdint.h>

/* Leituras de um ciclo. Sensor que falhou fica com has_* = false e o campo é omitido. */
typedef struct {
    bool has_soil;
    uint16_t soil_adc;
    bool has_air;
    float air_temp_c;
    float air_rh_pct;
    bool has_lux;
    float lux;
    bool has_water;
    bool water_ok;
} readings_t;

typedef struct {
    const char *fw_version;
    uint32_t seq;
    int64_t ts; /* epoch UTC em segundos; 0 quando o SNTP ainda não sincronizou */
    uint32_t dropped;
    readings_t readings;
} telemetry_batch_t;

/* Gera o JSON do payload v1 (ver docs/payload-v1.md). Retorna o tamanho ou -1 se não couber. */
int telemetry_payload_encode(const telemetry_batch_t *batch, char *buf, size_t cap);
