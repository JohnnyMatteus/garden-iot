#include "offline_buffer.h"

#include <string.h>

#define MAGIC 0xA55Au
#define STATE_LIVE 0xFFu
#define STATE_CONSUMED 0x00u
#define STATE_OFFSET 12u

typedef struct {
    uint16_t magic;
    uint16_t len;
    uint32_t seq;
    uint32_t crc;
    uint8_t state;
    uint8_t pad[3];
} header_t;

_Static_assert(sizeof(header_t) == OB_HEADER_SIZE, "header deve ter 16 bytes");

typedef enum { SLOT_BLANK, SLOT_LIVE, SLOT_CONSUMED, SLOT_CORRUPT } slot_kind_t;

static uint32_t crc32(const uint8_t *p, size_t n, uint32_t crc)
{
    crc = ~crc;
    while (n--) {
        crc ^= *p++;
        for (int k = 0; k < 8; k++) {
            crc = (crc >> 1) ^ (0xEDB88320u & (0u - (crc & 1u)));
        }
    }
    return ~crc;
}

static uint32_t record_crc(const header_t *h, const uint8_t *payload)
{
    uint32_t c = crc32((const uint8_t *)&h->len, sizeof h->len, 0);
    c = crc32((const uint8_t *)&h->seq, sizeof h->seq, c);
    return crc32(payload, h->len, c);
}

static size_t slot_offset(size_t slot)
{
    return slot * OB_SLOT_SIZE;
}

static slot_kind_t read_slot(ob_t *ob, size_t slot, header_t *h, uint8_t *payload)
{
    if (ob->st.read(ob->st.ctx, slot_offset(slot), h, sizeof *h) != 0) {
        return SLOT_CORRUPT;
    }
    const uint8_t *raw = (const uint8_t *)h;
    bool blank = true;
    for (size_t i = 0; i < sizeof *h; i++) {
        if (raw[i] != 0xFF) {
            blank = false;
            break;
        }
    }
    if (blank) {
        return SLOT_BLANK;
    }
    if (h->magic != MAGIC || h->len > OB_MAX_PAYLOAD) {
        return SLOT_CORRUPT;
    }
    uint8_t local[OB_MAX_PAYLOAD];
    uint8_t *dst = payload ? payload : local;
    if (h->len > 0 &&
        ob->st.read(ob->st.ctx, slot_offset(slot) + OB_HEADER_SIZE, dst, h->len) != 0) {
        return SLOT_CORRUPT;
    }
    if (record_crc(h, dst) != h->crc) {
        return SLOT_CORRUPT;
    }
    return h->state == STATE_LIVE ? SLOT_LIVE : SLOT_CONSUMED;
}

ob_status_t ob_init(ob_t *ob, const ob_storage_t *storage)
{
    if (ob == NULL || storage == NULL || storage->sector_size % OB_SLOT_SIZE != 0 ||
        storage->sector_count < 2) {
        return OB_ERR_CONFIG;
    }
    memset(ob, 0, sizeof *ob);
    ob->st = *storage;
    ob->slots_per_sector = storage->sector_size / OB_SLOT_SIZE;
    ob->slots_total = ob->slots_per_sector * storage->sector_count;

    size_t newest_slot = 0;
    for (size_t i = 0; i < ob->slots_total; i++) {
        header_t h;
        slot_kind_t kind = read_slot(ob, i, &h, NULL);
        if (kind == SLOT_LIVE) {
            ob->live++;
        }
        if (kind == SLOT_LIVE || kind == SLOT_CONSUMED) {
            if (!ob->has_seq || h.seq > ob->max_seq) {
                ob->max_seq = h.seq;
                ob->has_seq = true;
                newest_slot = i;
            }
        }
    }
    ob->tail = ob->has_seq ? (newest_slot + 1) % ob->slots_total : 0;
    return OB_OK;
}

static ob_status_t erase_sector_counting(ob_t *ob, size_t sector)
{
    for (size_t i = 0; i < ob->slots_per_sector; i++) {
        header_t h;
        if (read_slot(ob, sector * ob->slots_per_sector + i, &h, NULL) == SLOT_LIVE) {
            ob->live--;
            ob->dropped++;
        }
    }
    return ob->st.erase_sector(ob->st.ctx, sector) == 0 ? OB_OK : OB_ERR_IO;
}

ob_status_t ob_push(ob_t *ob, uint32_t seq, const void *data, size_t len)
{
    if (len > OB_MAX_PAYLOAD) {
        return OB_ERR_TOO_BIG;
    }
    header_t cur;
    if (read_slot(ob, ob->tail, &cur, NULL) != SLOT_BLANK) {
        ob_status_t s = erase_sector_counting(ob, ob->tail / ob->slots_per_sector);
        if (s != OB_OK) {
            return s;
        }
    }

    uint8_t slot[OB_SLOT_SIZE];
    memset(slot, 0xFF, sizeof slot);
    header_t h;
    memset(&h, 0xFF, sizeof h);
    h.magic = MAGIC;
    h.len = (uint16_t)len;
    h.seq = seq;
    h.state = STATE_LIVE;
    h.crc = record_crc(&h, data);
    memcpy(slot, &h, sizeof h);
    memcpy(slot + OB_HEADER_SIZE, data, len);
    if (ob->st.write(ob->st.ctx, slot_offset(ob->tail), slot, OB_HEADER_SIZE + len) != 0) {
        return OB_ERR_IO;
    }
    ob->tail = (ob->tail + 1) % ob->slots_total;
    ob->live++;
    if (!ob->has_seq || seq > ob->max_seq) {
        ob->max_seq = seq;
        ob->has_seq = true;
    }
    return OB_OK;
}

ob_status_t ob_peek_oldest(ob_t *ob, ob_record_t *rec)
{
    bool found = false;
    uint32_t best = 0;
    size_t best_slot = 0;
    for (size_t i = 0; i < ob->slots_total; i++) {
        header_t h;
        if (read_slot(ob, i, &h, NULL) == SLOT_LIVE && (!found || h.seq < best)) {
            found = true;
            best = h.seq;
            best_slot = i;
        }
    }
    if (!found) {
        return OB_EMPTY;
    }
    header_t h;
    if (read_slot(ob, best_slot, &h, rec->data) != SLOT_LIVE) {
        return OB_ERR_IO;
    }
    rec->slot = best_slot;
    rec->seq = h.seq;
    rec->len = h.len;
    return OB_OK;
}

ob_status_t ob_pop(ob_t *ob, const ob_record_t *rec)
{
    uint8_t consumed = STATE_CONSUMED;
    if (ob->st.write(ob->st.ctx, slot_offset(rec->slot) + STATE_OFFSET, &consumed, 1) != 0) {
        return OB_ERR_IO;
    }
    ob->live--;
    return OB_OK;
}

size_t ob_count(const ob_t *ob)
{
    return ob->live;
}

bool ob_max_seq(const ob_t *ob, uint32_t *seq)
{
    if (ob->has_seq) {
        *seq = ob->max_seq;
    }
    return ob->has_seq;
}

uint32_t ob_take_dropped(ob_t *ob)
{
    uint32_t d = ob->dropped;
    ob->dropped = 0;
    return d;
}

static ob_status_t flush(ob_t *ob, ob_publish_fn publish, void *ctx, size_t *published, bool *clean)
{
    *clean = true;
    for (;;) {
        ob_record_t rec;
        ob_status_t s = ob_peek_oldest(ob, &rec);
        if (s == OB_EMPTY) {
            return OB_OK;
        }
        if (s != OB_OK) {
            return s;
        }
        if (publish(ctx, rec.data, rec.len) != 0) {
            *clean = false;
            return OB_OK;
        }
        if (published) {
            (*published)++;
        }
        s = ob_pop(ob, &rec);
        if (s != OB_OK) {
            return s;
        }
    }
}

ob_status_t ob_send_or_buffer(ob_t *ob, uint32_t seq, const void *data, size_t len,
                              ob_publish_fn publish, void *ctx, size_t *published)
{
    if (published) {
        *published = 0;
    }
    bool clean;
    ob_status_t s = flush(ob, publish, ctx, published, &clean);
    if (s != OB_OK) {
        return s;
    }
    if (clean && publish(ctx, data, len) == 0) {
        if (published) {
            (*published)++;
        }
        return OB_OK;
    }
    return ob_push(ob, seq, data, len);
}
