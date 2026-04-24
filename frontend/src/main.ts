import { createApp } from 'vue'
import { createPinia } from 'pinia'
import { createRouter, createWebHashHistory } from 'vue-router'
import ElementPlus from 'element-plus'
import 'element-plus/dist/index.css'
import './assets/styles/themes/theme-variables.scss'
import './assets/styles/global.scss'

import App from './App.vue'
import MainLayout from './layouts/MainLayout.vue'
import DashboardView from './views/DashboardView.vue'
import DeviceView from './views/DeviceView.vue'
import MotionView from './views/MotionView.vue'
import CalibrationView from './views/CalibrationView.vue'
import ThreeHoleTestView from './views/ThreeHoleTestView.vue'
import SettingsView from './views/SettingsView.vue'

const routes = [
  {
    path: '/',
    component: MainLayout,
    children: [
      { path: '', name: 'dashboard', component: DashboardView },
      { path: 'device', name: 'device', component: DeviceView },
      { path: 'motion', name: 'motion', component: MotionView },
      { path: 'calibration', name: 'calibration', component: CalibrationView },
      { path: 'three-hole-test', name: 'three-hole-test', component: ThreeHoleTestView },
      { path: 'settings', name: 'settings', component: SettingsView },
    ]
  }
]

const router = createRouter({
  history: createWebHashHistory(),
  routes,
})

const pinia = createPinia()
const app = createApp(App)

app.use(pinia)
app.use(router)
app.use(ElementPlus, { size: 'default' })
app.mount('#app')
