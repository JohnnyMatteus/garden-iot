#include "flash_storage.h"

#include "esp_partition.h"

static const esp_partition_t *part;

static int rd(void *c, size_t off, void *dst, size_t len)
{
    (void)c;
    return esp_partition_read(part, off, dst, len) == ESP_OK ? 0 : -1;
}

static int wr(void *c, size_t off, const void *src, size_t len)
{
    (void)c;
    return esp_partition_write(part, off, src, len) == ESP_OK ? 0 : -1;
}

static int er(void *c, size_t sector)
{
    (void)c;
    return esp_partition_erase_range(part, sector * part->erase_size, part->erase_size) == ESP_OK
               ? 0
               : -1;
}

esp_err_t flash_storage_open(ob_storage_t *out)
{
    part = esp_partition_find_first(ESP_PARTITION_TYPE_DATA, 0x41, "offbuf");
    if (part == NULL) {
        return ESP_ERR_NOT_FOUND;
    }
    *out = (ob_storage_t){
        .sector_size = part->erase_size,
        .sector_count = part->size / part->erase_size,
        .read = rd,
        .write = wr,
        .erase_sector = er,
    };
    return ESP_OK;
}
