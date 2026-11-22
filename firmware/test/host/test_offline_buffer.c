#include <stdio.h>
#include <stdlib.h>
#include <string.h>

#include "offline_buffer.h"
#include "unity.h"

#define SECTORS 3
#define SECTOR_SIZE 1024u /* 4 slots por setor: 12 slots */

static uint8_t flash[SECTORS * SECTOR_SIZE];
static ob_storage_t st;
static ob_t ob;

/* Emula flash NOR: escrita só limpa bits (AND), apagar volta a 0xFF. */
static int f_read(void *c, size_t off, void *dst, size_t len)
{
    (void)c;
    memcpy(dst, flash + off, len);
    return 0;
}
static int f_write(void *c, size_t off, const void *src, size_t len)
{
    (void)c;
    for (size_t i = 0; i < len; i++) {
        flash[off + i] &= ((const uint8_t *)src)[i];
    }
    return 0;
}
static int f_erase(void *c, size_t sector)
{
    (void)c;
    memset(flash + sector * SECTOR_SIZE, 0xFF, SECTOR_SIZE);
    return 0;
}

void setUp(void)
{
    memset(flash, 0xFF, sizeof flash);
    st = (ob_storage_t){.sector_size = SECTOR_SIZE, .sector_count = SECTORS, .read = f_read,
                        .write = f_write, .erase_sector = f_erase};
    TEST_ASSERT_EQUAL(OB_OK, ob_init(&ob, &st));
}
void tearDown(void) {}

static void push_n(uint32_t first, uint32_t n)
{
    for (uint32_t s = first; s < first + n; s++) {
        char p[32];
        int len = snprintf(p, sizeof p, "lote-%lu", (unsigned long)s);
        TEST_ASSERT_EQUAL(OB_OK, ob_push(&ob, s, p, (size_t)len));
    }
}

static void test_fifo_com_seq_original(void)
{
    push_n(10, 3);
    for (uint32_t s = 10; s < 13; s++) {
        ob_record_t r;
        TEST_ASSERT_EQUAL(OB_OK, ob_peek_oldest(&ob, &r));
        TEST_ASSERT_EQUAL_UINT32(s, r.seq);
        TEST_ASSERT_EQUAL(OB_OK, ob_pop(&ob, &r));
    }
    ob_record_t r;
    TEST_ASSERT_EQUAL(OB_EMPTY, ob_peek_oldest(&ob, &r));
}

static void test_sobrevive_a_reinicio(void)
{
    push_n(1, 5);
    ob_record_t r;
    ob_peek_oldest(&ob, &r);
    ob_pop(&ob, &r);

    ob_t again;
    TEST_ASSERT_EQUAL(OB_OK, ob_init(&again, &st));
    TEST_ASSERT_EQUAL(4, ob_count(&again));
    uint32_t max;
    TEST_ASSERT_TRUE(ob_max_seq(&again, &max));
    TEST_ASSERT_EQUAL_UINT32(5, max);
    TEST_ASSERT_EQUAL(OB_OK, ob_push(&again, 6, "x", 1));
    TEST_ASSERT_EQUAL(OB_OK, ob_peek_oldest(&again, &r));
    TEST_ASSERT_EQUAL_UINT32(2, r.seq);
}

static void test_buffer_cheio_descarta_o_mais_antigo_e_conta(void)
{
    push_n(1, 12);
    TEST_ASSERT_EQUAL(12, ob_count(&ob));
    TEST_ASSERT_EQUAL_UINT32(0, ob_take_dropped(&ob));

    push_n(13, 1); /* apaga o setor mais antigo: seq 1..4 */
    TEST_ASSERT_EQUAL_UINT32(4, ob_take_dropped(&ob));
    TEST_ASSERT_EQUAL_UINT32(0, ob_take_dropped(&ob));
    ob_record_t r;
    TEST_ASSERT_EQUAL(OB_OK, ob_peek_oldest(&ob, &r));
    TEST_ASSERT_EQUAL_UINT32(5, r.seq);
    TEST_ASSERT_EQUAL(9, ob_count(&ob));
}

static void test_slot_corrompido_e_ignorado(void)
{
    push_n(1, 3);
    flash[256 + 20] ^= 0x01; /* corrompe o payload do 2o slot */
    ob_t again;
    TEST_ASSERT_EQUAL(OB_OK, ob_init(&again, &st));
    TEST_ASSERT_EQUAL(2, ob_count(&again));
    ob_record_t r;
    ob_peek_oldest(&again, &r);
    ob_pop(&again, &r);
    ob_peek_oldest(&again, &r);
    TEST_ASSERT_EQUAL_UINT32(3, r.seq);
}

static void test_payload_grande_demais(void)
{
    uint8_t big[OB_MAX_PAYLOAD + 1] = {0};
    TEST_ASSERT_EQUAL(OB_ERR_TOO_BIG, ob_push(&ob, 1, big, sizeof big));
}

typedef struct {
    int fail_after; /* publica com sucesso N vezes e depois falha; -1 = nunca falha */
    uint32_t seen[32];
    int n;
} pub_t;

static int fake_publish(void *c, const void *payload, size_t len)
{
    pub_t *p = c;
    if (p->fail_after == 0) {
        return -1;
    }
    if (p->fail_after > 0) {
        p->fail_after--;
    }
    char buf[32] = {0};
    memcpy(buf, payload, len < 31 ? len : 31);
    p->seen[p->n++] = (uint32_t)strtoul(buf + 5, NULL, 10);
    return 0;
}

static void test_reenvio_apos_queda_de_3_ciclos(void)
{
    pub_t down = {.fail_after = 0};
    for (uint32_t s = 1; s <= 3; s++) {
        char p[16];
        int n = snprintf(p, sizeof p, "lote-%lu", (unsigned long)s);
        TEST_ASSERT_EQUAL(OB_OK, ob_send_or_buffer(&ob, s, p, (size_t)n, fake_publish, &down, NULL));
    }
    TEST_ASSERT_EQUAL(3, ob_count(&ob));

    pub_t up = {.fail_after = -1};
    size_t sent;
    TEST_ASSERT_EQUAL(OB_OK, ob_send_or_buffer(&ob, 4, "lote-4", 6, fake_publish, &up, &sent));
    TEST_ASSERT_EQUAL(4, sent);
    TEST_ASSERT_EQUAL(0, ob_count(&ob));
    for (int i = 0; i < 4; i++) {
        TEST_ASSERT_EQUAL_UINT32((uint32_t)i + 1, up.seen[i]);
    }
}

static void test_falha_no_meio_preserva_ordem(void)
{
    pub_t down = {.fail_after = 0};
    ob_send_or_buffer(&ob, 1, "lote-1", 6, fake_publish, &down, NULL);
    ob_send_or_buffer(&ob, 2, "lote-2", 6, fake_publish, &down, NULL);

    pub_t flaky = {.fail_after = 1};
    ob_send_or_buffer(&ob, 3, "lote-3", 6, fake_publish, &flaky, NULL);
    TEST_ASSERT_EQUAL(1, flaky.n);
    TEST_ASSERT_EQUAL_UINT32(1, flaky.seen[0]);
    TEST_ASSERT_EQUAL(2, ob_count(&ob)); /* 2 e 3 ficam na fila, nessa ordem */
    ob_record_t r;
    ob_peek_oldest(&ob, &r);
    TEST_ASSERT_EQUAL_UINT32(2, r.seq);
}

int main(void)
{
    UNITY_BEGIN();
    RUN_TEST(test_fifo_com_seq_original);
    RUN_TEST(test_sobrevive_a_reinicio);
    RUN_TEST(test_buffer_cheio_descarta_o_mais_antigo_e_conta);
    RUN_TEST(test_slot_corrompido_e_ignorado);
    RUN_TEST(test_payload_grande_demais);
    RUN_TEST(test_reenvio_apos_queda_de_3_ciclos);
    RUN_TEST(test_falha_no_meio_preserva_ordem);
    return UNITY_END();
}
