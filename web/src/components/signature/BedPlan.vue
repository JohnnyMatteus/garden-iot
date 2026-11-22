<script setup lang="ts">
import { computed } from 'vue'
import type { BedOverview } from '@/api/schemas'
import { celsius, pct, points } from '@/lib/format'
import { describeBed, levelFraction, type Rect, valveView } from './plan-geometry'

const props = defineProps<{ bed: BedOverview; rect: Rect; selected?: boolean }>()
defineEmits<{ select: [id: string] }>()

const PAD = { top: 38, bottom: 46, left: 34, right: 14 }
const inner = computed(() => ({
  x: props.rect.x + PAD.left,
  y: props.rect.y + PAD.top,
  w: props.rect.w - PAD.left - PAD.right,
  h: props.rect.h - PAD.top - PAD.bottom,
}))

const hasReading = computed(() => props.bed.moisturePct !== undefined)
const yOf = (value: number) => inner.value.y + inner.value.h * (1 - levelFraction(value))
const levelY = computed(() => yOf(props.bed.moisturePct ?? 0))
const bandTopY = computed(() => yOf(props.bed.idealMaxPct))
const bandBottomY = computed(() => yOf(props.bed.idealMinPct))
// Hachura: o que falta do nível atual até o topo da faixa ideal.
const dryHeight = computed(() => (hasReading.value ? Math.max(0, levelY.value - bandTopY.value) : 0))

const valve = computed(() => valveView(props.bed))
const valveOpen = computed(() => valve.value === 'open' || valve.value === 'awaiting-close')
const valveX = computed(() => props.rect.x + props.rect.w - PAD.right - 12)
const valveY = computed(() => props.rect.y + props.rect.h - 20)
const label = computed(() => describeBed(props.bed))
const uid = computed(() => `bed-${props.bed.id}`)
</script>

<template>
  <g
    class="bed" :class="{ selected }" role="button" tabindex="0" :aria-label="label" :aria-pressed="selected ? 'true' : 'false'"
    @click="$emit('select', bed.id)" @keydown.enter.prevent="$emit('select', bed.id)" @keydown.space.prevent="$emit('select', bed.id)"
  >
    <title>{{ label }}</title>
    <clipPath :id="`${uid}-clip`"><rect :x="inner.x" :y="inner.y" :width="inner.w" :height="inner.h" /></clipPath>
    <rect class="hit" :x="rect.x" :y="rect.y" :width="rect.w" :height="rect.h" />

    <text class="legend" :x="rect.x + 4" :y="rect.y + 16">{{ bed.name }}</text>
    <text class="legend dim" :x="rect.x + 4" :y="rect.y + 30">{{ bed.crop || 'sem cultura' }} · {{ bed.irrigation === 'drip' ? 'gotejo' : 'aspersão' }}</text>

    <g :clip-path="`url(#${uid}-clip)`">
      <rect class="soil" :x="inner.x" :y="inner.y" :width="inner.w" :height="inner.h" />
      <template v-if="hasReading">
        <rect class="water" :x="inner.x" :y="levelY" :width="inner.w" :height="inner.y + inner.h - levelY" />
        <rect v-if="dryHeight > 0" class="dry" :x="inner.x" :y="bandTopY" :width="inner.w" :height="dryHeight" />
      </template>
    </g>
    <rect class="outline" :class="{ unknown: !hasReading }" :x="inner.x" :y="inner.y" :width="inner.w" :height="inner.h" />
    <line v-if="hasReading" class="level-line" :x1="inner.x" :x2="inner.x + inner.w" :y1="levelY" :y2="levelY" />

    <!-- cotas da faixa ideal: duas marcas na lateral, com o valor em mono -->
    <g class="band">
      <line :x1="inner.x - 8" :x2="inner.x" :y1="bandTopY" :y2="bandTopY" />
      <line :x1="inner.x - 8" :x2="inner.x" :y1="bandBottomY" :y2="bandBottomY" />
      <line :x1="inner.x - 8" :x2="inner.x - 8" :y1="bandTopY" :y2="bandBottomY" />
      <text :x="inner.x - 11" :y="bandTopY + 3" text-anchor="end">{{ bed.idealMaxPct.toFixed(0) }}</text>
      <text :x="inner.x - 11" :y="bandBottomY + 3" text-anchor="end">{{ bed.idealMinPct.toFixed(0) }}</text>
    </g>

    <g class="readout">
      <text v-if="hasReading" class="reading" :x="rect.x + 4" :y="rect.y + rect.h - 26">{{ pct(bed.moisturePct!) }}</text>
      <text v-else class="reading dim" :x="rect.x + 4" :y="rect.y + rect.h - 26">sem leitura</text>
      <text v-if="bed.deficitPct" class="legend dim" :x="rect.x + 4" :y="rect.y + rect.h - 10">déficit {{ points(bed.deficitPct) }}</text>
      <text v-else-if="bed.airTempC !== undefined" class="legend dim" :x="rect.x + 4" :y="rect.y + rect.h - 10">ar {{ celsius(bed.airTempC) }}</text>
    </g>

    <!-- tipo de irrigação desenhado: gotejo = linha de pontos; aspersão = arcos -->
    <g class="irrigation-glyph" aria-hidden="true">
      <template v-if="bed.irrigation === 'drip'">
        <circle v-for="i in 5" :key="i" :cx="inner.x + (i * inner.w) / 6" :cy="inner.y - 6" r="1.8" />
      </template>
      <template v-else>
        <path :d="`M${inner.x + inner.w - 26} ${inner.y - 4}a8 8 0 0 1 16 0M${inner.x + inner.w - 31} ${inner.y - 4}a13 13 0 0 1 26 0`" fill="none" />
      </template>
    </g>

    <!-- válvula de gaveta: dois triângulos opostos; vazia fechada, preenchida aberta -->
    <g class="valve" :class="[`v-${valve}`, { open: valveOpen }]" :transform="`translate(${valveX - 20} ${valveY})`">
      <path d="M0 -7V7L10 0ZM20 -7V7L10 0Z" />
      <path v-if="valve === 'awaiting-open' || valve === 'awaiting-close'" class="awaiting" d="M-4 -11H24V11H-4Z" />
    </g>
    <path v-if="valveOpen" class="flow" :d="`M${inner.x} ${rect.y + rect.h - 20}H${valveX - 24}`" />
  </g>
</template>

<style scoped lang="scss">
.bed {
  cursor: pointer;
  outline: none;
  &:focus-visible .hit,
  &.selected .hit {
    stroke: rgb(var(--v-theme-accent));
    stroke-width: 2;
  }
  &:hover .hit {
    fill: rgba(var(--v-theme-on-surface), 0.04);
  }
}
.hit {
  fill: transparent;
}
text {
  fill: rgb(var(--v-theme-on-surface));
  font-family: 'Spline Sans Mono Variable', ui-monospace, monospace;
  font-variant-numeric: tabular-nums;
}
.legend {
  font-size: 11px;
  letter-spacing: 0.03em;
  font-weight: 600;
}
.dim {
  opacity: 0.72;
  font-weight: 500;
}
.reading {
  font-size: 15px;
  font-weight: 600;
}
.soil {
  fill: rgb(var(--v-theme-surface-variant));
  opacity: 0.55;
}
.water {
  fill: rgb(var(--v-theme-water-fill));
  transition: y 400ms ease, height 400ms ease;
}
.dry {
  fill: url(#hatch-dry);
}
.outline {
  fill: none;
  stroke: rgb(var(--v-theme-on-surface));
  stroke-width: 1.5;
  &.unknown {
    stroke-dasharray: 5 4;
    opacity: 0.6;
  }
}
.level-line {
  stroke: rgb(var(--v-theme-water-edge));
  stroke-width: 2;
}
.band line {
  stroke: rgb(var(--v-theme-on-surface));
  stroke-width: 1;
}
.band text {
  font-size: 10px;
}
.irrigation-glyph {
  fill: rgb(var(--v-theme-on-surface));
  stroke: rgb(var(--v-theme-on-surface));
  opacity: 0.7;
  path {
    stroke-width: 1.2;
  }
}
.valve path {
  fill: rgb(var(--v-theme-surface));
  stroke: rgb(var(--v-theme-on-surface));
  stroke-width: 1.5;
  stroke-linejoin: round;
}
.valve.open path:first-child {
  fill: rgb(var(--v-theme-accent));
  stroke: rgb(var(--v-theme-accent));
}
.valve .awaiting {
  fill: none;
  stroke: rgb(var(--v-theme-warning));
  stroke-dasharray: 3 3;
}
.valve.v-unknown {
  opacity: 0.4;
}
.flow {
  fill: none;
  stroke: rgb(var(--v-theme-accent));
  stroke-width: 2.5;
  stroke-dasharray: 6 5;
  animation: flow 1.2s linear infinite;
}
@keyframes flow {
  to {
    stroke-dashoffset: -11;
  }
}
@media (prefers-reduced-motion: reduce) {
  .flow {
    animation: none;
  }
  .water {
    transition: none;
  }
}
</style>
