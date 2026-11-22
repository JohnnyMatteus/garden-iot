#pragma once

#include <stddef.h>
#include <stdint.h>

/* Mediana de n amostras (n > 0, n <= 32). Reordena o vetor. Filtra picos do ADC. */
uint16_t median_u16(uint16_t *samples, size_t n);
