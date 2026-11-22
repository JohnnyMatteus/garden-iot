/*
 * Ciclo do nó sensor (design doc 4.1): acorda, lê, publica em lote e volta ao deep sleep.
 * O estado (sequência, descartes) vive em NVS e a fila de lotes em flash, então um reinício por
 * watchdog continua de onde parou.
 */
#include <stdio.h>

#include "config.h"
#include "esp_log.h"
#include "esp_task_wdt.h"
#include "flash_storage.h"
#include "mqtt_link.h"
#include "net.h"
#include "node_state.h"
#include "nvs_flash.h"
#include "offline_buffer.h"
#include "power.h"
#include "sdkconfig.h"
#include "sensors.h"
#include "telemetry_payload.h"

#define TAG "node"
#define WIFI_TIMEOUT_MS 20000
#define SNTP_TIMEOUT_MS 10000
#define MQTT_TIMEOUT_MS 15000

static void init_watchdog(void)
{
    const esp_task_wdt_config_t cfg = {.timeout_ms = CONFIG_GARDEN_WDT_TIMEOUT_S * 1000,
                                       .idle_core_mask = 0,
                                       .trigger_panic = true};
    ESP_ERROR_CHECK(esp_task_wdt_init(&cfg));
    ESP_ERROR_CHECK(esp_task_wdt_add(NULL));
}

static void init_nvs(void)
{
    esp_err_t err = nvs_flash_init();
    if (err == ESP_ERR_NVS_NO_FREE_PAGES || err == ESP_ERR_NVS_NEW_VERSION_FOUND) {
        ESP_ERROR_CHECK(nvs_flash_erase());
        err = nvs_flash_init();
    }
    ESP_ERROR_CHECK(err);
}

static int no_publish(void *ctx, const void *payload, size_t len)
{
    (void)ctx;
    (void)payload;
    (void)len;
    return -1;
}

void app_main(void)
{
    init_nvs();
    init_watchdog();

    node_state_t state;
    ESP_ERROR_CHECK(node_state_load(&state));

    ob_storage_t storage;
    ob_t outbox;
    ESP_ERROR_CHECK(flash_storage_open(&storage));
    if (ob_init(&outbox, &storage) != OB_OK) {
        ESP_LOGE(TAG, "fila offline ilegível; reiniciando");
        esp_restart();
    }

    /* Nunca reusar uma sequência já gravada, mesmo que o NVS esteja atrasado. */
    uint32_t buffered_max;
    if (ob_max_seq(&outbox, &buffered_max) && buffered_max >= state.next_seq) {
        state.next_seq = buffered_max + 1;
    }

    sensors_init();
    readings_t readings = sensors_read();

    bool online = net_connect(WIFI_TIMEOUT_MS);
    if (online) {
        net_sync_time(SNTP_TIMEOUT_MS);
    }

    telemetry_batch_t batch = {.fw_version = CONFIG_GARDEN_FW_VERSION,
                               .seq = state.next_seq,
                               .ts = net_now_utc(),
                               .dropped = state.dropped,
                               .readings = readings};
    char payload[OB_MAX_PAYLOAD];
    int len = telemetry_payload_encode(&batch, payload, sizeof payload);
    if (len < 0) {
        ESP_LOGE(TAG, "payload não coube; descartando ciclo");
        power_deep_sleep(CONFIG_GARDEN_CYCLE_SECONDS);
    }

    /* Persiste a sequência antes de publicar: reinício no meio do ciclo não a repete. */
    state.next_seq++;
    ESP_ERROR_CHECK(node_state_save(&state));

    bool link_up = online && mqtt_link_connect(MQTT_TIMEOUT_MS);
    size_t published = 0;
    ob_status_t s = ob_send_or_buffer(&outbox, batch.seq, payload, (size_t)len,
                                      link_up ? mqtt_link_publish_batch : no_publish, NULL,
                                      &published);
    if (s != OB_OK) {
        ESP_LOGE(TAG, "falha na fila offline (%d)", s);
    }

    state.dropped = ob_take_dropped(&outbox);
    ESP_ERROR_CHECK(node_state_save(&state));

    if (published > 0 && ob_count(&outbox) == 0) {
        ESP_LOGI(TAG, "lote publicado seq=%lu (%u no total, fila vazia)", (unsigned long)batch.seq,
                 (unsigned)published);
    } else {
        ESP_LOGW(TAG, "lote enfileirado seq=%lu (fila=%u)", (unsigned long)batch.seq,
                 (unsigned)ob_count(&outbox));
    }

    if (link_up) {
        mqtt_link_close();
    }
    power_deep_sleep(CONFIG_GARDEN_CYCLE_SECONDS);
}
