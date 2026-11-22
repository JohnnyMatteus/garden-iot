<script setup lang="ts">
import { mdiBrightness6, mdiLogout } from '@mdi/js'
import { useRoute, useRouter } from 'vue-router'
import { useTheme } from 'vuetify'
import { useStorage } from '@vueuse/core'
import { watchEffect } from 'vue'
import { claims, isAuthenticated, signOut } from '@/features/auth/session'

const route = useRoute()
const router = useRouter()
const theme = useTheme()
const stored = useStorage<'cotaLight' | 'cotaDark'>('cota.theme', 'cotaLight')
watchEffect(() => theme.change(stored.value))

const toggleTheme = () => (stored.value = stored.value === 'cotaLight' ? 'cotaDark' : 'cotaLight')
const leave = () => {
  signOut()
  router.push({ name: 'login' })
}
const tabs = [
  { name: 'map', label: 'Mapa' },
  { name: 'commands', label: 'Regas' },
  { name: 'devices', label: 'Dispositivos' },
] as const
</script>

<template>
  <v-app>
    <v-app-bar v-if="isAuthenticated">
      <router-link :to="{ name: 'home' }" class="brand" aria-label="Cota, início">
        <svg width="22" height="22" viewBox="0 0 32 32" aria-hidden="true">
          <rect x="5" y="5" width="22" height="22" fill="none" stroke="currentColor" stroke-width="2.5" />
          <rect x="5" y="16" width="22" height="11" fill="rgb(var(--v-theme-water-fill))" />
          <path d="M5 16h22" stroke="rgb(var(--v-theme-primary))" stroke-width="2.5" />
        </svg>
        <span>Cota</span>
      </router-link>
      <nav v-if="route.params.gardenId" class="tabs" aria-label="Seções">
        <router-link
          v-for="t in tabs" :key="t.name"
          :to="{ name: t.name, params: { gardenId: route.params.gardenId } }"
          class="tab" :class="{ active: route.name === t.name || (t.name === 'map' && route.name === 'bed') }"
        >{{ t.label }}</router-link>
      </nav>
      <v-spacer />
      <span class="cota text-medium-emphasis d-none d-sm-inline mr-2">{{ claims?.tenant }} · {{ claims?.role }}</span>
      <v-btn icon variant="text" aria-label="Alternar tema claro e escuro" @click="toggleTheme">
        <v-icon :icon="mdiBrightness6" />
      </v-btn>
      <v-btn icon variant="text" aria-label="Sair" @click="leave"><v-icon :icon="mdiLogout" /></v-btn>
    </v-app-bar>
    <v-main>
      <router-view />
    </v-main>
  </v-app>
</template>

<style scoped lang="scss">
.brand {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  padding: 0 16px;
  font-weight: 600;
  font-size: 1.125rem;
  color: rgb(var(--v-theme-on-surface));
  text-decoration: none;
}
.tabs {
  display: flex;
  gap: 4px;
  overflow-x: auto;
}
.tab {
  display: inline-flex;
  align-items: center;
  min-height: 44px;
  padding: 0 12px;
  color: rgb(var(--v-theme-on-surface));
  opacity: 0.7;
  text-decoration: none;
  border-bottom: 2px solid transparent;
  white-space: nowrap;
  &.active {
    opacity: 1;
    border-bottom-color: rgb(var(--v-theme-primary));
    font-weight: 600;
  }
}
</style>
