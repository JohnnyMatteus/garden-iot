#include "median.h"

uint16_t median_u16(uint16_t *s, size_t n)
{
    for (size_t i = 1; i < n; i++) {
        uint16_t v = s[i];
        size_t j = i;
        while (j > 0 && s[j - 1] > v) {
            s[j] = s[j - 1];
            j--;
        }
        s[j] = v;
    }
    return s[n / 2];
}
