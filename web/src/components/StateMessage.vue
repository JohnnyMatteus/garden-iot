<script setup lang="ts">
import { mdiAlertOctagonOutline, mdiSprout, mdiTimerSand } from '@mdi/js'
import { computed } from 'vue'

const props = defineProps<{ kind: 'loading' | 'empty' | 'error'; title: string; detail?: string }>()
const icon = computed(() => ({ loading: mdiTimerSand, empty: mdiSprout, error: mdiAlertOctagonOutline })[props.kind])
</script>

<template>
  <div class="state" :role="kind === 'error' ? 'alert' : 'status'">
    <v-icon :icon="icon" :color="kind === 'error' ? 'error' : undefined" size="28" />
    <div>
      <p class="font-weight-medium">{{ title }}</p>
      <p v-if="detail" class="text-body-2 text-medium-emphasis">{{ detail }}</p>
    </div>
  </div>
</template>

<style scoped>
.state {
  display: flex;
  gap: 12px;
  align-items: flex-start;
  padding: 24px 0;
}
</style>
