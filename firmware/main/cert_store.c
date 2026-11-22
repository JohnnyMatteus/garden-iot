#include "cert_store.h"

#include <string.h>

#include "esp_partition.h"

#define BLOB_SIZE 0x1000

static bool is_pem(const char *p)
{
    return strnlen(p, BLOB_SIZE) < BLOB_SIZE && strncmp(p, "-----BEGIN", 10) == 0;
}

esp_err_t cert_store_load(device_certs_t *out)
{
    const esp_partition_t *p = esp_partition_find_first(ESP_PARTITION_TYPE_DATA, 0x40, "devcert");
    if (p == NULL) {
        return ESP_ERR_NOT_FOUND;
    }
    const void *base;
    esp_partition_mmap_handle_t handle;
    esp_err_t err = esp_partition_mmap(p, 0, 3 * BLOB_SIZE, ESP_PARTITION_MMAP_DATA, &base, &handle);
    if (err != ESP_OK) {
        return err;
    }
    const char *b = base;
    if (!is_pem(b) || !is_pem(b + BLOB_SIZE) || !is_pem(b + 2 * BLOB_SIZE)) {
        return ESP_ERR_NOT_FOUND;
    }
    *out = (device_certs_t){.ca = b, .cert = b + BLOB_SIZE, .key = b + 2 * BLOB_SIZE};
    return ESP_OK;
}
