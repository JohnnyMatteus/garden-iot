#include "sensors.h"

#include <math.h>

#include "dht.h"
#include "driver/gpio.h"
#include "driver/i2c_master.h"
#include "esp_adc/adc_oneshot.h"
#include "esp_log.h"
#include "freertos/FreeRTOS.h"
#include "freertos/task.h"
#include "median.h"
#include "sdkconfig.h"

#define TAG "sensors"
#define SOIL_SAMPLES 9
#define BH1750_ADDR 0x23
#define BH1750_CONT_HRES 0x10
#define BH1750_POWER_ON 0x01

static adc_oneshot_unit_handle_t adc;
static adc_channel_t soil_channel;
static i2c_master_dev_handle_t bh1750;

void sensors_init(void)
{
    adc_unit_t unit;
    ESP_ERROR_CHECK(adc_oneshot_io_to_channel(CONFIG_GARDEN_SOIL_ADC_GPIO, &unit, &soil_channel));
    adc_oneshot_unit_init_cfg_t ucfg = {.unit_id = unit};
    ESP_ERROR_CHECK(adc_oneshot_new_unit(&ucfg, &adc));
    adc_oneshot_chan_cfg_t ccfg = {.atten = ADC_ATTEN_DB_12, .bitwidth = ADC_BITWIDTH_12};
    ESP_ERROR_CHECK(adc_oneshot_config_channel(adc, soil_channel, &ccfg));

    gpio_config_t water = {.pin_bit_mask = 1ULL << CONFIG_GARDEN_WATER_GPIO,
                           .mode = GPIO_MODE_INPUT,
                           .pull_up_en = GPIO_PULLUP_ENABLE};
    ESP_ERROR_CHECK(gpio_config(&water));

    i2c_master_bus_handle_t bus;
    i2c_master_bus_config_t bcfg = {.clk_source = I2C_CLK_SRC_DEFAULT,
                                    .i2c_port = I2C_NUM_0,
                                    .sda_io_num = CONFIG_GARDEN_I2C_SDA_GPIO,
                                    .scl_io_num = CONFIG_GARDEN_I2C_SCL_GPIO,
                                    .glitch_ignore_cnt = 7,
                                    .flags.enable_internal_pullup = true};
    ESP_ERROR_CHECK(i2c_new_master_bus(&bcfg, &bus));
    i2c_device_config_t dcfg = {.dev_addr_length = I2C_ADDR_BIT_LEN_7,
                                .device_address = BH1750_ADDR,
                                .scl_speed_hz = 100000};
    ESP_ERROR_CHECK(i2c_master_bus_add_device(bus, &dcfg, &bh1750));
}

static bool read_soil(uint16_t *out)
{
    uint16_t s[SOIL_SAMPLES];
    for (int i = 0; i < SOIL_SAMPLES; i++) {
        int raw;
        if (adc_oneshot_read(adc, soil_channel, &raw) != ESP_OK) {
            return false;
        }
        s[i] = (uint16_t)raw;
    }
    *out = median_u16(s, SOIL_SAMPLES);
    return true;
}

static bool read_lux(float *out)
{
    uint8_t cmd = BH1750_POWER_ON;
    if (i2c_master_transmit(bh1750, &cmd, 1, 100) != ESP_OK) {
        return false;
    }
    cmd = BH1750_CONT_HRES;
    if (i2c_master_transmit(bh1750, &cmd, 1, 100) != ESP_OK) {
        return false;
    }
    vTaskDelay(pdMS_TO_TICKS(180)); /* tempo de conversão em alta resolução */
    uint8_t d[2];
    if (i2c_master_receive(bh1750, d, 2, 100) != ESP_OK) {
        return false;
    }
    *out = (float)((d[0] << 8) | d[1]) / 1.2f;
    return true;
}

readings_t sensors_read(void)
{
    readings_t r = {0};
    uint16_t soil;
    if ((r.has_soil = read_soil(&soil))) {
        r.soil_adc = soil;
    } else {
        ESP_LOGW(TAG, "solo indisponível");
    }

    float rh, t;
    if ((r.has_air = dht_read_float_data(DHT_TYPE_AM2301, CONFIG_GARDEN_DHT_GPIO, &rh, &t) == ESP_OK)) {
        r.air_rh_pct = rh;
        r.air_temp_c = t;
    } else {
        ESP_LOGW(TAG, "ar indisponível");
    }

    if (!(r.has_lux = read_lux(&r.lux))) {
        ESP_LOGW(TAG, "luz indisponível");
    }

    r.has_water = true;
    r.water_ok = gpio_get_level(CONFIG_GARDEN_WATER_GPIO) == 1;
    return r;
}
