#include "median.h"
#include "unity.h"

void setUp(void) {}
void tearDown(void) {}

static void test_median_descarta_pico(void)
{
    uint16_t s[5] = {2100, 4095, 2098, 0, 2102};
    TEST_ASSERT_EQUAL_UINT16(2100, median_u16(s, 5));
}

static void test_median_uma_amostra(void)
{
    uint16_t s[1] = {7};
    TEST_ASSERT_EQUAL_UINT16(7, median_u16(s, 1));
}

int main(void)
{
    UNITY_BEGIN();
    RUN_TEST(test_median_descarta_pico);
    RUN_TEST(test_median_uma_amostra);
    return UNITY_END();
}
