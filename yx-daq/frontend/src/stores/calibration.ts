import { ref, computed } from 'vue'
import { defineStore } from 'pinia'
import {
  StartCalibration, PauseCalibration, ResumeCalibration,
  StopCalibration, GetCalibrationStatus,
} from '../../wailsjs/go/main/App'
import { EventsOn, EventsOff } from '../../wailsjs/runtime/runtime'

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
      await StartCalibration(config)
      await fetchStatus()
    } catch (e) {
      console.error('startCalibration failed:', e)
    }
  }

  async function pauseCalibration() {
    try {
      await PauseCalibration()
    } catch (e) {
      console.error('pauseCalibration failed:', e)
    }
  }

  async function resumeCalibration() {
    try {
      await ResumeCalibration()
    } catch (e) {
      console.error('resumeCalibration failed:', e)
    }
  }

  async function stopCalibration() {
    try {
      await StopCalibration()
      await fetchStatus()
    } catch (e) {
      console.error('stopCalibration failed:', e)
    }
  }

  async function fetchStatus() {
    try {
      taskStatus.value = await GetCalibrationStatus() as CalibrationTaskStatus
    } catch (e) {
      console.warn('fetchCalibStatus failed:', e)
    }
  }

  function startListening() {
    try {
      EventsOn('calibration:progress', (data: CalibrationProgressEvent) => {
        progress.value = data
      })
      EventsOn('calibration:realtime', (data: CalibrationRealtimeEvent) => {
        realtime.value = data
      })
      EventsOn('calibration:complete', () => {
        fetchStatus()
      })
    } catch (e) {
      console.warn('calibration startListening failed:', e)
    }
  }

  function stopListening() {
    try {
      EventsOff('calibration:progress')
      EventsOff('calibration:realtime')
      EventsOff('calibration:complete')
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
