import { ref, computed } from 'vue'
import { defineStore } from 'pinia'
import {
  SelectFiveHoleCalibFiles,
  LoadFiveHoleCalibFiles,
  GetFiveHoleCalibInfo,
  StartFiveHoleTraversal,
  PauseFiveHoleTraversal,
  ResumeFiveHoleTraversal,
  StopFiveHoleTraversal,
  GetFiveHoleTraversalStatus,
  StartFiveHoleRealtimeMonitor,
  StopFiveHoleRealtimeMonitor,
  SelectAndStartFiveHoleRealtimeRecording,
  StopFiveHoleRealtimeRecording,
  SaveFiveHoleConfig,
  LoadFiveHoleConfig,
  ConnectDevice,
  StartAcquisition,
  GetDeviceStatusAll,
} from '../wails-compat/app'
import { EventsOn, EventsOff } from '../../wailsjs/runtime/runtime'
import {
  FiveHoleChannelRole,
  TraversalPattern,
  AxisName,
} from '../api/enums'
import { useMotionStore } from './motion'
import type {
  FiveHoleTraversalConfig,
  FiveHoleProbeConfig,
  FiveHoleProbeChannelConfig,
  FiveHoleCalibFileInfo,
  FiveHoleTraversalTaskStatus,
  FiveHoleTraversalProgressEvent,
  FiveHoleTraversalRealtimeEvent,
  FiveHoleTraversalCompleteEvent,
  FiveHoleTraversalErrorEvent,
  FiveHoleTraversalDataPoint,
} from './fiveHoleTest/types'

// ==================== 默认配置构造 ====================

const PROBE_IDS = ['probe1', 'probe2', 'probe3'] as const

function defaultProbeChannels(): FiveHoleProbeChannelConfig[] {
  return [
    { name: 'P1', role: FiveHoleChannelRole.P1, deviceId: '', channel: 0, enabled: true },
    { name: 'P2', role: FiveHoleChannelRole.P2, deviceId: '', channel: 1, enabled: true },
    { name: 'P3', role: FiveHoleChannelRole.P3, deviceId: '', channel: 2, enabled: true },
    { name: 'P4', role: FiveHoleChannelRole.P4, deviceId: '', channel: 3, enabled: true },
    { name: 'P5', role: FiveHoleChannelRole.P5, deviceId: '', channel: 4, enabled: true },
  ]
}

function defaultProbe(probeId: string): FiveHoleProbeConfig {
  return {
    probeId,
    enabled: true,
    probeChannels: defaultProbeChannels(),
    motionAlpha: { controllerId: '', axis: AxisName.X },
    motionBeta: { controllerId: '', axis: AxisName.Y },
    calibFiles: [],
  }
}

function defaultConfig(): FiveHoleTraversalConfig {
  return {
    name: `五孔移位测试-${new Date().toLocaleDateString()}`,
    layout: {
      pattern: TraversalPattern.RECTANGLE,
      rectangle: {
        xMin: -20, xMax: 20, yMin: -20, yMax: 20,
        xSteps: [{ start: -20, end: 20, step: 5 }],
        ySteps: [{ start: -20, end: 20, step: 5 }],
      },
    },
    dwellTimeMs: 2000,
    samplesPerPoint: 10,
    sampleIntervalMs: 50,
    motionTimeoutMs: 30000,
    pAtmDeviceId: '',
    pAtmChannel: 16,
    tAtmDeviceId: '',
    tAtmChannel: 17,
    probes: PROBE_IDS.map(defaultProbe),
    savePath: '',
    saveFileName: '',
  }
}

// ==================== Store ====================

export const useFiveHoleTestStore = defineStore('fiveHoleTest', () => {
  // 状态
  const taskStatus = ref<FiveHoleTraversalTaskStatus | null>(null)
  const progress = ref<FiveHoleTraversalProgressEvent | null>(null)
  const realtime = ref<FiveHoleTraversalRealtimeEvent | null>(null)
  const isRunning = computed(() => taskStatus.value?.status === 'running' || taskStatus.value?.status === 'paused')
  const isPaused = computed(() => taskStatus.value?.status === 'paused')
  // 每探针独立的校准文件加载状态
  const calibLoadedMap = ref<Record<string, boolean>>({ probe1: false, probe2: false, probe3: false })
  const calibFilesMap = ref<Record<string, string[]>>({ probe1: [], probe2: [], probe3: [] })
  const lastError = ref<string>('')

  // 配置持久化 key（全局单实例，不按 probeID 区分）
  const configStorageKey = 'fiveHoleTestConfig'

  const config = ref<FiveHoleTraversalConfig>(defaultConfig())

  // 计算属性
  const statusText = computed(() => {
    if (!taskStatus.value) return '未启动'
    const map: Record<string, string> = {
      idle: '空闲', running: '运行中', paused: '已暂停',
      completed: '已完成', error: '错误',
    }
    return map[taskStatus.value.status] || taskStatus.value.status
  })

  const enabledProbes = computed(() => config.value.probes.filter(p => p.enabled))
  const allCalibLoaded = computed(() => enabledProbes.value.every(p => calibLoadedMap.value[p.probeId]))

  // ==================== API 调用 ====================

  async function selectCalibFiles(probeId: string) {
    try {
      const files = await SelectFiveHoleCalibFiles() as string[]
      if (!files || files.length === 0) return
      await LoadFiveHoleCalibFiles(probeId, files)
      const infos = await GetFiveHoleCalibInfo(probeId) as FiveHoleCalibFileInfo[]
      calibFilesMap.value[probeId] = files
      calibLoadedMap.value[probeId] = true
      // 同步到 config.probes[probeId].calibFiles
      const probe = config.value.probes.find(p => p.probeId === probeId)
      if (probe) {
        probe.calibFiles = files.map((f, i) => ({
          filePath: f,
          fileName: f.split(/[/\\]/).pop() || f,
          cMa: infos[i]?.cMa ?? 0,
        }))
      }
      await ensureDevicesAcquiring()
    } catch (e) {
      console.error(`selectCalibFiles(${probeId}) failed:`, e)
      lastError.value = `探针 ${probeId} 加载校准文件失败: ${e}`
    }
  }

  // 确保所有涉及到的采集设备都已连接并启动采集
  async function ensureDevicesAcquiring() {
    const deviceIds = new Set<string>()
    if (config.value.pAtmDeviceId) deviceIds.add(config.value.pAtmDeviceId)
    if (config.value.tAtmDeviceId) deviceIds.add(config.value.tAtmDeviceId)
    for (const probe of config.value.probes) {
      if (!probe.enabled) continue
      for (const ch of probe.probeChannels) {
        if (ch.enabled && ch.deviceId) deviceIds.add(ch.deviceId)
      }
    }

    try {
      const statuses = await GetDeviceStatusAll() as { id: string; status: string; acquiring: boolean }[]
      for (const deviceId of deviceIds) {
        const ds = statuses.find(s => s.id === deviceId)
        if (!ds) continue
        if (ds.status !== 'Connected') {
          try { await ConnectDevice(deviceId) } catch (e) {
            lastError.value = `自动连接设备 ${deviceId} 失败: ${e}`
            continue
          }
        }
        const updated = (await GetDeviceStatusAll() as { id: string; status: string; acquiring: boolean }[])
          .find(s => s.id === deviceId)
        if (updated && !updated.acquiring) {
          try { await StartAcquisition(deviceId) } catch (e) {
            lastError.value = `自动启动采集 ${deviceId} 失败: ${e}`
          }
        }
      }
    } catch (e) {
      console.error('ensureDevicesAcquiring failed:', e)
    }
  }

  async function startTest() {
    if (isRunning.value) return
    if (taskStatus.value?.status === 'running' || taskStatus.value?.status === 'paused') {
      lastError.value = '请先停止当前正在运行的测试'
      return
    }
    if (!allCalibLoaded.value) {
      lastError.value = '请为所有启用的探针加载校准文件'
      return
    }
    // 检查每探针位移机构连接状态
    const motionStore = useMotionStore()
    for (const probe of enabledProbes.value) {
      for (const axisMap of [probe.motionAlpha, probe.motionBeta]) {
        if (!axisMap.controllerId) continue
        const mc = motionStore.statuses.find(s => s.id === axisMap.controllerId)
        if (!mc || mc.status !== 'Connected') {
          lastError.value = `探针 ${probe.probeId} 的位移机构 ${axisMap.controllerId} 未连接`
          return
        }
      }
    }

    taskStatus.value = null
    lastError.value = ''
    realtime.value = null
    progress.value = null
    // 清理上一次测试的缓存数据点，避免导出 CSV 时返回旧数据
    completeProbeDataPoints.value = null

    try {
      await StartFiveHoleTraversal(config.value as any)
      await fetchStatus()
    } catch (e) {
      console.error('startTest failed:', e)
      lastError.value = `启动测试失败: ${e}`
      taskStatus.value = null
      realtime.value = null
      progress.value = null
    }
  }

  async function pauseTest() {
    try {
      await PauseFiveHoleTraversal()
      await fetchStatus()
    } catch (e) {
      console.error('pauseTest failed:', e)
    }
  }

  async function resumeTest() {
    try {
      await ResumeFiveHoleTraversal()
      await fetchStatus()
    } catch (e) {
      console.error('resumeTest failed:', e)
    }
  }

  async function stopTest() {
    try {
      await StopFiveHoleTraversal()
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
      if (retries >= maxRetries) {
        console.warn('stopTest: 超时未收到停止确认，强制清理状态')
        realtime.value = null
        progress.value = null
      }
    } catch (e) {
      console.error('stopTest failed:', e)
      realtime.value = null
      progress.value = null
    }
  }

  // ==================== 实时数据监控 ====================

  async function startRealtimeMonitor() {
    try {
      await StartFiveHoleRealtimeMonitor(config.value as any)
    } catch (e) {
      console.error('startRealtimeMonitor failed:', e)
    }
  }

  async function stopRealtimeMonitor() {
    try {
      await StopFiveHoleRealtimeMonitor()
    } catch (e) {
      console.error('stopRealtimeMonitor failed:', e)
    }
  }

  async function selectAndStartRealtimeRecording() {
    try {
      return await SelectAndStartFiveHoleRealtimeRecording()
    } catch (e) {
      console.error('selectAndStartRealtimeRecording failed:', e)
      lastError.value = `开始实时录制失败: ${e}`
      return ''
    }
  }

  async function stopRealtimeRecording() {
    try {
      await StopFiveHoleRealtimeRecording()
    } catch (e) {
      console.error('stopRealtimeRecording failed:', e)
    }
  }

  async function fetchStatus() {
    try {
      taskStatus.value = await GetFiveHoleTraversalStatus() as FiveHoleTraversalTaskStatus
    } catch (e) {
      console.warn('fetchStatus failed:', e)
    }
  }

  // ==================== 事件监听（五孔事件全局，不按 probeID 区分） ====================

  let listening = false

  function startListening() {
    if (listening) return
    try {
      EventsOn('five-hole:progress', (data: FiveHoleTraversalProgressEvent) => {
        progress.value = data
      })
      EventsOn('five-hole:realtime', (data: FiveHoleTraversalRealtimeEvent) => {
        realtime.value = data
      })
      EventsOn('five-hole:complete', async (data: FiveHoleTraversalCompleteEvent) => {
        // 缓存每探针数据点，供 CSV 导出使用
        setCompleteData(data.probeDataPoints)
        progress.value = null
        await fetchStatus()
      })
      EventsOn('five-hole:error', (data: FiveHoleTraversalErrorEvent) => {
        lastError.value = data.error
        if (data.isFatal) {
          progress.value = null
        }
        // 数据停滞等非致命错误会触发自动暂停，刷新状态以同步 UI
        fetchStatus()
      })
      listening = true
    } catch (e) {
      console.warn('startListening failed:', e)
    }
  }

  function stopListening() {
    if (!listening) return
    try {
      EventsOff('five-hole:progress')
      EventsOff('five-hole:realtime')
      EventsOff('five-hole:complete')
      EventsOff('five-hole:error')
    } catch (e) {
      console.warn('stopListening failed:', e)
    }
    listening = false
  }

  // ==================== 错误清除 ====================

  function clearError() {
    lastError.value = ''
  }

  // ==================== CSV 导出（按探针独立导出，含 β 列） ====================

  function exportProbeCSV(probeId: string) {
    // 从 taskStatus 中按 probeId 提取数据点
    // 注意：FiveHoleTraversalTaskStatus.probeStatuses 仅含实时状态，最终数据点
    // 通过 complete 事件返回（probeDataPoints map[probeId][]DataPoint）。
    // 完成事件触发时若需导出，应缓存 complete 事件数据。
    const probeData = completeProbeDataPoints.value?.[probeId] ?? []
    if (probeData.length === 0) return

    const BOM = '﻿'
    const headers = [
      '点号', '探针ID', 'X', 'Y',
      'P1', 'P2', 'P3', 'P4', 'P5', 'P∞', 'T∞',
      '总压Pt', '静压Ps', '马赫数Ma', '攻角Alpha', '侧滑角Beta', '速度V',
      '迭代次数', '采样数', '时间戳',
    ]
    const rows = probeData.map(p => [
      p.pointId, p.probeId, p.x.toFixed(4), p.y.toFixed(4),
      p.rawData.p1.toFixed(6), p.rawData.p2.toFixed(6), p.rawData.p3.toFixed(6),
      p.rawData.p4.toFixed(6), p.rawData.p5.toFixed(6),
      p.rawData.pAtm.toFixed(6), p.rawData.tAtm.toFixed(6),
      p.interpResult.ptProbe.toFixed(6), p.interpResult.psProbe.toFixed(6),
      p.interpResult.machProbe.toFixed(6), p.interpResult.alphaProbe.toFixed(4),
      p.interpResult.betaProbe.toFixed(4), p.interpResult.velocityProbe.toFixed(4),
      p.interpResult.iterationCount.toString(), p.sampleCount.toString(),
      p.timestamp.toString(),
    ].join(','))

    const csv = BOM + headers.join(',') + '\n' + rows.join('\n')
    const blob = new Blob([csv], { type: 'text/csv;charset=utf-8' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `five-hole-${probeId}-${new Date().toISOString().slice(0, 19).replace(/:/g, '-')}.csv`
    a.click()
    URL.revokeObjectURL(url)
  }

  // 缓存 complete 事件返回的每探针数据点，供 CSV 导出使用
  const completeProbeDataPoints = ref<Record<string, FiveHoleTraversalDataPoint[]> | null>(null)

  function setCompleteData(data: Record<string, FiveHoleTraversalDataPoint[]>) {
    completeProbeDataPoints.value = data
  }

  // ==================== 配置持久化 ====================

  function saveConfig() {
    try {
      localStorage.setItem(configStorageKey, JSON.stringify(config.value))
      SaveFiveHoleConfig(config.value as any).catch((e: unknown) => {
        console.error('保存五孔测试配置到后端失败:', e)
      })
    } catch (e) {
      console.error('保存五孔测试配置失败:', e)
    }
  }

  async function loadConfig() {
    try {
      // 优先从后端加载
      try {
        const loaded = await LoadFiveHoleConfig() as any
        if (loaded && loaded.probes && loaded.probes.length > 0) {
          config.value = loaded as FiveHoleTraversalConfig
          localStorage.setItem(configStorageKey, JSON.stringify(loaded))
        } else {
          loadConfigFromLocal()
        }
      } catch {
        loadConfigFromLocal()
      }
    } catch (e) {
      console.error('加载五孔测试配置失败:', e)
    }

    // 恢复每探针保存的校准文件
    for (const probe of config.value.probes) {
      const savedFiles = probe.calibFiles
      if (!savedFiles || savedFiles.length === 0) continue
      const filePaths = savedFiles.map(f => f.filePath).filter(Boolean)
      if (filePaths.length === 0) continue
      try {
        await LoadFiveHoleCalibFiles(probe.probeId, filePaths)
        const infos = await GetFiveHoleCalibInfo(probe.probeId) as FiveHoleCalibFileInfo[]
        calibFilesMap.value[probe.probeId] = filePaths
        calibLoadedMap.value[probe.probeId] = true
        probe.calibFiles = filePaths.map((f, i) => ({
          filePath: f,
          fileName: f.split(/[/\\]/).pop() || f,
          cMa: infos[i]?.cMa ?? 0,
        }))
      } catch (e) {
        console.error(`恢复探针 ${probe.probeId} 校准文件失败:`, e)
      }
    }
  }

  function loadConfigFromLocal() {
    try {
      const raw = localStorage.getItem(configStorageKey)
      if (!raw) return
      const data = JSON.parse(raw)
      if (data && data.probes && data.probes.length > 0) {
        config.value = data as FiveHoleTraversalConfig
      }
    } catch (e) {
      console.error('从localStorage加载配置失败:', e)
    }
  }

  return {
    // 状态
    taskStatus, progress, realtime, isRunning, isPaused, lastError,
    calibLoadedMap, calibFilesMap, allCalibLoaded,
    config, statusText, enabledProbes,
    completeProbeDataPoints,
    // 方法
    selectCalibFiles, ensureDevicesAcquiring,
    startTest, pauseTest, resumeTest, stopTest,
    fetchStatus, startListening, stopListening, clearError,
    exportProbeCSV, setCompleteData,
    saveConfig, loadConfig,
    startRealtimeMonitor, stopRealtimeMonitor,
    selectAndStartRealtimeRecording, stopRealtimeRecording,
  }
})
