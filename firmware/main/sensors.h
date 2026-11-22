#pragma once

#include "telemetry_payload.h"

/* Lê todos os sensores; o que falhar fica com has_* = false. */
void sensors_init(void);
readings_t sensors_read(void);
