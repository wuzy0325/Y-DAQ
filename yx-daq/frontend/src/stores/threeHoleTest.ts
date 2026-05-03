import { ref, computed } from 'vue'
import { defineStore } from 'pinia'
import {
  SelectThreeHoleCalibFiles,
  LoadThreeHoleCalibFiles,
  GetThreeHoleCalibInfo,
  StartThreeHoleTraversal,
  PauseThreeHoleTraversal,
  ResumeThreeHoleTraversal,
  StopThreeHoleTraversal,
  GetThreeHoleTraversalStatus,
  StartThreeHoleRealtimeMonitor,
  StopThreeHoleRealtimeMonitor,
  SaveThreeHoleConfig,
  LoadThreeHoleConfig,
  ConnectDevice,
  StartAcquisition,
  GetDeviceStatusAll,
} from '../../wailsjs/go/main/App'
import { EventsOn, EventsOff } from '../../wailsjs/runtime/runtime'
import {
  ThreeHoleChannelRole,
  TraversalPattern,
  type ThreeHoleChannelRoleValue,
  type TraversalPatternValue,
} from '../api/enums'
import { useMotionStore } from './motion'

// ==================== 类型定义 ====================

interface ThreeHoleRawData {
  p1: number; p2: number; p3: number; pAtm: number; tAtm: number
}

interface ThreeHoleInterpolationResult {
  ptProbe: number; psProbe: number; machProbe: number; alphaProbe: number
  iterationCount: number; converged: boolean; valid: boolean; errorMsg?: string
}

interface ThreeHoleTraversalDataPoint {
  pointId: string; x: number; y: number
  rawData: ThreeHoleRawData; interpResult: ThreeHoleInterpolationResult
  sampleCount: number; timestamp: number
}

interface ThreeHoleTraversalTaskStatus {
  taskId: string; status: string
  totalPoints: number; completedPoints: number; progress: number
  currentPoint: { id: string; x: number; y: number } | null
  dataPoints: ThreeHoleTraversalDataPoint[]
  lastError: string
}

interface ThreeHoleTraversalProgressEvent {
  taskId: string; totalPoints: number; completedPoints: number
  progress: number; currentX: number; currentY: number
  phase?: string
}

interface ThreeHoleTraversalRealtimeEvent {
  taskId: string; pointId: string
  rawData: ThreeHoleRawData; interpResult: ThreeHoleInterpolationResult
}

interface ThreeHoleTraversalCompleteEvent {
  taskId: string; status: string
  dataPoints: ThreeHoleTraversalDataPoint[]
}

interface ThreeHoleTraversalErrorEvent {
  taskId: string; error: string; isFatal: boolean
}

// ==================== 配置类型 ====================

interface StepSegment {
  start: number; end: number; step: number
}

interface LineLayout {
  startX: number; startY: number; endX: number; endY: number
  xSteps: StepSegment[]; ySteps: StepSegment[]
}

interface RectangleLayout {
  xMin: number; xMax: number; yMin: number; yMax: number
  xSteps: StepSegment[]; ySteps: StepSegment[]
}

interface TraversalLayout {
  pattern: TraversalPatternValue
  line?: LineLayout
  rectangle?: RectangleLayout
  customPoints?: { id: string; x: number; y: number }[]
}

interface ThreeHoleProbeChannelConfig {
  name: string; role: ThreeHoleChannelRoleValue; channel: number; enabled: boolean
}

interface MotionAxisMapping {
  axis: string
}

interface ThreeHoleCalibFileInfo {
  filePath: string; fileName: string; cMa: number
}

interface ThreeHoleTraversalConfig {
  name: string
  deviceId: string
  motionControllerId: string
  layout: TraversalLayout
  probeChannels: ThreeHoleProbeChannelConfig[]
  motionAlpha: MotionAxisMapping
  motionBeta: MotionAxisMapping
  calibFiles: ThreeHoleCalibFileInfo[]
  dwellTimeMs: number
  samplesPerPoint: number
  sampleIntervalMs: number
  motionTimeoutMs: number
  savePath: string
  saveFileName: string
}

// ==================== Store ====================

export const useThreeHoleTestStore = defineStore('threeHoleTest', () => {
  // 状态
  const taskStatus = ref<ThreeHoleTraversalTaskStatus | null>(null)
  const progress = ref<ThreeHoleTraversalProgressEvent | null>(null)
  const realtime = ref<ThreeHoleTraversalRealtimeEvent | null>(null)
  const isRunning = computed(() => taskStatus.value?.status === 'running' || taskStatus.value?.status === 'paused')
  const isPaused = computed(() => taskStatus.value?.status === 'paused')
  const calibLoaded = ref(false)
  const calibFiles = ref<string[]>([])
  const lastError = ref<string>('')

  // 配置持久化 key
  const CONFIG_STORAGE_KEY = 'threeHoleTestConfig'

  // 配置
  const config = ref<ThreeHoleTraversalConfig>({
    name: `三孔移位测试-${new Date().toLocaleDateString()}`,
    deviceId: '',
    motionControllerId: '',
    layout: {
      pattern: TraversalPattern.RECTANGLE,
      rectangle: {
        xMin: -20, xMax: 20, yMin: -20, yMax: 20,
        xSteps: [{ start: -20, end: 20, step: 5 }],
        ySteps: [{ start: -20, end: 20, step: 5 }],
      },
    },
    probeChannels: [
      { name: 'P1', role: ThreeHoleChannelRole.P1, channel: 0, enabled: true },
      { name: 'P2', role: ThreeHoleChannelRole.P2, channel: 1, enabled: true },
      { name: 'P3', role: ThreeHoleChannelRole.P3, channel: 2, enabled: true },
      { name: '大气压', role: ThreeHoleChannelRole.P_ATM, channel: 16, enabled: true },
      { name: '大气温度', role: ThreeHoleChannelRole.T_ATM, channel: 17, enabled: true },
    ],
    motionAlpha: { axis: 'X' },
    motionBeta: { axis: 'Y' },
    calibFiles: [],
    dwellTimeMs: 2000,
    samplesPerPoint: 10,
    sampleIntervalMs: 50,
    motionTimeoutMs: 30000,
    savePath: '',
    saveFileName: '',
  })

  // 计算属性
  const statusText = computed(() => {
    if (!taskStatus.value) return '未启动'
    const map: Record<string, string> = {
      idle: '空闲', running: '运行中', paused: '已暂停',
      completed: '已完成', error: '错误',
    }
    return map[taskStatus.value.status] || taskStatus.value.status
  })

  const hasResults = computed(() => (taskStatus.value?.dataPoints?.length ?? 0) > 0)

  // ==================== API 调用 ====================

  async function selectCalibFiles() {
    try {
      const files = await SelectThreeHoleCalibFiles() as string[]
      if (files && files.length > 0) {
        calibFiles.value = files
        await LoadThreeHoleCalibFiles(files)
        const infos = await GetThreeHoleCalibInfo() as { cMa: number }[]
        calibLoaded.value = true
        config.value.calibFiles = files.map((f, i) => ({
          filePath: f,
          fileName: f.split(/[/\\]/).pop() || f,
          cMa: infos[i]?.cMa ?? 0,
        }))

        await ensureDeviceAcquiring()
      }
    } catch (e) {
      console.error('selectCalibFiles failed:', e)
      lastError.value = `加载校准文件失败: ${e}`
    }
  }

  async function ensureDeviceAcquiring() {
    const deviceId = config.value.deviceId
    if (!deviceId) return

    try {
      const statuses = await GetDeviceStatusAll() as { id: string; status: string; acquiring: boolean }[]
      const ds = statuses.find(s => s.id === deviceId)
      if (!ds) return

      if (ds.status !== 'Connected') {
        try {
          await ConnectDevice(deviceId)
        } catch (e) {
          lastError.value = `自动连接设备失败: ${e}`
          return
        }
      }

      const updated = (await GetDeviceStatusAll() as { id: string; status: string; acquiring: boolean }[])
        .find(s => s.id === deviceId)
      if (updated && !updated.acquiring) {
        try {
          await StartAcquisition(deviceId)
        } catch (e) {
          lastError.value = `自动启动采集失败: ${e}`
        }
      }
    } catch (e) {
      console.error('ensureDeviceAcquiring failed:', e)
    }
  }

  async function startTest() {
    if (isRunning.value) return

    // 等待之前的测试完全停止
    if (taskStatus.value?.status === 'running' || taskStatus.value?.status === 'paused') {
      lastError.value = '请先停止当前正在运行的测试'
      return
    }

    if (!calibLoaded.value) {
      lastError.value = '请先加载校准文件'
      return
    }
    // 检查运动控制器连接状态
    const mcId = config.value.motionControllerId
    if (mcId) {
      const motionStore = useMotionStore()
      const mcStatus = motionStore.statuses.find(s => s.id === mcId)
      if (!mcStatus || mcStatus.status !== 'Connected') {
        lastError.value = '运动控制器未连接，请先在设备页面连接运动控制器'
        return
      }
    }

    taskStatus.value = null
    lastError.value = ''
    realtime.value = null
    progress.value = null

    try {
      await StartThreeHoleTraversal(config.value as any)
      // 后端启动成功后，通过 fetchStatus 获取最新 taskStatus，isRunning/isPaused 自动派生
      await fetchStatus()
    } catch (e) {
      console.error('startTest failed:', e)
      lastError.value = `启动测试失败: ${e}`
      // 如果是启动失败，确保清理状态
      taskStatus.value = null
      realtime.value = null
      progress.value = null
    }
  }

  async function pauseTest() {
    try {
      await PauseThreeHoleTraversal()
      await fetchStatus()
    } catch (e) {
      console.error('pauseTest failed:', e)
    }
  }

  async function resumeTest() {
    try {
      await ResumeThreeHoleTraversal()
      await fetchStatus()
    } catch (e) {
      console.error('resumeTest failed:', e)
    }
  }

  async function stopTest() {
    try {
      await StopThreeHoleTraversal()
      // 轮询状态直到确认停止，最多等待2秒
      let retries = 0
      const maxRetries = 10
      while (retries < maxRetries) {
        await fetchStatus()
        const status = taskStatus.value?.status
        if (status === 'idle' || status === 'completed' || status === 'error') {
          realtime.value = null
          progress.value = null
          break
        }
        await new Promise(resolve => setTimeout(resolve, 200))
        retries++
      }
      // 如果超时仍未停止，强制清理状态
      if (retries >= maxRetries) {
        console.warn('stopTest: 超时未收到停止确认，强制清理状态')
        realtime.value = null
        progress.value = null
      }
    } catch (e) {
      console.error('stopTest failed:', e)
      // 如果是停止失败，确保清理状态
      realtime.value = null
      progress.value = null
    }
  }

  // ==================== 实时数据监控 ====================

  async function startRealtimeMonitor() {
    try {
      await StartThreeHoleRealtimeMonitor(config.value as any)
    } catch (e) {
      console.error('startRealtimeMonitor failed:', e)
    }
  }

  async function stopRealtimeMonitor() {
    try {
      await StopThreeHoleRealtimeMonitor()
    } catch (e) {
      console.error('stopRealtimeMonitor failed:', e)
    }
  }

  async function fetchStatus() {
    try {
      taskStatus.value = await GetThreeHoleTraversalStatus() as ThreeHoleTraversalTaskStatus
    } catch (e) {
      console.warn('fetchStatus failed:', e)
    }
  }

  // ==================== 事件监听 ====================

  function startListening() {
    try {
      EventsOn('three-hole:progress', (data: ThreeHoleTraversalProgressEvent) => {
        if (isRunning.value) progress.value = data
      })
      EventsOn('three-hole:realtime', (data: ThreeHoleTraversalRealtimeEvent) => {
        realtime.value = data
      })
      EventsOn('three-hole:complete', async (_data: ThreeHoleTraversalCompleteEvent) => {
        progress.value = null
        await fetchStatus()
      })
      EventsOn('three-hole:error', (data: ThreeHoleTraversalErrorEvent) => {
        lastError.value = data.error
        if (data.isFatal) {
          progress.value = null
        }
      })
    } catch (e) {
      console.warn('startListening failed:', e)
    }
  }

  function stopListening() {
    try {
      EventsOff('three-hole:progress')
      EventsOff('three-hole:realtime')
      EventsOff('three-hole:complete')
      EventsOff('three-hole:error')
    } catch (e) {
      console.warn('stopListening failed:', e)
    }
  }

  // ==================== 错误清除 ====================

  function clearError() {
    lastError.value = ''
  }

  // ==================== CSV 导出 ====================

  function exportCSV() {
    const dataPoints = taskStatus.value?.dataPoints ?? []
    if (dataPoints.length === 0) return

    const BOM = '﻿'
    const headers = [
      '点号', 'X', 'Y', 'P1', 'P2', 'P3', 'P∞', 'T∞',
      '总压Pt', '静压Ps', '马赫数Ma', '攻角Alpha', '迭代次数', '采样数', '时间戳',
    ]
    const rows = dataPoints.map(p => [
      p.pointId, p.x.toFixed(4), p.y.toFixed(4),
      p.rawData.p1.toFixed(6), p.rawData.p2.toFixed(6), p.rawData.p3.toFixed(6),
      p.rawData.pAtm.toFixed(6), p.rawData.tAtm.toFixed(6),
      p.interpResult.ptProbe.toFixed(6), p.interpResult.psProbe.toFixed(6),
      p.interpResult.machProbe.toFixed(6), p.interpResult.alphaProbe.toFixed(4),
      p.interpResult.iterationCount.toString(), p.sampleCount.toString(),
      p.timestamp.toString(),
    ].join(','))

    const csv = BOM + headers.join(',') + '\n' + rows.join('\n')
    const blob = new Blob([csv], { type: 'text/csv;charset=utf-8' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `three-hole-traversal-${new Date().toISOString().slice(0, 19).replace(/:/g, '-')}.csv`
    a.click()
    URL.revokeObjectURL(url)
  }

  // ==================== 配置持久化 ====================

  function saveConfig() {
    try {
      localStorage.setItem(CONFIG_STORAGE_KEY, JSON.stringify(config.value))
      SaveThreeHoleConfig(config.value as any).catch(() => {})
    } catch (e) {
      console.error('保存三孔测试配置失败:', e)
    }
  }

  async function loadConfig() {
    try {
      // 优先从后端加载
      try {
        const loaded = await LoadThreeHoleConfig() as any
        if (loaded && loaded.probeChannels && loaded.probeChannels.length > 0) {
          config.value = loaded as ThreeHoleTraversalConfig
          localStorage.setItem(CONFIG_STORAGE_KEY, JSON.stringify(loaded))
        } else {
          loadConfigFromLocal()
        }
      } catch {
        loadConfigFromLocal()
      }
    } catch (e) {
      console.error('加载三孔测试配置失败:', e)
    }

    // 恢复保存的校准文件路径
    const savedCalibFiles = config.value.calibFiles
    if (savedCalibFiles && savedCalibFiles.length > 0) {
      const filePaths = savedCalibFiles.map(f => f.filePath).filter(Boolean)
      if (filePaths.length > 0) {
        try {
          await LoadThreeHoleCalibFiles(filePaths)
          const infos = await GetThreeHoleCalibInfo() as { cMa: number }[]
          calibFiles.value = filePaths
          calibLoaded.value = true
          config.value.calibFiles = filePaths.map((f, i) => ({
            filePath: f,
            fileName: f.split(/[/\\]/).pop() || f,
            cMa: infos[i]?.cMa ?? 0,
          }))
        } catch (e) {
          console.error('恢复校准文件失败:', e)
        }
      }
    }
  }

  function loadConfigFromLocal() {
    try {
      const raw = localStorage.getItem(CONFIG_STORAGE_KEY)
      if (!raw) return
      const data = JSON.parse(raw)
      if (data && data.probeChannels && data.probeChannels.length > 0) {
        config.value = data as ThreeHoleTraversalConfig
      }
    } catch (e) {
      console.error('从localStorage加载配置失败:', e)
    }
  }

  return {
    // 状态
    taskStatus, progress, realtime, isRunning, isPaused, calibLoaded, calibFiles, lastError,
    config, statusText, hasResults,
    // 方法
    selectCalibFiles, startTest, pauseTest, resumeTest, stopTest,
    ensureDeviceAcquiring,
    fetchStatus, startListening, stopListening, clearError, exportCSV,
    saveConfig, loadConfig,
    startRealtimeMonitor, stopRealtimeMonitor,
  }
})
