#include "telemetry_payload.h"

#include <math.h>
#include <stdio.h>

typedef struct {
    char *buf;
    size_t cap;
    size_t len;
    bool overflow;
} writer_t;

static void put(writer_t *w, const char *fmt, ...) __attribute__((format(printf, 2, 3)));

#include <stdarg.h>

static void put(writer_t *w, const char *fmt, ...)
{
    if (w->overflow) {
        return;
    }
    va_list ap;
    va_start(ap, fmt);
    int n = vsnprintf(w->buf + w->len, w->cap - w->len, fmt, ap);
    va_end(ap);
    if (n < 0 || (size_t)n >= w->cap - w->len) {
        w->overflow = true;
        return;
    }
    w->len += (size_t)n;
}

/* Separador de itens de r[]: a primeira leitura não leva vírgula. */
static const char *sep(bool *first)
{
    const char *s = *first ? "" : ",";
    *first = false;
    return s;
}

int telemetry_payload_encode(const telemetry_batch_t *b, char *buf, size_t cap)
{
    if (b == NULL || b->fw_version == NULL || buf == NULL || cap == 0) {
        return -1;
    }
    writer_t w = {.buf = buf, .cap = cap};
    const readings_t *r = &b->readings;
    bool first = true;

    put(&w, "{\"fw\":\"%s\",\"seq\":%lu,\"ts\":%lld,\"dropped\":%lu,\"r\":[", b->fw_version,
        (unsigned long)b->seq, (long long)b->ts, (unsigned long)b->dropped);
    if (r->has_soil) {
        put(&w, "%s{\"id\":\"soil-1\",\"k\":\"soil_adc\",\"v\":%u}", sep(&first),
            (unsigned)r->soil_adc);
    }
    if (r->has_air && !isnan(r->air_temp_c) && !isnan(r->air_rh_pct)) {
        put(&w, "%s{\"id\":\"air-1\",\"k\":\"air_temp_c\",\"v\":%.2f}", sep(&first),
            (double)r->air_temp_c);
        put(&w, "%s{\"id\":\"air-1\",\"k\":\"air_rh_pct\",\"v\":%.2f}", sep(&first),
            (double)r->air_rh_pct);
    }
    if (r->has_lux && !isnan(r->lux)) {
        put(&w, "%s{\"id\":\"light-1\",\"k\":\"light_lux\",\"v\":%.1f}", sep(&first),
            (double)r->lux);
    }
    if (r->has_water) {
        put(&w, "%s{\"id\":\"water-1\",\"k\":\"water_level_ok\",\"v\":%d}", sep(&first),
            r->water_ok ? 1 : 0);
    }
    put(&w, "]}");
    return w.overflow ? -1 : (int)w.len;
}
