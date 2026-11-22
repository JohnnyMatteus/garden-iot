<script setup lang="ts">
import { mdiChip, mdiValve, mdiWaterPercent } from '@mdi/js'
import { computed } from 'vue'
import { useNodes, useOverview } from '@/api/queries'
import StateMessage from '@/components/StateMessage.vue'
import { ago, hhmmss } from '@/lib/format'
import { isOnline, OFFLINE_AFTER_MS } from './status'

const props = defineProps<{ gardenId: string }>()
const nodes = useNodes(() => props.gardenId)
const overview = useOverview(() => props.gardenId)

const rows = computed(() =>
  (nodes.data.value ?? [])
    .map((n) => ({ ...n, online: isOnline(n.lastSeenAt), bed: overview.data.value?.beds.find((b) => b.id === n.bedId)?.name }))
    // offline primeiro: é o que o dono precisa ver
    .sort((a, b) => Number(a.online) - Number(b.online) || a.id.localeCompare(b.id)),
)
const offline = computed(() => rows.value.filter((r) => !r.online).length)
</script>

<template>
  <div class="page">
    <p class="legenda text-medium-emphasis">ESP32 em campo</p>
    <h1 class="text-h5 mb-1">Dispositivos</h1>
    <p v-if="nodes.data.value" class="text-body-2 text-medium-emphasis mb-4" aria-live="polite">
      {{ offline ? `${offline} de ${rows.length} nós sem telemetria há mais de ${OFFLINE_AFTER_MS / 60_000} min.` : 'Todos os nós reportaram nos últimos 5 minutos.' }}
    </p>

    <StateMessage v-if="nodes.isPending.value" kind="loading" title="Chamando os nós" />
    <StateMessage v-else-if="nodes.error.value" kind="error" title="Não foi possível listar os dispositivos" :detail="nodes.error.value.message" />
    <StateMessage v-else-if="!rows.length" kind="empty" title="Nenhum nó provisionado nesta horta" detail="Provisione um dispositivo (make provision) para ele aparecer aqui." />
    <div v-else class="cards">
      <v-card v-for="n in rows" :key="n.id" class="pa-4" :class="{ off: !n.online }">
        <div class="d-flex justify-space-between align-start">
          <div>
            <p class="cota text-h6">{{ n.id }}</p>
            <p class="text-body-2 text-medium-emphasis d-flex align-center ga-1">
              <v-icon :icon="n.kind === 'sensor' ? mdiWaterPercent : mdiValve" size="16" />
              {{ n.kind === 'sensor' ? 'Nó sensor' : 'Nó atuador' }} · {{ n.bed ?? 'sem canteiro' }}
            </p>
          </div>
          <v-chip :color="n.online ? 'success' : 'error'">{{ n.online ? 'online' : 'offline' }}</v-chip>
        </div>
        <dl class="facts mt-3">
          <dt>Última telemetria</dt>
          <dd class="cota">{{ n.lastSeenAt ? `${hhmmss(n.lastSeenAt)} · ${ago(n.lastSeenAt)}` : 'nunca reportou' }}</dd>
          <dt><v-icon :icon="mdiChip" size="14" class="mr-1" />Firmware</dt>
          <dd class="cota">{{ n.fwVersion || 'não informado' }}</dd>
        </dl>
      </v-card>
    </div>
  </div>
</template>

<style scoped>
.cards {
  display: grid;
  gap: 12px;
  grid-template-columns: repeat(auto-fill, minmax(300px, 1fr));
}
.off {
  border-color: rgb(var(--v-theme-error));
}
.facts {
  display: grid;
  grid-template-columns: auto 1fr;
  gap: 4px 12px;
  font-size: 0.875rem;
  dt {
    opacity: 0.72;
  }
  dd {
    text-align: right;
  }
}
</style>
