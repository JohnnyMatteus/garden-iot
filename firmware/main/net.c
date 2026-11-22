#include "net.h"

#include <string.h>
#include <time.h>

#include "esp_event.h"
#include "esp_log.h"
#include "esp_netif.h"
#include "esp_netif_sntp.h"
#include "esp_task_wdt.h"
#include "esp_wifi.h"
#include "freertos/FreeRTOS.h"
#include "freertos/event_groups.h"
#include "sdkconfig.h"

#define TAG "net"
#define GOT_IP BIT0
/* Qualquer epoch anterior a 2023 indica relógio não sincronizado. */
#define MIN_VALID_EPOCH 1672531200LL

static EventGroupHandle_t events;

static void on_event(void *arg, esp_event_base_t base, int32_t id, void *data)
{
    (void)arg;
    (void)data;
    if (base == WIFI_EVENT && id == WIFI_EVENT_STA_START) {
        esp_wifi_connect();
    } else if (base == WIFI_EVENT && id == WIFI_EVENT_STA_DISCONNECTED) {
        xEventGroupClearBits(events, GOT_IP);
        esp_wifi_connect();
    } else if (base == IP_EVENT && id == IP_EVENT_STA_GOT_IP) {
        xEventGroupSetBits(events, GOT_IP);
    }
}

bool net_connect(int timeout_ms)
{
    events = xEventGroupCreate();
    ESP_ERROR_CHECK(esp_netif_init());
    ESP_ERROR_CHECK(esp_event_loop_create_default());
    esp_netif_create_default_wifi_sta();
    wifi_init_config_t cfg = WIFI_INIT_CONFIG_DEFAULT();
    ESP_ERROR_CHECK(esp_wifi_init(&cfg));
    ESP_ERROR_CHECK(esp_event_handler_register(WIFI_EVENT, ESP_EVENT_ANY_ID, on_event, NULL));
    ESP_ERROR_CHECK(esp_event_handler_register(IP_EVENT, IP_EVENT_STA_GOT_IP, on_event, NULL));

    wifi_config_t wc = {0};
    strlcpy((char *)wc.sta.ssid, CONFIG_GARDEN_WIFI_SSID, sizeof wc.sta.ssid);
    strlcpy((char *)wc.sta.password, CONFIG_GARDEN_WIFI_PASSWORD, sizeof wc.sta.password);
    ESP_ERROR_CHECK(esp_wifi_set_mode(WIFI_MODE_STA));
    ESP_ERROR_CHECK(esp_wifi_set_config(WIFI_IF_STA, &wc));
    ESP_ERROR_CHECK(esp_wifi_start());

    for (int waited = 0; waited < timeout_ms; waited += 500) {
        if (xEventGroupWaitBits(events, GOT_IP, pdFALSE, pdTRUE, pdMS_TO_TICKS(500)) & GOT_IP) {
            ESP_LOGI(TAG, "wi-fi conectado");
            return true;
        }
        esp_task_wdt_reset();
    }
    ESP_LOGW(TAG, "wi-fi indisponível");
    return false;
}

long long net_now_utc(void)
{
    time_t now = time(NULL);
    return now >= MIN_VALID_EPOCH ? (long long)now : 0;
}

bool net_sync_time(int timeout_ms)
{
    if (net_now_utc() != 0) {
        return true; /* o relógio sobrevive ao deep sleep */
    }
    esp_sntp_config_t cfg = ESP_NETIF_SNTP_DEFAULT_CONFIG("pool.ntp.org");
    ESP_ERROR_CHECK(esp_netif_sntp_init(&cfg));
    for (int waited = 0; waited < timeout_ms; waited += 500) {
        if (esp_netif_sntp_sync_wait(pdMS_TO_TICKS(500)) == ESP_OK) {
            ESP_LOGI(TAG, "sntp sincronizado");
            return true;
        }
        esp_task_wdt_reset();
    }
    ESP_LOGW(TAG, "sntp indisponível");
    return false;
}
