<script setup lang="ts">
import { mdiValve } from '@mdi/js'
import { computed, ref } from 'vue'
import { useRequestIrrigation } from '@/api/queries'
import { ApiError } from '@/api/client'
import type { ActuatorState, BedOverview } from '@/api/schemas'
import { valveText, valveView } from '@/components/signature/plan-geometry'
import { canOperate } from '@/features/auth/session'
import { ago, celsius, duration, hhmmss, pct, points } from '@/lib/format'

const props = defineProps<{ gardenId: string; bed: BedOverview }>()

const seconds = ref(180)
const options = [60, 180, 300, 600].map((s) => ({ title: duration(s), value: s }))
const irrigate = useRequestIrrigation(() => props.gardenId)

const view = computed(() => valveView(props.bed))
const violations = computed(() => {
  const e = irrigate.error.value
  return e instanceof ApiError ? (e.problem?.violations ?? []) : []
})
const failure = computed(() => {
  const e = irrigate.error.value
  if (!e) return ''
  return e instanceof ApiError && e.status === 409 && !violations.value.length ? 'Já existe uma rega em andamento neste nó.' : e.message
})

function submit() {
  if (!props.bed.actuatorNodeId) return
  irrigate.mutate({ nodeId: props.bed.actuatorNodeId, durationSeconds: seconds.value })
}
const stateLine = (s?: ActuatorState) => (s ? `${s.valve === 'open' ? 'aberta' : 'fechada'} às ${hhmmss(s.at)}` : 'sem registro')
</script>

<template>
  <v-card class="pa-4" :aria-label="`Canteiro ${bed.name}`">
    <div class="d-flex justify-space-between align-start mb-2">
      <div>
        <h2 class="text-h6">{{ bed.name }}</h2>
        <p class="text-body-2 text-medium-emphasis">{{ bed.crop || 'sem cultura' }}</p>
      </div>
      <v-btn variant="text" :to="{ name: 'bed', params: { gardenId, bedId: bed.id } }">Detalhe</v-btn>
    </div>

    <dl class="facts">
      <dt>Umidade do solo</dt>
      <dd class="cota">{{ bed.moisturePct !== undefined ? pct(bed.moisturePct) : 'sem leitura' }}</dd>
      <dt>Faixa ideal</dt>
      <dd class="cota">{{ bed.idealMinPct.toFixed(0) }} a {{ bed.idealMaxPct.toFixed(0) }} %</dd>
      <dt>Déficit até o topo da faixa</dt>
      <dd class="cota">{{ bed.deficitPct !== undefined ? points(bed.deficitPct) : 'sem leitura' }}</dd>
      <dt>Ar</dt>
      <dd class="cota">{{ bed.airTempC !== undefined ? celsius(bed.airTempC) : 'sem leitura' }}</dd>
      <dt>Última leitura</dt>
      <dd>{{ bed.readAt ? ago(bed.readAt) : 'há mais de 30 minutos' }}</dd>
    </dl>

    <v-divider class="my-3" />

    <div class="d-flex align-center ga-2 mb-2">
      <v-icon :icon="mdiValve" size="20" />
      <p class="font-weight-medium">Válvula</p>
      <v-chip :color="view === 'open' ? 'accent' : view.startsWith('awaiting') ? 'warning' : undefined">{{ valveText(view) }}</v-chip>
    </div>
    <dl v-if="bed.valve" class="facts">
      <dt>Desejado</dt>
      <dd class="cota">{{ stateLine(bed.valve.desired) }}</dd>
      <dt>Reportado pelo nó</dt>
      <dd class="cota">{{ stateLine(bed.valve.reported) }}</dd>
    </dl>
    <p v-else class="text-body-2 text-medium-emphasis">Nenhum nó atuador ligado a este canteiro.</p>

    <form v-if="bed.actuatorNodeId" class="mt-4" @submit.prevent="submit">
      <template v-if="canOperate">
        <div class="d-flex ga-2 align-start">
          <v-select v-model="seconds" :items="options" label="Duração" density="comfortable" hide-details style="max-width: 150px" />
          <v-btn type="submit" color="primary" :loading="irrigate.isPending.value" :disabled="bed.irrigating || !!bed.openCommand">Regar agora</v-btn>
        </div>
        <p v-if="bed.openCommand" class="text-body-2 text-medium-emphasis mt-2">Comando em andamento: aguardando o nó confirmar.</p>
      </template>
      <p v-else class="text-body-2 text-medium-emphasis">Seu papel é de leitura; só dono e operador pedem rega.</p>
      <v-alert v-if="irrigate.isSuccess.value" type="success" class="mt-3">Comando enviado. A válvula só muda quando o nó confirmar.</v-alert>
      <v-alert v-if="irrigate.isError.value" type="warning" class="mt-3" title="Rega não enviada">
        <p v-if="failure">{{ failure }}</p>
        <ul v-if="violations.length" class="pl-4">
          <li v-for="v in violations" :key="v.rule">{{ v.reason }}</li>
        </ul>
      </v-alert>
    </form>
  </v-card>
</template>

<style scoped>
.facts {
  display: grid;
  grid-template-columns: 1fr auto;
  gap: 4px 12px;
  font-size: 0.875rem;
  dt {
    color: rgb(var(--v-theme-on-surface));
    opacity: 0.72;
  }
  dd {
    text-align: right;
  }
}
</style>
