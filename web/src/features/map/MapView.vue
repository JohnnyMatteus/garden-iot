<script setup lang="ts">
import { mdiTable } from '@mdi/js'
import { computed, ref } from 'vue'
import { useGardens, useOverview } from '@/api/queries'
import GardenPlan from '@/components/signature/GardenPlan.vue'
import { valveText, valveView } from '@/components/signature/plan-geometry'
import StateMessage from '@/components/StateMessage.vue'
import { ago, pct } from '@/lib/format'
import BedPanel from './BedPanel.vue'

const props = defineProps<{ gardenId: string }>()
const { data: overview, isPending, error } = useOverview(() => props.gardenId)
const { data: gardens } = useGardens()

const gardenName = computed(() => gardens.value?.find((g) => g.id === props.gardenId)?.name ?? props.gardenId)
const picked = ref<string | null>(null)
const selected = computed(() => overview.value?.beds.find((b) => b.id === (picked.value ?? overview.value?.beds[0]?.id)) ?? null)

const below = computed(() => overview.value?.beds.filter((b) => b.moistureStatus === 'below').length ?? 0)
const irrigating = computed(() => overview.value?.beds.filter((b) => b.irrigating).length ?? 0)
const lastRead = computed(() => {
  const times = overview.value?.beds.flatMap((b) => (b.readAt ? [b.readAt.getTime()] : [])) ?? []
  return times.length ? new Date(Math.max(...times)) : null
})
const STATUS_TEXT = { below: 'abaixo da faixa', within: 'dentro da faixa', above: 'acima da faixa', unknown: 'sem leitura' } as const
</script>

<template>
  <div class="page">
    <StateMessage v-if="isPending" kind="loading" title="Desenhando a planta da horta" />
    <StateMessage v-else-if="error" kind="error" title="A planta não carregou" :detail="error.message" />
    <StateMessage
      v-else-if="overview && !overview.beds.length" kind="empty" title="Esta horta ainda não tem canteiros"
      detail="Cadastre canteiros e nós pela API; assim que o primeiro sensor reportar, o nível de água aparece aqui."
    />
    <template v-else-if="overview">
      <header class="head">
        <div>
          <p class="legenda text-medium-emphasis">Planta baixa</p>
          <h1 class="text-h5">{{ gardenName }}</h1>
        </div>
        <p class="cota summary" aria-live="polite">
          <span><strong>{{ below }}</strong> de {{ overview.beds.length }} canteiros abaixo da faixa</span>
          <span><strong>{{ irrigating }}</strong> regando agora</span>
          <span><strong>{{ overview.openAlerts }}</strong> alertas abertos</span>
          <span>última leitura {{ lastRead ? ago(lastRead) : 'há mais de 30 minutos' }}</span>
        </p>
      </header>

      <div class="layout">
        <section class="board" aria-label="Planta da horta">
          <GardenPlan :beds="overview.beds" :selected-id="selected?.id" :name="gardenName" @select="picked = $event" />
          <ul class="legend-row legenda text-medium-emphasis" aria-label="Legenda">
            <li><i class="sw sw-water" />nível de água</li>
            <li><i class="sw sw-dry" />déficit até a faixa</li>
            <li><i class="sw sw-valve" />válvula fechada</li>
            <li><i class="sw sw-valve open" />aberta, rega em curso</li>
            <li><i class="sw sw-unknown" />sem leitura</li>
          </ul>
        </section>
        <aside v-if="selected" aria-label="Canteiro selecionado">
          <BedPanel :garden-id="gardenId" :bed="selected" />
        </aside>
      </div>

      <details class="mt-6">
        <summary class="text-body-2"><v-icon :icon="mdiTable" size="18" class="mr-1" />Ver a planta como tabela</summary>
        <v-table class="mt-2">
          <thead>
            <tr><th>Canteiro</th><th>Cultura</th><th class="text-right">Umidade</th><th>Situação</th><th>Válvula</th></tr>
          </thead>
          <tbody>
            <tr v-for="b in overview.beds" :key="b.id">
              <td>{{ b.name }}</td>
              <td>{{ b.crop || '—' }}</td>
              <td class="cota text-right">{{ b.moisturePct !== undefined ? pct(b.moisturePct) : '—' }}</td>
              <td>{{ STATUS_TEXT[b.moistureStatus] }}</td>
              <td>{{ valveText(valveView(b)) }}</td>
            </tr>
          </tbody>
        </v-table>
      </details>
    </template>
  </div>
</template>

<style scoped lang="scss">
.head {
  display: flex;
  flex-wrap: wrap;
  justify-content: space-between;
  align-items: flex-end;
  gap: 8px 24px;
  margin-bottom: 16px;
}
.summary {
  display: flex;
  flex-wrap: wrap;
  gap: 4px 20px;
  font-size: 0.8125rem;
}
.layout {
  display: grid;
  gap: 16px;
  grid-template-columns: minmax(0, 1fr);
  @media (min-width: 960px) {
    grid-template-columns: minmax(0, 1fr) 380px;
    align-items: start;
  }
}
.board {
  background: rgb(var(--v-theme-surface));
  border: 1px solid rgba(var(--v-border-color), var(--v-border-opacity));
  border-radius: 4px;
  padding: 8px;
  // papel milimetrado: grade de 16 px bem leve atrás da planta
  background-image:
    linear-gradient(rgba(var(--v-border-color), 0.06) 1px, transparent 1px),
    linear-gradient(90deg, rgba(var(--v-border-color), 0.06) 1px, transparent 1px);
  background-size: 16px 16px;
}
.legend-row {
  display: flex;
  flex-wrap: wrap;
  gap: 4px 16px;
  list-style: none;
  padding: 8px 4px 4px;
}
.sw {
  display: inline-block;
  width: 14px;
  height: 10px;
  margin-right: 6px;
  vertical-align: -1px;
  border: 1px solid rgb(var(--v-theme-on-surface));
  &-water { background: rgb(var(--v-theme-water-fill)); }
  &-dry { background: repeating-linear-gradient(45deg, rgb(var(--v-theme-dry-hatch)) 0 1px, transparent 1px 4px); }
  &-valve { background: rgb(var(--v-theme-surface)); &.open { background: rgb(var(--v-theme-accent)); border-color: rgb(var(--v-theme-accent)); } }
  &-unknown { border-style: dashed; }
}
summary {
  cursor: pointer;
  min-height: 44px;
  display: flex;
  align-items: center;
}
</style>
