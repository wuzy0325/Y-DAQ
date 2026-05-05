import { createRouter, createWebHashHistory } from 'vue-router'
import MainLayout from '../layouts/MainLayout.vue'

const routes = [
  {
    path: '/',
    component: MainLayout,
    children: [
      {
        path: '',
        name: 'dashboard',
        component: () => import('../views/DashboardView.vue')
      },
      {
        path: 'device',
        name: 'device',
        component: () => import('../views/DeviceView.vue')
      },
      {
        path: 'motion',
        name: 'motion',
        component: () => import('../views/MotionView.vue')
      },
      {
        path: 'calibration',
        name: 'calibration',
        component: () => import('../views/CalibrationView.vue')
      },
      {
        path: 'settings',
        name: 'settings',
        component: () => import('../views/SettingsView.vue')
      },
    ],
  },
  // 探针测试窗口独立路由（无 MainLayout 导航栏）
  {
    path: '/three-hole-test',
    name: 'three-hole-test',
    component: () => import('../views/ThreeHoleTestView.vue'),
  },
]

export const router = createRouter({
  history: createWebHashHistory(),
  routes,
})
