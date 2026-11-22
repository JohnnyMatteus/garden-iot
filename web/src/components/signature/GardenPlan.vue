<script setup lang="ts">
import { computed } from 'vue'
import type { BedOverview } from '@/api/schemas'
import BedPlan from './BedPlan.vue'
import { layoutBeds } from './plan-geometry'

const props = defineProps<{ beds: BedOverview[]; selectedId?: string | null; name: string }>()
defineEmits<{ select: [id: string] }>()

const layout = computed(() => layoutBeds(props.beds.length))
</script>

<template>
  <svg
    class="plan" :viewBox="`0 0 ${layout.width} ${layout.height}`" role="group" :aria-label="`Planta da horta ${name}, ${beds.length} canteiros`"
    preserveAspectRatio="xMidYMin meet"
  >
    <defs>
      <pattern id="hatch-dry" width="6" height="6" patternUnits="userSpaceOnUse" patternTransform="rotate(45)">
        <line x1="0" y1="0" x2="0" y2="6" class="hatch-line" />
      </pattern>
    </defs>
    <BedPlan v-for="(bed, i) in beds" :key="bed.id" :bed="bed" :rect="layout.rects[i]!" :selected="bed.id === selectedId" @select="$emit('select', $event)" />
    <!-- rosa dos ventos: ancora a leitura como vista de cima -->
    <g class="compass" aria-hidden="true" :transform="`translate(${layout.width - 22} 22)`">
      <path d="M0 -10L4 4L0 1L-4 4Z" />
      <text y="-13" text-anchor="middle">N</text>
    </g>
  </svg>
</template>

<style scoped>
.plan {
  width: 100%;
  height: auto;
  display: block;
}
.hatch-line {
  stroke: rgb(var(--v-theme-dry-hatch));
  stroke-width: 1.4;
}
.compass path {
  fill: rgb(var(--v-theme-on-surface));
  opacity: 0.6;
}
.compass text {
  font: 600 9px 'Spline Sans Mono Variable', ui-monospace, monospace;
  fill: rgb(var(--v-theme-on-surface));
  opacity: 0.6;
}
</style>
