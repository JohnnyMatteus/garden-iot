import '@fontsource-variable/instrument-sans'
import '@fontsource-variable/spline-sans-mono'
import { VueQueryPlugin } from '@tanstack/vue-query'
import { createApp } from 'vue'
import App from './App.vue'
import vuetify from './plugins/vuetify'
import router from './router'
import './styles/main.scss'

createApp(App).use(vuetify).use(router).use(VueQueryPlugin, {
  queryClientConfig: { defaultOptions: { queries: { staleTime: 5_000, retry: 1, refetchOnWindowFocus: true } } },
}).mount('#app')
