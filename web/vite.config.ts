import { fileURLToPath, URL } from 'node:url'
import vue from '@vitejs/plugin-vue'
import vuetify from 'vite-plugin-vuetify'
import { defineConfig } from 'vitest/config'

// Em dev o Vite faz o papel do nginx: /api vai para a API Go sem o prefixo.
const apiTarget = process.env.VITE_API_TARGET ?? 'http://localhost:8090'

export default defineConfig({
  plugins: [vue(), vuetify({ autoImport: true, styles: { configFile: 'src/styles/settings.scss' } })],
  resolve: { alias: { '@': fileURLToPath(new URL('./src', import.meta.url)) } },
  server: {
    port: 5173,
    proxy: { '/api': { target: apiTarget, changeOrigin: true, rewrite: (p) => p.replace(/^\/api/, '') } },
  },
  test: { environment: 'jsdom', include: ['src/**/*.test.ts'], server: { deps: { inline: ['vuetify'] } } },
})
