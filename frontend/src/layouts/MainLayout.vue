<template>
  <div class="main-layout">
    <header class="topbar">
      <div class="topbar-left">
        <div class="logo-icon">⚡</div>
        <div class="logo-text">YX-DAQ</div>
      </div>
      <nav class="topbar-nav">
        <router-link
          v-for="item in navItems"
          :key="item.path"
          :to="item.path"
          class="nav-item"
          :class="{ active: $route.path === item.path || ($route.path === '/' && item.path === '/') }"
        >
          <span class="nav-icon">{{ item.icon }}</span>
          <span class="nav-label">{{ item.label }}</span>
        </router-link>
      </nav>
      <div class="topbar-right">
        <span class="status-dot" :class="deviceConnected ? 'connected' : 'disconnected'"></span>
        <span class="status-text">{{ deviceConnected ? '设备已连接' : '设备未连接' }}</span>
        <span class="status-divider">|</span>
        <span class="status-path" :title="dataSavePath">📂 {{ dataSavePath }}</span>
        <span class="status-divider">|</span>
        <span class="status-time">{{ currentTime }}</span>
      </div>
    </header>
    <main class="content-area">
      <router-view v-slot="{ Component }">
        <keep-alive include="DashboardView">
          <component :is="Component" />
        </keep-alive>
      </router-view>
    </main>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useRoute } from 'vue-router'
import { useDeviceStore } from '../stores/device'

const route = useRoute()
const deviceStore = useDeviceStore()

const navItems = [
  { path: '/', icon: '📊', label: '仪表盘' },
  { path: '/device', icon: '📡', label: '设备管理' },
  { path: '/motion', icon: '🎯', label: '运动控制' },
  // { path: '/calibration', icon: '🔬', label: '五孔校准' },
  { path: '/three-hole-test', icon: '🔧', label: '三孔插值移位测试' },
  { path: '/settings', icon: '⚙️', label: '设置' },
]

const deviceConnected = computed(() => deviceStore.isConnected)

const dataSavePath = ref('')

const currentTime = ref('')
let timer: number | null = null

const updateTime = () => {
  const now = new Date()
  currentTime.value = now.toLocaleTimeString('zh-CN', { hour12: false })
}

onMounted(async () => {
  updateTime()
  timer = window.setInterval(updateTime, 1000)
  try {
    const { GetDataDir } = await import('../../wailsjs/go/main/App')
    dataSavePath.value = await GetDataDir() as string
  } catch (e) {
    console.error('load data save path failed:', e)
  }
})

onUnmounted(() => {
  if (timer) clearInterval(timer)
})
</script>

<style lang="scss" scoped>
.main-layout {
  display: flex;
  flex-direction: column;
  width: 100%;
  height: 100vh;
  background: var(--bg-primary, #0a0a1a);
  color: var(--text-primary, #ffffff);
  overflow: hidden;
}

.topbar {
  height: 48px;
  min-height: 48px;
  display: flex;
  align-items: center;
  padding: 0 16px;
  background: var(--bg-secondary, rgba(255,255,255,0.06));
  border-bottom: 1px solid var(--border-color, rgba(255,255,255,0.12));
  backdrop-filter: blur(16px);
  gap: 16px;
}

.topbar-left {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-shrink: 0;
}

.logo-icon {
  font-size: 20px;
  filter: drop-shadow(0 0 8px rgba(184, 41, 255, 0.6));
}

.logo-text {
  font-size: 16px;
  font-weight: bold;
  background: linear-gradient(135deg, #b829ff, #00f5ff);
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
  background-clip: text;
}

.topbar-nav {
  display: flex;
  align-items: center;
  gap: 2px;
  flex: 1;
  padding: 0 12px;
}

.nav-item {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 6px 14px;
  border-radius: 6px;
  color: var(--text-secondary, rgba(255,255,255,0.8));
  text-decoration: none;
  transition: all 150ms ease;
  font-size: 13px;
  white-space: nowrap;

  &:hover {
    background: var(--bg-hover, rgba(255,255,255,0.05));
    color: var(--text-primary, #ffffff);
  }

  &.active {
    background: rgba(184, 41, 255, 0.15);
    color: #b829ff;
    box-shadow: 0 0 10px rgba(184, 41, 255, 0.2);
  }
}

.nav-icon {
  font-size: 16px;
}

.nav-label {
  font-weight: 500;
}

.topbar-right {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 12px;
  color: var(--text-secondary, rgba(255,255,255,0.8));
  flex-shrink: 0;
}

.status-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: #666;

  &.connected {
    background: #00ff88;
    box-shadow: 0 0 8px rgba(0, 255, 136, 0.6);
  }

  &.disconnected {
    background: #ff3366;
    box-shadow: 0 0 8px rgba(255, 51, 102, 0.4);
  }
}

.status-divider {
  color: var(--text-muted, rgba(255,255,255,0.3));
}

.status-path {
  max-width: 260px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: rgba(0,245,255,0.7);
  font-family: monospace;
  font-size: 11px;
}

.content-area {
  flex: 1;
  overflow-y: auto;
  padding: 16px;
}
</style>
