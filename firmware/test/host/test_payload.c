#include <stdio.h>
#include <string.h>

#include "telemetry_payload.h"
#include "unity.h"

void setUp(void) {}
void tearDown(void) {}

static telemetry_batch_t full_batch(void)
{
    telemetry_batch_t b = {.fw_version = "1.0.0", .seq = 42, .ts = 1732190000, .dropped = 0};
    b.readings = (readings_t){.has_soil = true, .soil_adc = 2100, .has_air = true,
                              .air_temp_c = 23.5f, .air_rh_pct = 61.25f, .has_lux = true,
                              .lux = 12000.0f, .has_water = true, .water_ok = true};
    return b;
}

static void read_golden(const char *name, char *out, size_t cap)
{
    char path[512];
    snprintf(path, sizeof path, "%s/%s", GOLDEN_DIR, name);
    FILE *f = fopen(path, "r");
    TEST_ASSERT_NOT_NULL_MESSAGE(f, path);
    size_t n = fread(out, 1, cap - 1, f);
    fclose(f);
    while (n > 0 && (out[n - 1] == '\n' || out[n - 1] == '\r')) {
        n--;
    }
    out[n] = '\0';
}

static void test_lote_completo_confere_com_golden(void)
{
    telemetry_batch_t b = full_batch();
    char buf[256], golden[256];
    int n = telemetry_payload_encode(&b, buf, sizeof buf);
    TEST_ASSERT_GREATER_THAN(0, n);
    read_golden("completo.json", golden, sizeof golden);
    TEST_ASSERT_EQUAL_STRING(golden, buf);
}

static void test_sensor_ausente_omite_campo_e_leva_descartes(void)
{
    telemetry_batch_t b = full_batch();
    b.readings.has_air = false;
    b.dropped = 3;
    char buf[256], golden[256];
    TEST_ASSERT_GREATER_THAN(0, telemetry_payload_encode(&b, buf, sizeof buf));
    read_golden("sem_ar_com_descartes.json", golden, sizeof golden);
    TEST_ASSERT_EQUAL_STRING(golden, buf);
}

static void test_nao_cabe_devolve_erro(void)
{
    telemetry_batch_t b = full_batch();
    char buf[20];
    TEST_ASSERT_EQUAL_INT(-1, telemetry_payload_encode(&b, buf, sizeof buf));
}

static void test_cabe_no_slot_do_buffer(void)
{
    telemetry_batch_t b = full_batch();
    b.dropped = 4294967295u;
    b.seq = 4294967295u;
    char buf[256];
    int n = telemetry_payload_encode(&b, buf, sizeof buf);
    TEST_ASSERT_TRUE(n > 0 && n <= 240);
}

int main(void)
{
    UNITY_BEGIN();
    RUN_TEST(test_lote_completo_confere_com_golden);
    RUN_TEST(test_sensor_ausente_omite_campo_e_leva_descartes);
    RUN_TEST(test_nao_cabe_devolve_erro);
    RUN_TEST(test_cabe_no_slot_do_buffer);
    return UNITY_END();
}
