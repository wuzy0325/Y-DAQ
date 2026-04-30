import { createRouter, createWebHashHistory } from 'vue-router'
import MainLayout from '../layouts/MainLayout.vue'
import CalibrationView from '../views/CalibrationView.vue'
import DashboardView from '../views/DashboardView.vue'
import DeviceView from '../views/DeviceView.vue'
import MotionView from '../views/MotionView.vue'
import SettingsView from '../views/SettingsView.vue'
import ThreeHoleTestView from '../views/ThreeHoleTestView.vue'

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
    ],
  },
]

export const router = createRouter({
  history: createWebHashHistory(),
  routes,
})
