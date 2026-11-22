<script setup lang="ts">
import { mdiArrowLeft } from '@mdi/js'
import { useNow } from '@vueuse/core'
import { computed, ref } from 'vue'
import { useAggregate, useCommands, useDecisions, useOverview } from '@/api/queries'
import RangeChart from '@/components/signature/RangeChart.vue'
import StateMessage from '@/components/StateMessage.vue'
import { celsius, dayMonth, dayMonthTime, duration, pct } from '@/lib/format'
import { dailyBalance } from './balance'

const props = defineProps<{ gardenId: string; bedId: string }>()

const RANGES = {
  '6h': { label: '6 horas', ms: 6 * 3_600_000, bucket: '5m', bucketMs: 300_000 },
  '24h': { label: '24 horas', ms: 24 * 3_600_000, bucket: '15m', bucketMs: 900_000 },
  '7d': { label: '7 dias', ms: 7 * 86_400_000, bucket: '1h', bucketMs: 3_600_000 },
} as const
const range = ref<keyof typeof RANGES>('24h')

// Arredondado a 5 min para a chave da consulta não mudar a cada renderização.
const now = useNow({ interval: 60_000 })
const to = computed(() => new Date(Math.ceil(now.value.getTime() / 300_000) * 300_000))
const from = computed(() => new Date(to.value.getTime() - RANGES[range.value].ms))

const { data: overview, isPending, error } = useOverview(() => props.gardenId)
const bed = computed(() => overview.value?.beds.find((b) => b.id === props.bedId))

const moisture = useAggregate(() => ({
  gardenId: props.gardenId, nodeId: bed.value?.sensorNodeId, kind: 'soil_moisture_vwc_pct',
  bucket: RANGES[range.value].bucket, from: from.value, to: to.value,
}))
const temperature = useAggregate(() => ({
  gardenId: props.gardenId, nodeId: bed.value?.sensorNodeId, kind: 'air_temp_c',
  bucket: RANGES[range.value].bucket, from: from.value, to: to.value,
}))
const weekFrom = computed(() => new Date(to.value.getTime() - 7 * 86_400_000))
const daily = useAggregate(() => ({
  gardenId: props.gardenId, nodeId: bed.value?.sensorNodeId, kind: 'soil_moisture_vwc_pct', bucket: '24h', from: weekFrom.value, to: to.value,
}))
const commands = useCommands(() => props.gardenId, () => bed.value?.actuatorNodeId)
const decisions = useDecisions(() => props.gardenId, () => props.bedId)

const irrigations = computed(() =>
  (commands.data.value ?? []).filter((c) => c.status === 'confirmed' && c.createdAt >= from.value).map((c) => ({ at: c.createdAt, seconds: c.durationSeconds })),
)
const balance = computed(() => dailyBalance(daily.data.value ?? [], (commands.data.value ?? []).filter((c) => c.createdAt >= weekFrom.value)))
const band = computed(() => (bed.value ? { min: bed.value.idealMinPct, max: bed.value.idealMaxPct } : null))

const OUTCOME: Record<string, string> = {
  pending: 'pendente', sent: 'enviado ao nó', confirmed: 'confirmado pelo nó', rejected: 'recusado pelo nó', expired: 'expirou sem resposta',
  failed: 'falhou ao publicar', info: 'informativo', warning: 'atenção', critical: 'crítico',
}
</script>

<template>
  <div class="page">
    <v-btn variant="text" :prepend-icon="mdiArrowLeft" :to="{ name: 'map', params: { gardenId } }" class="mb-2">Planta</v-btn>
    <StateMessage v-if="isPending" kind="loading" title="Carregando o canteiro" />
    <StateMessage v-else-if="error" kind="error" title="Não foi possível carregar o canteiro" :detail="error.message" />
    <StateMessage v-else-if="!bed" kind="empty" title="Canteiro não encontrado nesta horta" />
    <template v-else>
      <header class="d-flex flex-wrap justify-space-between align-end ga-3 mb-4">
        <div>
          <p class="legenda text-medium-emphasis">Canteiro · {{ bed.crop || 'sem cultura' }}</p>
          <h1 class="text-h5">{{ bed.name }}</h1>
        </div>
        <v-btn-toggle v-model="range" mandatory density="comfortable" divided aria-label="Período">
          <v-btn v-for="(r, k) in RANGES" :key="k" :value="k" variant="outlined" size="small">{{ r.label }}</v-btn>
        </v-btn-toggle>
      </header>

      <div class="grid">
        <v-card class="pa-4">
          <StateMessage v-if="!bed.sensorNodeId" kind="empty" title="Sem sensor neste canteiro" detail="Ligue um nó sensor ao canteiro para ver a série de umidade." />
          <StateMessage v-else-if="moisture.error.value" kind="error" title="Série de umidade indisponível" :detail="moisture.error.value.message" />
          <RangeChart
            v-else title="Umidade do solo" :points="moisture.data.value ?? []" :from="from" :to="to" :bucket-ms="RANGES[range].bucketMs"
            :band="band" :irrigations="irrigations" :format-value="pct"
          />
          <p class="legenda text-medium-emphasis mt-2"><span class="sw-bar" />barras na base: regas confirmadas pelo nó</p>
        </v-card>

        <v-card class="pa-4">
          <StateMessage v-if="!bed.sensorNodeId" kind="empty" title="Sem sensor neste canteiro" />
          <StateMessage v-else-if="temperature.error.value" kind="error" title="Série de temperatura indisponível" :detail="temperature.error.value.message" />
          <RangeChart v-else title="Temperatura do ar" :points="temperature.data.value ?? []" :from="from" :to="to" :bucket-ms="RANGES[range].bucketMs" :format-value="celsius" />
        </v-card>

        <v-card class="pa-4">
          <h2 class="text-h6 mb-1">Balanço observado, 7 dias</h2>
          <p class="text-body-2 text-medium-emphasis mb-3">
            O que o solo mediu e quanto tempo de rega o nó aplicou. O balanço por evapotranspiração (FAO-56) aparece quando o planejador de irrigação passar a gravar o déficit.
          </p>
          <StateMessage v-if="!balance.length" kind="empty" title="Sem leituras nem regas nos últimos 7 dias" />
          <v-table v-else density="comfortable">
            <thead>
              <tr><th>Dia</th><th class="text-right">Umidade média</th><th class="text-right">Mín. a máx.</th><th class="text-right">Rega aplicada</th></tr>
            </thead>
            <tbody>
              <tr v-for="d in balance" :key="d.day.getTime()">
                <td class="cota">{{ dayMonth(d.day) }}</td>
                <td class="cota text-right">{{ d.avg !== null ? pct(d.avg) : '—' }}</td>
                <td class="cota text-right">{{ d.min !== null && d.max !== null ? `${d.min.toFixed(1)} a ${d.max.toFixed(1)}` : '—' }}</td>
                <td class="cota text-right">{{ d.irrigations ? `${duration(d.irrigatedSeconds)} em ${d.irrigations}` : 'nenhuma' }}</td>
              </tr>
            </tbody>
          </v-table>
        </v-card>

        <v-card class="pa-4">
          <h2 class="text-h6 mb-3">Decisões e motivos</h2>
          <StateMessage v-if="decisions.error.value" kind="error" title="Não foi possível carregar as decisões" :detail="decisions.error.value.message" />
          <StateMessage v-else-if="!decisions.data.value?.length" kind="empty" title="Nenhuma decisão registrada para este canteiro" detail="Regas pedidas e alertas das regras de segurança aparecem aqui, com o motivo." />
          <ol v-else class="timeline">
            <li v-for="d in decisions.data.value" :key="d.at.getTime() + d.reason">
              <span class="cota when">{{ dayMonthTime(d.at) }}</span>
              <div>
                <p>{{ d.reason }}</p>
                <p class="text-body-2 text-medium-emphasis">
                  <v-chip :color="d.kind === 'alert' ? 'warning' : d.outcome === 'confirmed' ? 'success' : undefined">{{ d.kind === 'alert' ? 'alerta' : 'rega' }}</v-chip>
                  {{ OUTCOME[d.outcome] ?? d.outcome }}<template v-if="d.durationSeconds"> · {{ duration(d.durationSeconds) }}</template>
                </p>
              </div>
            </li>
          </ol>
        </v-card>
      </div>
    </template>
  </div>
</template>

<style scoped lang="scss">
.grid {
  display: grid;
  gap: 16px;
  grid-template-columns: minmax(0, 1fr);
  @media (min-width: 960px) {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}
.sw-bar {
  display: inline-block;
  width: 14px;
  height: 6px;
  margin-right: 6px;
  background: rgb(var(--v-theme-accent));
}
.timeline {
  list-style: none;
  padding: 0;
  li {
    display: grid;
    grid-template-columns: 96px 1fr;
    gap: 12px;
    padding: 10px 0;
    border-top: 1px solid rgba(var(--v-border-color), var(--v-border-opacity));
  }
  .when {
    font-size: 0.8125rem;
    opacity: 0.8;
  }
}
</style>
