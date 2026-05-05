import { ref, computed } from 'vue'
import { defineStore } from 'pinia'
import { ThreeHoleService, ConfigService, DeviceService } from '../../bindings/yx-daq/internal/app'
import { Events } from '@wailsio/runtime'
import { ThreeHoleChannelRole, TraversalPattern } from '../api/enums'
import { useMotionStore } from './motion'
import type {
  ThreeHoleRawData,
  ThreeHoleInterpolationResult,
  ThreeHoleTraversalDataPoint,
  ThreeHoleTraversalTaskStatus,
  ThreeHoleTraversalProgressEvent,
  ThreeHoleTraversalRealtimeEvent,
  ThreeHoleTraversalCompleteEvent,
  ThreeHoleTraversalErrorEvent,
  ThreeHoleTraversalConfig,
} from './threeHoleTest/types'

// ==================== Store (Pinia, probe parameter via factory) ====================

function createThreeHoleStore(probeID: string) {
  return defineStore(`threeHoleTest-${probeID}`, () => {
    const prefix = `three-hole:${probeID}:`

    const taskStatus = ref<ThreeHoleTraversalTaskStatus | null>(null)
    const progress = ref<ThreeHoleTraversalProgressEvent | null>(null)
    const realtime = ref<ThreeHoleTraversalRealtimeEvent | null>(null)
    const isRunning = computed(() => taskStatus.value?.status === 'running' || taskStatus.value?.status === 'paused')
    const isPaused = computed(() => taskStatus.value?.status === 'paused')
    const calibLoaded = ref(false)
    const calibFiles = ref<string[]>([])
    const lastError = ref<string>('')

    const config = ref<ThreeHoleTraversalConfig>({
      name: `三孔移位测试-${probeID}`,
      deviceId: '',
      motionControllerId: '',
      layout: {
        pattern: TraversalPattern.RECTANGLE,
        rectangle: { xMin: -20, xMax: 20, yMin: -20, yMax: 20, xSteps: [{ start: -20, end: 20, step: 5 }], ySteps: [{ start: -20, end: 20, step: 5 }] },
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

    const statusText = computed(() => {
      if (!taskStatus.value) return '未启动'
      const map: Record<string, string> = { idle: '空闲', running: '运行中', paused: '已暂停', completed: '已完成', error: '错误' }
      return map[taskStatus.value.status] || taskStatus.value.status
    })

    const hasResults = computed(() => (taskStatus.value?.dataPoints?.length ?? 0) > 0)

    async function selectCalibFiles() {
      try {
        const files = await ThreeHoleService.SelectThreeHoleCalibFiles() as string[]
        if (files && files.length > 0) {
          calibFiles.value = files
          await ThreeHoleService.LoadThreeHoleCalibFiles(probeID, files)
          const infos = await ThreeHoleService.GetThreeHoleCalibInfo(probeID) as { cMa: number }[]
          calibLoaded.value = true
          config.value.calibFiles = files.map((f, i) => ({ filePath: f, fileName: f.split(/[/\\]/).pop() || f, cMa: infos[i]?.cMa ?? 0 }))
          await ensureDeviceAcquiring()
        }
      } catch (e: any) {
        console.error('selectCalibFiles failed:', e)
        lastError.value = `加载校准文件失败: ${e}`
      }
    }

    async function ensureDeviceAcquiring() {
      const deviceId = config.value.deviceId
      if (!deviceId) return
      try {
        const statuses = await DeviceService.GetDeviceStatusAll() as { id: string; status: string; acquiring: boolean }[]
        const ds = statuses.find((s: { id: string }) => s.id === deviceId)
        if (!ds) return
        if (ds.status !== 'Connected') {
          try { await DeviceService.ConnectDevice(deviceId) } catch (e) { lastError.value = `自动连接设备失败: ${e}`; return }
        }
        const updated = (await DeviceService.GetDeviceStatusAll() as { id: string; acquiring: boolean }[]).find((s: { id: string }) => s.id === deviceId)
        if (updated && !updated.acquiring) {
          try { await DeviceService.StartAcquisition(deviceId) } catch (e) { lastError.value = `自动启动采集失败: ${e}` }
        }
      } catch (e) { console.error('ensureDeviceAcquiring failed:', e) }
    }

    async function startTest() {
      if (isRunning.value) return
      if (!calibLoaded.value) { lastError.value = '请先加载校准文件'; return }
      const mcId = config.value.motionControllerId
      if (mcId) {
        const motionStore = useMotionStore()
        const mcStatus = motionStore.statuses.find(s => s.id === mcId)
        if (!mcStatus || mcStatus.status !== 'Connected') { lastError.value = '运动控制器未连接'; return }
      }
      taskStatus.value = null; lastError.value = ''; realtime.value = null; progress.value = null
      try {
        await ThreeHoleService.StartThreeHoleTraversal(probeID, config.value as any)
        await fetchStatus()
      } catch (e: any) {
        console.error('startTest failed:', e)
        lastError.value = `启动测试失败: ${e}`; taskStatus.value = null; realtime.value = null; progress.value = null
      }
    }

    async function pauseTest() { try { await ThreeHoleService.PauseThreeHoleTraversal(probeID); await fetchStatus() } catch (e) { console.error(e) } }
    async function resumeTest() { try { await ThreeHoleService.ResumeThreeHoleTraversal(probeID); await fetchStatus() } catch (e) { console.error(e) } }

    async function stopTest() {
      try {
        await ThreeHoleService.StopThreeHoleTraversal(probeID)
        for (let i = 0; i < 10; i++) {
          await fetchStatus()
          if (taskStatus.value?.status === 'idle' || taskStatus.value?.status === 'completed' || taskStatus.value?.status === 'error') {
            realtime.value = null; progress.value = null; break
          }
          await new Promise(r => setTimeout(r, 200))
        }
      } catch (e) { console.error(e); realtime.value = null; progress.value = null }
    }

    async function startRealtimeMonitor() { try { await ThreeHoleService.StartThreeHoleRealtimeMonitor(probeID, config.value as any) } catch (e) { console.error(e) } }
    async function stopRealtimeMonitor() { try { await ThreeHoleService.StopThreeHoleRealtimeMonitor(probeID) } catch (e) { console.error(e) } }

    async function fetchStatus() {
      try { taskStatus.value = await ThreeHoleService.GetThreeHoleTraversalStatus(probeID) as ThreeHoleTraversalTaskStatus } catch (e) { console.warn(e) }
    }

    function startListening() {
      try {
        Events.On(prefix + 'progress', (ev: { data: ThreeHoleTraversalProgressEvent }) => { if (isRunning.value) progress.value = ev.data })
        Events.On(prefix + 'realtime', (ev: { data: ThreeHoleTraversalRealtimeEvent }) => { realtime.value = ev.data })
        Events.On(prefix + 'complete', async () => { progress.value = null; await fetchStatus() })
        Events.On(prefix + 'error', (ev: { data: ThreeHoleTraversalErrorEvent }) => { lastError.value = ev.data.error; if (ev.data.isFatal) progress.value = null })
      } catch (e) { console.warn('startListening failed:', e) }
    }

    function stopListening() {
      try { Events.Off(prefix + 'progress'); Events.Off(prefix + 'realtime'); Events.Off(prefix + 'complete'); Events.Off(prefix + 'error') } catch (e) { console.warn(e) }
    }

    function clearError() { lastError.value = '' }

    function exportCSV() {
      const dataPoints = taskStatus.value?.dataPoints ?? []
      if (dataPoints.length === 0) return
      const BOM = '\uFEFF'
      const headers = ['点号', 'X', 'Y', 'P1', 'P2', 'P3', 'P∞', 'T∞', '总压Pt', '静压Ps', '马赫数Ma', '攻角Alpha', '迭代次数', '采样数', '时间戳']
      const rows = dataPoints.map(p => [
        p.pointId, p.x.toFixed(4), p.y.toFixed(4), p.rawData.p1.toFixed(6), p.rawData.p2.toFixed(6), p.rawData.p3.toFixed(6),
        p.rawData.pAtm.toFixed(6), p.rawData.tAtm.toFixed(6), p.interpResult.ptProbe.toFixed(6), p.interpResult.psProbe.toFixed(6),
        p.interpResult.machProbe.toFixed(6), p.interpResult.alphaProbe.toFixed(4), p.interpResult.iterationCount.toString(), p.sampleCount.toString(), p.timestamp.toString(),
      ].join(','))
      const blob = new Blob([BOM + headers.join(',') + '\n' + rows.join('\n')], { type: 'text/csv;charset=utf-8' })
      const url = URL.createObjectURL(blob)
      const a = document.createElement('a'); a.href = url; a.download = `three-hole-${probeID}-${new Date().toISOString().slice(0, 19).replace(/:/g, '-')}.csv`
      a.click(); URL.revokeObjectURL(url)
    }

    const configKey = probeID === 'probe2' ? 'threeHoleTestConfig_probe2' : 'threeHoleTestConfig'

    function saveConfig() {
      try {
        localStorage.setItem(configKey, JSON.stringify(config.value))
        if (probeID === 'probe2') ConfigService.SaveThreeHoleProbe2Config(config.value as any).catch(() => {})
        else ConfigService.SaveThreeHoleProbe1Config(config.value as any).catch(() => {})
      } catch (e) { console.error(e) }
    }

    async function loadConfig() {
      try {
        let loaded: any
        if (probeID === 'probe2') loaded = await ConfigService.LoadThreeHoleProbe2Config() as any
        else loaded = await ConfigService.LoadThreeHoleProbe1Config() as any
        if (loaded?.probeChannels?.length > 0) { config.value = loaded; localStorage.setItem(configKey, JSON.stringify(loaded)) }
        else loadConfigFromLocal()
      } catch { loadConfigFromLocal() }
      const savedCalibFiles = config.value.calibFiles
      if (savedCalibFiles?.length > 0) {
        const filePaths = savedCalibFiles.map(f => f.filePath).filter(Boolean)
        if (filePaths.length > 0) {
          try { await ThreeHoleService.LoadThreeHoleCalibFiles(probeID, filePaths); calibFiles.value = filePaths; calibLoaded.value = true } catch (e) { console.error(e) }
        }
      }
    }

    function loadConfigFromLocal() {
      try {
        const raw = localStorage.getItem(configKey)
        if (raw) { const data = JSON.parse(raw); if (data?.probeChannels?.length > 0) config.value = data }
      } catch (e) { console.error(e) }
    }

    return {
      taskStatus, progress, realtime, isRunning, isPaused, calibLoaded, calibFiles, lastError,
      config, statusText, hasResults, probeID,
      selectCalibFiles, startTest, pauseTest, resumeTest, stopTest,
      ensureDeviceAcquiring, fetchStatus, startListening, stopListening, clearError, exportCSV,
      saveConfig, loadConfig, startRealtimeMonitor, stopRealtimeMonitor,
    }
  })
}

// 预定义两个探针的 store
export const useThreeHoleTestStoreProbe1 = createThreeHoleStore('probe1')
export const useThreeHoleTestStoreProbe2 = createThreeHoleStore('probe2')

// 根据 probeID 获取对应 store
export function useThreeHoleTestStore(probeID: string) {
  if (probeID === 'probe2') return useThreeHoleTestStoreProbe2()
  return useThreeHoleTestStoreProbe1()
}
