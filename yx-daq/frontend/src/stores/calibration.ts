import { ref, computed } from 'vue'
import { defineStore } from 'pinia'
import { CalibrationService } from '../../bindings/yx-daq/internal/app'
import { Events } from '@wailsio/runtime'

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
  const isRunning = computed(() => taskStatus.value?.status === 'running' || taskStatus.value?.status === 'paused')

  async function startCalibration(config: any) {
    try {
      await CalibrationService.StartCalibration(config)
      await fetchStatus()
    } catch (e) {
      console.error('startCalibration failed:', e)
    }
  }

  async function pauseCalibration() {
    try {
      await CalibrationService.PauseCalibration()
    } catch (e) {
      console.error('pauseCalibration failed:', e)
    }
  }

  async function resumeCalibration() {
    try {
      await CalibrationService.ResumeCalibration()
    } catch (e) {
      console.error('resumeCalibration failed:', e)
    }
  }

  async function stopCalibration() {
    try {
      await CalibrationService.StopCalibration()
      await fetchStatus()
    } catch (e) {
      console.error('stopCalibration failed:', e)
    }
  }

  async function fetchStatus() {
    try {
      taskStatus.value = await CalibrationService.GetCalibrationStatus() as CalibrationTaskStatus
    } catch (e) {
      console.warn('fetchCalibStatus failed:', e)
    }
  }

  function startListening() {
    try {
      Events.On('calibration:progress', (event: { data: CalibrationProgressEvent }) => {
        progress.value = event.data
      })
      Events.On('calibration:realtime', (event: { data: CalibrationRealtimeEvent }) => {
        realtime.value = event.data
      })
      Events.On('calibration:complete', () => {
        fetchStatus()
      })
    } catch (e) {
      console.warn('calibration startListening failed:', e)
    }
  }

  function stopListening() {
    try {
      Events.Off('calibration:progress')
      Events.Off('calibration:realtime')
      Events.Off('calibration:complete')
    } catch (e) {
      console.warn('calibration stopListening failed:', e)
    }
  }

  return {
    taskStatus, progress, realtime, isRunning,
    startCalibration, pauseCalibration, resumeCalibration, stopCalibration,
    fetchStatus, startListening, stopListening,
  }
})
