<script setup lang="ts">
import { computed, ref } from 'vue'
import type { AggregatePoint } from '@/api/schemas'
import { dayMonthTime } from '@/lib/format'
import { format } from 'date-fns'
import { buildPaths, chartDomain, nearestPoint, scaleX, scaleY, type ChartBox } from './chart-geometry'

const props = defineProps<{
  title: string
  points: AggregatePoint[]
  from: Date
  to: Date
  bucketMs: number
  band?: { min: number; max: number } | null
  /** Regas aplicadas: barras na base, largura proporcional à duração. */
  irrigations?: { at: Date; seconds: number }[]
  formatValue: (v: number) => string
}>()

const box: ChartBox = { width: 720, height: 240, left: 52, right: 12, top: 12, bottom: 30 }
const domain = computed(() => chartDomain(props.points, props.band ?? null, props.from, props.to))
const paths = computed(() => buildPaths(props.points, box, domain.value, props.bucketMs))

const yTicks = computed(() => {
  const { y0, y1 } = domain.value
  return Array.from({ length: 4 }, (_, i) => y0 + ((y1 - y0) * i) / 3)
})
const xTicks = computed(() => {
  const { t0, t1 } = domain.value
  return Array.from({ length: 5 }, (_, i) => new Date(t0 + ((t1 - t0) * i) / 4))
})
const spanDays = computed(() => (props.to.getTime() - props.from.getTime()) / 86_400_000)
const tickLabel = (d: Date) => (spanDays.value > 1.5 ? format(d, 'dd/MM') : format(d, 'HH:mm'))

const bandRect = computed(() => {
  if (!props.band) return null
  const top = scaleY(box, domain.value, props.band.max)
  return { y: top, h: scaleY(box, domain.value, props.band.min) - top }
})

const bars = computed(() =>
  (props.irrigations ?? []).map((r) => {
    const x = scaleX(box, domain.value, r.at.getTime())
    const w = Math.max(3, (r.seconds * 1000 * (box.width - box.left - box.right)) / (domain.value.t1 - domain.value.t0))
    return { x, w, key: r.at.getTime() }
  }),
)

const hover = ref<AggregatePoint | null>(null)
function onMove(e: PointerEvent) {
  const svg = e.currentTarget as SVGSVGElement
  const r = svg.getBoundingClientRect()
  const px = ((e.clientX - r.left) / r.width) * box.width
  const t = domain.value.t0 + ((px - box.left) / (box.width - box.left - box.right)) * (domain.value.t1 - domain.value.t0)
  hover.value = nearestPoint(props.points, t)
}
const hoverX = computed(() => (hover.value ? scaleX(box, domain.value, hover.value.at.getTime()) : 0))

const summary = computed(() => {
  if (!props.points.length) return `${props.title}: sem leituras no período`
  const last = props.points.at(-1)!
  const band = props.band ? `, faixa ideal de ${props.formatValue(props.band.min)} a ${props.formatValue(props.band.max)}` : ''
  return `${props.title}: última média ${props.formatValue(last.avg)}${band}`
})
</script>

<template>
  <figure class="chart">
    <figcaption class="d-flex justify-space-between align-baseline flex-wrap ga-2">
      <span class="font-weight-medium">{{ title }}</span>
      <span class="cota text-body-2 text-medium-emphasis" aria-live="polite">
        <template v-if="hover">{{ dayMonthTime(hover.at) }} · média {{ formatValue(hover.avg) }} · {{ formatValue(hover.min) }} a {{ formatValue(hover.max) }}</template>
        <template v-else-if="band">faixa ideal {{ formatValue(band.min) }} a {{ formatValue(band.max) }}</template>
      </span>
    </figcaption>
    <svg
      :viewBox="`0 0 ${box.width} ${box.height}`" role="img" :aria-label="summary" @pointermove="onMove" @pointerleave="hover = null"
    >
      <rect v-if="bandRect" class="band" :x="box.left" :y="bandRect.y" :width="box.width - box.left - box.right" :height="bandRect.h" />
      <line v-if="bandRect" class="band-edge" :x1="box.left" :x2="box.width - box.right" :y1="bandRect.y" :y2="bandRect.y" />
      <line v-if="bandRect" class="band-edge" :x1="box.left" :x2="box.width - box.right" :y1="bandRect.y + bandRect.h" :y2="bandRect.y + bandRect.h" />

      <g class="grid">
        <g v-for="v in yTicks" :key="v">
          <line :x1="box.left" :x2="box.width - box.right" :y1="scaleY(box, domain, v)" :y2="scaleY(box, domain, v)" />
          <text :x="box.left - 6" :y="scaleY(box, domain, v) + 3" text-anchor="end">{{ formatValue(v) }}</text>
        </g>
        <text v-for="t in xTicks" :key="t.getTime()" :x="scaleX(box, domain, t.getTime())" :y="box.height - 10" text-anchor="middle">{{ tickLabel(t) }}</text>
      </g>

      <path class="shade" :d="paths.shade" />
      <path class="line" :d="paths.line" />
      <circle v-for="p in points.length === 1 ? points : []" :key="p.at.getTime()" class="dot" :cx="scaleX(box, domain, p.at.getTime())" :cy="scaleY(box, domain, p.avg)" r="3" />

      <rect v-for="b in bars" :key="b.key" class="irrigation" :x="b.x" :y="box.height - box.bottom - 8" :width="b.w" height="8" />

      <g v-if="hover">
        <line class="cursor" :x1="hoverX" :x2="hoverX" :y1="box.top" :y2="box.height - box.bottom" />
        <circle class="dot" :cx="hoverX" :cy="scaleY(box, domain, hover.avg)" r="3.5" />
      </g>
      <text v-if="!points.length" class="empty" :x="box.width / 2" :y="box.height / 2" text-anchor="middle">sem leituras neste período</text>
    </svg>
  </figure>
</template>

<style scoped>
.chart svg {
  width: 100%;
  height: auto;
  display: block;
  touch-action: pan-y;
}
.band {
  fill: rgb(var(--v-theme-water-fill));
  opacity: 0.45;
}
.band-edge {
  stroke: rgb(var(--v-theme-water-edge));
  stroke-width: 1;
  stroke-dasharray: 5 4;
}
.grid line {
  stroke: rgb(var(--v-theme-on-surface));
  opacity: 0.12;
}
.grid text,
.empty {
  fill: rgb(var(--v-theme-on-surface));
  opacity: 0.75;
  font: 500 10.5px 'Spline Sans Mono Variable', ui-monospace, monospace;
}
.shade {
  fill: rgb(var(--v-theme-primary));
  opacity: 0.16;
}
.line {
  fill: none;
  stroke: rgb(var(--v-theme-primary));
  stroke-width: 1.8;
  stroke-linejoin: round;
}
.dot {
  fill: rgb(var(--v-theme-primary));
}
.irrigation {
  fill: rgb(var(--v-theme-accent));
}
.cursor {
  stroke: rgb(var(--v-theme-on-surface));
  opacity: 0.4;
}
</style>
