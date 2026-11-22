#pragma once

#include <stdbool.h>

/* Conecta ao Wi-Fi; false se estourar o tempo. */
bool net_connect(int timeout_ms);
/* Sincroniza o relógio por SNTP; false se estourar o tempo (relógio segue inválido). */
bool net_sync_time(int timeout_ms);
/* Epoch UTC em segundos, ou 0 se o relógio ainda não é confiável. */
long long net_now_utc(void);
