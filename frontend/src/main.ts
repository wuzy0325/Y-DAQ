import { createApp } from 'vue'
import { createPinia } from 'pinia'
import ElementPlus from 'element-plus'
import 'element-plus/dist/index.css'
import './assets/styles/themes/theme-variables.scss'
import './assets/styles/global.scss'

import App from './App.vue'
import { router } from './router'

const pinia = createPinia()
const app = createApp(App)

app.use(pinia)
app.use(router)
app.use(ElementPlus, { size: 'default' })
app.mount('#app')
