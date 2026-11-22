<script setup lang="ts">
import { watchEffect } from 'vue'
import { useRouter } from 'vue-router'
import { useGardens } from '@/api/queries'
import StateMessage from '@/components/StateMessage.vue'

const router = useRouter()
const { data, isPending, error } = useGardens()

watchEffect(() => {
  const first = data.value?.[0]
  if (first) router.replace({ name: 'map', params: { gardenId: first.id } })
})
</script>

<template>
  <div class="page">
    <StateMessage v-if="isPending" kind="loading" title="Abrindo a prancheta" />
    <StateMessage v-else-if="error" kind="error" title="Não foi possível listar as hortas" :detail="error.message" />
    <StateMessage v-else-if="!data?.length" kind="empty" title="Nenhuma horta cadastrada neste tenant" detail="Cadastre a primeira horta pela API (POST /v1/gardens) para ver a planta." />
  </div>
</template>
