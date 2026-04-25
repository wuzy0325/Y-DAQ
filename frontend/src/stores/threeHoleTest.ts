import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import {
  ThreeHoleChannelRole,
  TraversalPattern,
  type ThreeHoleChannelRoleValue,
  type TraversalPatternValue,
} from '../api/enums'

// ==================== 类型定义 ====================

interface ThreeHoleRawData {
  p1: number; p2: number; p3: number; pAtm: number; tAtm: number
}

interface ThreeHoleInterpolationResult {
  ptProbe: number; psProbe: number; machProbe: number; alphaProbe: number
  iterationCount: number; valid: boolean
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
  taskId: string; error: string
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
  axis: string; scale: number; offset: number
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
  motionX: MotionAxisMapping
  motionY: MotionAxisMapping
  calibFiles: ThreeHoleCalibFileInfo[]
  dwellTimeMs: number
  samplesPerPoint: number
  savePath: string
  saveFileName: string
}

// ==================== Store ====================

export const useThreeHoleTestStore = defineStore('threeHoleTest', () => {
  // 状态
  const taskStatus = ref<ThreeHoleTraversalTaskStatus | null>(null)
  const progress = ref<ThreeHoleTraversalProgressEvent | null>(null)
  const realtime = ref<ThreeHoleTraversalRealtimeEvent | null>(null)
  const isRunning = ref(false)
  const calibLoaded = ref(false)
  const calibFiles = ref<string[]>([])
  const lastError = ref<string>('')

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
    motionX: { axis: 'X', scale: 1, offset: 0 },
    motionY: { axis: 'Y', scale: 1, offset: 0 },
    calibFiles: [],
    dwellTimeMs: 2000,
    samplesPerPoint: 10,
    savePath: '',
    saveFileName: `ThreeHoleTraversal-${new Date().toISOString().slice(0, 10)}.csv`,
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
      const { SelectThreeHoleCalibFiles } = await import('../../wailsjs/go/main/App')
      const files = await SelectThreeHoleCalibFiles() as string[]
      if (files && files.length > 0) {
        calibFiles.value = files
        // 加载校准文件
        const { LoadThreeHoleCalibFiles } = await import('../../wailsjs/go/main/App')
        await LoadThreeHoleCalibFiles(files)
        calibLoaded.value = true
        // 更新配置中的校准文件信息
        config.value.calibFiles = files.map(f => ({
          filePath: f,
          fileName: f.split(/[/\\]/).pop() || f,
          cMa: 0, // CMa 在后端解析后可知
        }))
      }
    } catch (e) {
      console.error('selectCalibFiles failed:', e)
      lastError.value = `加载校准文件失败: ${e}`
    }
  }

  async function startTest() {
    try {
      // 检查校准文件
      if (!calibLoaded.value) {
        lastError.value = '请先加载校准文件'
        return
      }

      const { StartThreeHoleTraversal } = await import('../../wailsjs/go/main/App')
      await StartThreeHoleTraversal(config.value as any)
      isRunning.value = true
      lastError.value = ''
    } catch (e) {
      console.error('startTest failed:', e)
      lastError.value = `启动测试失败: ${e}`
    }
  }

  async function pauseTest() {
    try {
      const { PauseThreeHoleTraversal } = await import('../../wailsjs/go/main/App')
      await PauseThreeHoleTraversal()
    } catch (e) {
      console.error('pauseTest failed:', e)
    }
  }

  async function resumeTest() {
    try {
      const { ResumeThreeHoleTraversal } = await import('../../wailsjs/go/main/App')
      await ResumeThreeHoleTraversal()
    } catch (e) {
      console.error('resumeTest failed:', e)
    }
  }

  async function stopTest() {
    try {
      const { StopThreeHoleTraversal } = await import('../../wailsjs/go/main/App')
      await StopThreeHoleTraversal()
      isRunning.value = false
    } catch (e) {
      console.error('stopTest failed:', e)
    }
  }

  async function fetchStatus() {
    try {
      const { GetThreeHoleTraversalStatus } = await import('../../wailsjs/go/main/App')
      taskStatus.value = await GetThreeHoleTraversalStatus() as ThreeHoleTraversalTaskStatus
    } catch (e) {
      console.warn('fetchStatus failed:', e)
    }
  }

  // ==================== 事件监听 ====================

  function startListening() {
    try {
      import('../../wailsjs/runtime/runtime').then(({ EventsOn }) => {
        EventsOn('three-hole:progress', (data: ThreeHoleTraversalProgressEvent) => {
          progress.value = data
          isRunning.value = true
        })
        EventsOn('three-hole:realtime', (data: ThreeHoleTraversalRealtimeEvent) => {
          realtime.value = data
        })
        EventsOn('three-hole:complete', (data: ThreeHoleTraversalCompleteEvent) => {
          isRunning.value = false
          fetchStatus()
        })
        EventsOn('three-hole:error', (data: ThreeHoleTraversalErrorEvent) => {
          lastError.value = data.error
          isRunning.value = false
        })
      })
    } catch (e) {
      console.warn('startListening failed:', e)
    }
  }

  // ==================== CSV 导出 ====================

  function exportCSV() {
    const dataPoints = taskStatus.value?.dataPoints ?? []
    if (dataPoints.length === 0) return

    const BOM = '\uFEFF'
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

  return {
    // 状态
    taskStatus, progress, realtime, isRunning, calibLoaded, calibFiles, lastError,
    config, statusText, hasResults,
    // 方法
    selectCalibFiles, startTest, pauseTest, resumeTest, stopTest,
    fetchStatus, startListening, exportCSV,
  }
})
