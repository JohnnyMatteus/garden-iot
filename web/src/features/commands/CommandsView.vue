<script setup lang="ts">
import { computed } from 'vue'
import { useCommands, useNodes, useOverview } from '@/api/queries'
import type { CommandStatus } from '@/api/schemas'
import StateMessage from '@/components/StateMessage.vue'
import { dayMonthTime, duration, hhmmss } from '@/lib/format'
import TwinCard from './TwinCard.vue'

const props = defineProps<{ gardenId: string }>()
const commands = useCommands(() => props.gardenId)
const nodes = useNodes(() => props.gardenId)
const overview = useOverview(() => props.gardenId)

const actuators = computed(() => nodes.data.value?.filter((n) => n.kind === 'actuator') ?? [])
const bedName = (nodeId: string) => {
  const bedId = nodes.data.value?.find((n) => n.id === nodeId)?.bedId
  return overview.data.value?.beds.find((b) => b.id === bedId)?.name ?? '—'
}

const STATUS: Record<CommandStatus, { text: string; color?: string }> = {
  pending: { text: 'pendente', color: 'warning' },
  sent: { text: 'enviado, aguardando o nó', color: 'accent' },
  confirmed: { text: 'confirmado', color: 'success' },
  rejected: { text: 'recusado pelo nó', color: 'error' },
  expired: { text: 'expirou sem resposta', color: 'error' },
  failed: { text: 'falhou ao publicar', color: 'error' },
}
</script>

<template>
  <div class="page">
    <p class="legenda text-medium-emphasis">Atuadores</p>
    <h1 class="text-h5 mb-4">Regas e comandos</h1>

    <section aria-label="Estado desejado e reportado" class="twins mb-6">
      <StateMessage v-if="nodes.isPending.value" kind="loading" title="Consultando os atuadores" />
      <StateMessage v-else-if="!actuators.length" kind="empty" title="Nenhum nó atuador cadastrado" detail="Sem atuador não há válvula para comandar nem estado para comparar." />
      <TwinCard v-for="n in actuators" :key="n.id" :garden-id="gardenId" :node-id="n.id" :bed-name="bedName(n.id)" />
    </section>

    <h2 class="text-h6 mb-2">Histórico</h2>
    <StateMessage v-if="commands.isPending.value" kind="loading" title="Carregando comandos" />
    <StateMessage v-else-if="commands.error.value" kind="error" title="Não foi possível carregar o histórico" :detail="commands.error.value.message" />
    <StateMessage v-else-if="!commands.data.value?.length" kind="empty" title="Nenhuma rega pedida ainda" detail="Peça uma rega no mapa; ela aparece aqui com o resultado confirmado pelo nó." />
    <v-card v-else>
      <v-table density="comfortable">
        <thead>
          <tr><th>Quando</th><th>Canteiro</th><th>Nó</th><th class="text-right">Duração</th><th>Pedido por</th><th>Resultado</th></tr>
        </thead>
        <tbody>
          <tr v-for="c in commands.data.value" :key="c.id">
            <td class="cota">{{ dayMonthTime(c.createdAt) }}</td>
            <td>{{ bedName(c.nodeId) }}</td>
            <td class="cota">{{ c.nodeId }}</td>
            <td class="cota text-right">{{ duration(c.durationSeconds) }}</td>
            <td>{{ c.requestedBy }}</td>
            <td>
              <v-chip :color="STATUS[c.status].color">{{ STATUS[c.status].text }}</v-chip>
              <span v-if="c.status === 'sent' || c.status === 'pending'" class="cota text-medium-emphasis ml-2">vence {{ hhmmss(c.expiresAt) }}</span>
            </td>
          </tr>
        </tbody>
      </v-table>
    </v-card>
  </div>
</template>

<style scoped>
.twins {
  display: grid;
  gap: 12px;
  grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
}
</style>
