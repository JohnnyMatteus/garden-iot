<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { signIn } from './session'

const router = useRouter()
const token = ref('')
const error = ref('')

function submit() {
  if (!signIn(token.value)) {
    error.value = 'Esse texto não é um token da API (esperado: JWT com tenant, papel e validade).'
    return
  }
  router.push({ name: 'home' })
}
</script>

<template>
  <main class="login">
    <v-card class="pa-6" max-width="520" width="100%">
      <p class="legenda text-medium-emphasis mb-1">Prancheta da horta</p>
      <h1 class="text-h5 mb-2">Entrar na Cota</h1>
      <p class="text-body-2 text-medium-emphasis mb-4">
        A API emite tokens JWT por tenant e papel. Em desenvolvimento, gere um com
        <code class="cota">make token TENANT=demo ROLE=operator</code> e cole abaixo.
      </p>
      <form @submit.prevent="submit">
        <v-textarea
          v-model="token" label="Token de acesso" rows="4" auto-grow variant="outlined" class="cota"
          :error-messages="error" autocomplete="off" spellcheck="false" @update:model-value="error = ''"
        />
        <v-btn type="submit" color="primary" block :disabled="!token.trim()">Entrar</v-btn>
      </form>
    </v-card>
  </main>
</template>

<style scoped>
.login {
  min-height: 100vh;
  display: grid;
  place-items: center;
  padding: 16px;
  background: rgb(var(--v-theme-background));
}
</style>
