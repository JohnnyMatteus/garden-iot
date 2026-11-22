#pragma once

/*
 * Fila circular de lotes em flash (M1-03).
 *
 * Layout: setores de sector_size bytes divididos em slots de OB_SLOT_SIZE. Um slot é escrito uma
 * vez (flash só vai de 1 para 0) e "consumido" zerando o byte de estado, sem apagar. Quando o
 * próximo slot de escrita ainda tem registros vivos (fila cheia), o setor inteiro, o mais antigo,
 * é apagado e os registros vivos contam como descartados.
 * A capacidade útil fica entre (setores-1)*slots_por_setor e setores*slots_por_setor.
 */

#include <stdbool.h>
#include <stddef.h>
#include <stdint.h>

#define OB_SLOT_SIZE 256u
#define OB_HEADER_SIZE 16u
#define OB_MAX_PAYLOAD (OB_SLOT_SIZE - OB_HEADER_SIZE)

typedef struct {
    void *ctx;
    size_t sector_size; /* múltiplo de OB_SLOT_SIZE */
    size_t sector_count;
    int (*read)(void *ctx, size_t offset, void *dst, size_t len);
    int (*write)(void *ctx, size_t offset, const void *src, size_t len);
    int (*erase_sector)(void *ctx, size_t sector);
} ob_storage_t;

typedef enum {
    OB_OK = 0,
    OB_EMPTY = 1,
    OB_ERR_IO = -1,
    OB_ERR_TOO_BIG = -2,
    OB_ERR_CONFIG = -3,
} ob_status_t;

typedef struct {
    ob_storage_t st;
    size_t slots_total;
    size_t slots_per_sector;
    size_t tail; /* próximo slot de escrita */
    size_t live;
    uint32_t max_seq;
    bool has_seq;
    uint32_t dropped;
} ob_t;

typedef struct {
    size_t slot;
    uint32_t seq;
    size_t len;
    uint8_t data[OB_MAX_PAYLOAD];
} ob_record_t;

/* Varre a flash e reconstrói o estado (sobrevive a reinício e deep sleep). */
ob_status_t ob_init(ob_t *ob, const ob_storage_t *storage);
ob_status_t ob_push(ob_t *ob, uint32_t seq, const void *data, size_t len);
/* Menor seq ainda não consumida. OB_EMPTY quando não há registros. */
ob_status_t ob_peek_oldest(ob_t *ob, ob_record_t *rec);
ob_status_t ob_pop(ob_t *ob, const ob_record_t *rec);

size_t ob_count(const ob_t *ob);
/* Maior seq já gravada, para nunca reutilizar sequência após reinício. */
bool ob_max_seq(const ob_t *ob, uint32_t *seq);
/* Registros descartados por buffer cheio desde a última chamada; zera o contador. */
uint32_t ob_take_dropped(ob_t *ob);

/* Publica um lote; devolve 0 em sucesso. */
typedef int (*ob_publish_fn)(void *ctx, const void *payload, size_t len);

/*
 * Envia primeiro o que está na fila (mais antigo primeiro, com timestamp e seq originais) e
 * depois o lote atual; o que falhar vai para a fila. Nunca publica o novo antes dos antigos.
 * Devolve quantos lotes saíram em *published (opcional).
 */
ob_status_t ob_send_or_buffer(ob_t *ob, uint32_t seq, const void *data, size_t len,
                              ob_publish_fn publish, void *ctx, size_t *published);
