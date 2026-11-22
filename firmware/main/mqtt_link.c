#include "mqtt_link.h"

#include "cert_store.h"
#include "config.h"
#include "esp_log.h"
#include "esp_task_wdt.h"
#include "freertos/FreeRTOS.h"
#include "freertos/event_groups.h"
#include "mqtt_client.h"

#define TAG "mqtt"
#define CONNECTED BIT0
#define ACKED BIT1
#define ACK_TIMEOUT_MS 10000

static esp_mqtt_client_handle_t client;
static EventGroupHandle_t events;
static volatile int pending_id = -1;

static void on_mqtt(void *arg, esp_event_base_t base, int32_t id, void *data)
{
    (void)arg;
    (void)base;
    esp_mqtt_event_handle_t ev = data;
    switch ((esp_mqtt_event_id_t)id) {
    case MQTT_EVENT_CONNECTED:
        xEventGroupSetBits(events, CONNECTED);
        break;
    case MQTT_EVENT_DISCONNECTED:
        xEventGroupClearBits(events, CONNECTED);
        break;
    case MQTT_EVENT_PUBLISHED:
        /* Limite conhecido (ADR 0013): o esp-mqtt do ESP-IDF 5.4.1 não expõe o reason code do PUBACK, então
         * um 0x10 (nenhum assinante recebeu o lote) é indistinguível de uma entrega confirmada. */
        if (ev->msg_id == pending_id) {
            xEventGroupSetBits(events, ACKED);
        }
        break;
    default:
        break;
    }
}

bool mqtt_link_connect(int timeout_ms)
{
    events = xEventGroupCreate();
    device_certs_t certs;
    bool have_certs = cert_store_load(&certs) == ESP_OK;
#ifndef CONFIG_GARDEN_ALLOW_INSECURE_MQTT
    if (!have_certs) {
        ESP_LOGE(TAG, "nó sem certificado de dispositivo; mTLS é obrigatório");
        return false;
    }
#endif

    esp_mqtt_client_config_t cfg = {
        .broker.address.uri = CONFIG_GARDEN_MQTT_URI,
        .credentials.client_id = CONFIG_GARDEN_TENANT "/" CONFIG_GARDEN_HORTA "/" CONFIG_GARDEN_NODE_ID, /* = CN do certificado */
        .session.protocol_ver = MQTT_PROTOCOL_V_5,
        .session.last_will = {.topic = TOPIC_LWT, .msg = "{\"online\":false}", .qos = 1, .retain = 0},
    };
    if (have_certs) {
        cfg.broker.verification.certificate = certs.ca;
        cfg.credentials.authentication.certificate = certs.cert;
        cfg.credentials.authentication.key = certs.key;
    }
    client = esp_mqtt_client_init(&cfg);
    if (client == NULL) {
        return false;
    }
    esp_mqtt_client_register_event(client, ESP_EVENT_ANY_ID, on_mqtt, NULL);
    esp_mqtt_client_start(client);

    for (int waited = 0; waited < timeout_ms; waited += 500) {
        if (xEventGroupWaitBits(events, CONNECTED, pdFALSE, pdTRUE, pdMS_TO_TICKS(500)) & CONNECTED) {
            return true;
        }
        esp_task_wdt_reset();
    }
    ESP_LOGW(TAG, "broker indisponível");
    return false;
}

int mqtt_link_publish_batch(void *ctx, const void *payload, size_t len)
{
    (void)ctx;
    xEventGroupClearBits(events, ACKED);
    pending_id = esp_mqtt_client_publish(client, TOPIC_TELEMETRY, payload, (int)len, 1, 0);
    if (pending_id < 0) {
        return -1;
    }
    for (int waited = 0; waited < ACK_TIMEOUT_MS; waited += 500) {
        if (xEventGroupWaitBits(events, ACKED, pdTRUE, pdTRUE, pdMS_TO_TICKS(500)) & ACKED) {
            return 0;
        }
        esp_task_wdt_reset();
    }
    return -1;
}

void mqtt_link_close(void)
{
    if (client != NULL) {
        esp_mqtt_client_stop(client);
        esp_mqtt_client_destroy(client);
        client = NULL;
    }
}
