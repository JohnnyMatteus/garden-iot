#pragma once

#include "esp_err.h"
#include "offline_buffer.h"

/* Liga a fila offline à partição "offbuf". */
esp_err_t flash_storage_open(ob_storage_t *out);
