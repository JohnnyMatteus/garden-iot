#pragma once

#include <stdint.h>

#include "esp_err.h"

typedef struct {
    uint32_t next_seq;
    uint32_t dropped; /* descartes por buffer cheio ainda não reportados */
} node_state_t;

esp_err_t node_state_load(node_state_t *st);
esp_err_t node_state_save(const node_state_t *st);
