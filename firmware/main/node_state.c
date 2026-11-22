#include "node_state.h"

#include "nvs.h"

#define NS "garden"

esp_err_t node_state_load(node_state_t *st)
{
    *st = (node_state_t){.next_seq = 1};
    nvs_handle_t h;
    esp_err_t err = nvs_open(NS, NVS_READONLY, &h);
    if (err == ESP_ERR_NVS_NOT_FOUND) {
        return ESP_OK;
    }
    if (err != ESP_OK) {
        return err;
    }
    uint32_t v;
    if (nvs_get_u32(h, "next_seq", &v) == ESP_OK) {
        st->next_seq = v;
    }
    if (nvs_get_u32(h, "dropped", &v) == ESP_OK) {
        st->dropped = v;
    }
    nvs_close(h);
    return ESP_OK;
}

esp_err_t node_state_save(const node_state_t *st)
{
    nvs_handle_t h;
    esp_err_t err = nvs_open(NS, NVS_READWRITE, &h);
    if (err != ESP_OK) {
        return err;
    }
    err = nvs_set_u32(h, "next_seq", st->next_seq);
    if (err == ESP_OK) {
        err = nvs_set_u32(h, "dropped", st->dropped);
    }
    if (err == ESP_OK) {
        err = nvs_commit(h);
    }
    nvs_close(h);
    return err;
}
