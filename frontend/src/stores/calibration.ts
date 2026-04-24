import { defineStore } from 'pinia'
import { ref } from 'vue'

interface FiveHoleRawData {
  p1: number; p2: number; p3: number; p4: number; p5: number
  pAtm: number; tAtm: number; pTotal?: number
}

interface FiveHoleCoefficients {
  Kalpha: number; Kbeta: number; CPT: number; CPS: number
}

interface CalibrationDataPoint {
  pointId: string; alpha: number; beta: number
  rawData: FiveHoleRawData; coefficients: FiveHoleCoefficients
  sampleCount: number; stdDev: number
}

interface CalibrationTaskStatus {
  taskId: string; status: string
  totalPoints: number; completedPoints: number; progress: number
  currentPoint: { id: string; alpha: number; beta: number } | null
  dataPoints: CalibrationDataPoint[]
  lastError: string
}

interface CalibrationProgressEvent {
  taskId: string; totalPoints: number; completedPoints: number
  progress: number; currentAlpha: number; currentBeta: number
}

interface CalibrationRealtimeEvent {
  taskId: string; pointId: string
  rawData: FiveHoleRawData; coefficients: FiveHoleCoefficients
}

export const useCalibrationStore = defineStore('calibration', () => {
  const taskStatus = ref<CalibrationTaskStatus | null>(null)
  const progress = ref<CalibrationProgressEvent | null>(null)
  const realtime = ref<CalibrationRealtimeEvent | null>(null)
  const isRunning = ref(false)

  async function startCalibration(config: any) {
    try {
      const { StartCalibration } = await import('../../wailsjs/go/main/App')
      await StartCalibration(config)
      isRunning.value = true
    } catch (e) {
      console.error('startCalibration failed:', e)
    }
  }

  async function pauseCalibration() {
    try {
      const { PauseCalibration } = await import('../../wailsjs/go/main/App')
      await PauseCalibration()
    } catch (e) {
      console.error('pauseCalibration failed:', e)
    }
  }

  async function resumeCalibration() {
    try {
      const { ResumeCalibration } = await import('../../wailsjs/go/main/App')
      await ResumeCalibration()
    } catch (e) {
      console.error('resumeCalibration failed:', e)
    }
  }

  async function stopCalibration() {
    try {
      const { StopCalibration } = await import('../../wailsjs/go/main/App')
      await StopCalibration()
      isRunning.value = false
    } catch (e) {
      console.error('stopCalibration failed:', e)
    }
  }

  async function fetchStatus() {
    try {
      const { GetCalibrationStatus } = await import('../../wailsjs/go/main/App')
      taskStatus.value = await GetCalibrationStatus() as CalibrationTaskStatus
    } catch (e) {
      console.warn('fetchCalibStatus failed:', e)
    }
  }

  function startListening() {
    try {
      import('../../wailsjs/runtime/runtime').then(({ EventsOn }) => {
        EventsOn('calibration:progress', (data: CalibrationProgressEvent) => {
          progress.value = data
          isRunning.value = true
        })
        EventsOn('calibration:realtime', (data: CalibrationRealtimeEvent) => {
          realtime.value = data
        })
        EventsOn('calibration:complete', () => {
          isRunning.value = false
          fetchStatus()
        })
      })
    } catch (e) {
      console.warn('calibration startListening failed:', e)
    }
  }

  return {
    taskStatus, progress, realtime, isRunning,
    startCalibration, pauseCalibration, resumeCalibration, stopCalibration,
    fetchStatus, startListening,
  }
})
