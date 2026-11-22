#pragma once

#include <stdbool.h>

#include "esp_err.h"

typedef struct {
    const char *ca;
    const char *cert;
    const char *key;
} device_certs_t;

/*
 * Mapeia os PEM da partição "devcert" (CA, certificado, chave; 4 KiB cada, terminados em NUL).
 * Devolve ESP_ERR_NOT_FOUND quando o nó ainda não foi provisionado.
 */
esp_err_t cert_store_load(device_certs_t *out);
