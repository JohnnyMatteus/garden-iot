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

int telemetry_payload_encode(const telemetry_batch_t *b, char *buf, size_t cap)
{
    if (b == NULL || b->fw_version == NULL || buf == NULL || cap == 0) {
        return -1;
    }
    writer_t w = {.buf = buf, .cap = cap};
    const readings_t *r = &b->readings;

    put(&w, "{\"fw_version\":\"%s\",\"seq\":%lu,\"ts\":%lld,\"dropped\":%lu", b->fw_version,
        (unsigned long)b->seq, (long long)b->ts, (unsigned long)b->dropped);
    if (r->has_soil) {
        put(&w, ",\"soil_adc\":%u", (unsigned)r->soil_adc);
    }
    if (r->has_air && !isnan(r->air_temp_c) && !isnan(r->air_rh_pct)) {
        put(&w, ",\"air_temp_c\":%.2f,\"air_rh_pct\":%.2f", (double)r->air_temp_c,
            (double)r->air_rh_pct);
    }
    if (r->has_lux && !isnan(r->lux)) {
        put(&w, ",\"lux\":%.1f", (double)r->lux);
    }
    if (r->has_water) {
        put(&w, ",\"water_level_ok\":%s", r->water_ok ? "true" : "false");
    }
    put(&w, "}");
    return w.overflow ? -1 : (int)w.len;
}
