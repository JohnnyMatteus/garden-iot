<script setup lang="ts">
import { mdiAlertOutline, mdiCheckCircleOutline } from '@mdi/js'
import { useNodeState } from '@/api/queries'
import type { ActuatorState } from '@/api/schemas'
import { hhmmss } from '@/lib/format'

const props = defineProps<{ gardenId: string; nodeId: string; bedName: string }>()
const { data: state, isPending, error } = useNodeState(() => props.gardenId, () => props.nodeId)

const valve = (s?: ActuatorState) => (s ? (s.valve === 'open' ? 'aberta' : 'fechada') : 'sem registro')
</script>

<template>
  <v-card class="pa-4">
    <p class="legenda text-medium-emphasis">{{ bedName }} · <span class="cota">{{ nodeId }}</span></p>
    <p v-if="isPending" class="text-body-2 mt-2">Consultando o nó...</p>
    <p v-else-if="error" class="text-body-2 text-error mt-2" role="alert">{{ error.message }}</p>
    <template v-else-if="state">
      <div class="pair mt-3">
        <div>
          <p class="legenda text-medium-emphasis">Desejado</p>
          <p class="cota text-h6">{{ valve(state.desired) }}</p>
          <p v-if="state.desired" class="cota text-body-2 text-medium-emphasis">{{ hhmmss(state.desired.at) }}</p>
        </div>
        <div>
          <p class="legenda text-medium-emphasis">Reportado</p>
          <p class="cota text-h6">{{ valve(state.reported) }}</p>
          <p v-if="state.reported" class="cota text-body-2 text-medium-emphasis">
            {{ hhmmss(state.reported.at) }}<template v-if="state.reported.flowLpm !== undefined"> · {{ state.reported.flowLpm.toFixed(1) }} L/min</template>
          </p>
        </div>
      </div>
      <p class="d-flex align-center ga-1 mt-3 text-body-2" :class="state.inSync ? 'text-success' : 'text-warning'">
        <v-icon :icon="state.inSync ? mdiCheckCircleOutline : mdiAlertOutline" size="18" />
        {{ state.inSync ? 'O nó confirmou a posição pedida' : 'Fora de sincronia: o nó ainda não confirmou' }}
      </p>
    </template>
  </v-card>
</template>

<style scoped>
.pair {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 12px;
  border-top: 1px solid rgba(var(--v-border-color), var(--v-border-opacity));
  padding-top: 12px;
}
</style>
