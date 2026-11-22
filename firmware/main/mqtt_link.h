#pragma once

#include <stdbool.h>
#include <stddef.h>

/* Conecta ao broker (MQTT 5, mTLS) e espera a sessão abrir. */
bool mqtt_link_connect(int timeout_ms);
/* Publica em QoS 1 e espera o PUBACK. Devolve 0 em sucesso (assinatura de ob_publish_fn). */
int mqtt_link_publish_batch(void *ctx, const void *payload, size_t len);
void mqtt_link_close(void);
