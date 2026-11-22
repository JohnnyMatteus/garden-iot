#include "power.h"

#include "esp_log.h"
#include "esp_sleep.h"

void power_deep_sleep(unsigned seconds)
{
    ESP_LOGI("power", "deep sleep por %u s", seconds);
    esp_deep_sleep((uint64_t)seconds * 1000000ULL);
}
