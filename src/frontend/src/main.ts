import { createApp } from 'vue'
import { createPinia } from 'pinia'
import './assets/styles/themes/theme-variables.scss'
import './assets/styles/global.scss'

import App from './App.vue'
import { router } from './router'

function showStartupError(message: string) {
  const root = document.getElementById('app')
  if (!root) return
  root.innerHTML = `
    <div style="padding:24px;color:#ffb4b4;background:#0a0a1a;font:14px/1.6 Consolas,monospace;white-space:pre-wrap;">
      <h2 style="margin:0 0 12px;color:#ff5c7a;">前端启动失败</h2>
      <div>${message.replace(/[&<>]/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;' }[c]!))}</div>
    </div>
  `
}

window.addEventListener('error', event => {
  const message = `${event.message}\n${event.filename}:${event.lineno}:${event.colno}\n${event.error?.stack || ''}`
  localStorage.setItem('yx-daq:last-frontend-error', message)
  showStartupError(message)
})

window.addEventListener('unhandledrejection', event => {
  const reason = event.reason
  const message = reason?.stack || reason?.message || String(reason)
  localStorage.setItem('yx-daq:last-frontend-error', message)
  showStartupError(message)
})

const app = createApp(App)
const pinia = createPinia()
app.use(pinia)
app.use(router)
app.mount('#app')
