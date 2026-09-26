import { createApp } from 'vue'
import { createPinia } from 'pinia'
// ElMessage/ElMessageBox 为编程式调用，unplugin-vue-components 不会自动注入其样式，
// 必须显式引入，否则弹出层无任何样式（关闭按钮变成浏览器原生按钮）
import 'element-plus/es/components/message/style/css'
import 'element-plus/es/components/message-box/style/css'
import './assets/styles/themes/theme-variables.scss'
import './assets/styles/global.scss'

import App from './App.vue'
import { router } from './router'
import { reportFrontendError } from './utils/frontendLog'

// 应用挂载前发生的错误才替换页面显示启动失败；挂载后的错误仅记录日志，不破坏运行中的界面
let appMounted = false

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
  reportFrontendError('error', `uncaught: ${event.message}`, event.error?.stack || message)
  if (!appMounted) showStartupError(message)
})

window.addEventListener('unhandledrejection', event => {
  const reason = event.reason
  const message = reason?.stack || reason?.message || String(reason)
  localStorage.setItem('yx-daq:last-frontend-error', message)
  reportFrontendError('error', `unhandledrejection: ${reason?.message || String(reason)}`, reason?.stack || '')
  if (!appMounted) showStartupError(message)
})

const app = createApp(App)
const pinia = createPinia()
app.use(pinia)
app.use(router)

app.config.errorHandler = (err, _instance, info) => {
  const error = err instanceof Error ? err : new Error(String(err))
  reportFrontendError('error', `vue(${info}): ${error.message}`, error.stack || '')
}

app.mount('#app')
appMounted = true
