#pragma once

/* Deep sleep até o próximo ciclo. Não retorna. */
void power_deep_sleep(unsigned seconds) __attribute__((noreturn));
