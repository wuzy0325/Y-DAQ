import { createApp } from 'vue'
import { createPinia } from 'pinia'
import './assets/styles/themes/theme-variables.scss'
import './assets/styles/global.scss'

import App from './App.vue'
import { router } from './router'

const el = document.getElementById('app')
if (el) el.innerHTML = '<div style="color:#0f0;padding:20px;font-size:18px;">Loading YX-DAQ...</div>'

try {
  const app = createApp(App)
  const pinia = createPinia()
  app.use(pinia)
  app.use(router)
  app.mount('#app')
} catch (e: any) {
  if (el) el.innerHTML = `<div style="color:#f00;padding:20px;font-size:14px;white-space:pre-wrap;">Vue mount error: ${e?.stack || e}</div>`
}
