/** E2E mock: CalibrationService（五孔探针校准服务） */
import { mockState, __logCall } from '../../state'
import { __emitEvent } from '../../wails-runtime'

function emitStatus(): void {
  __emitEvent('calibration:status-updated', mockState.calibrationStatus)
}

export function GetCalibrationStatus(): Promise<any> {
  __logCall('GetCalibrationStatus', [])
  return Promise.resolve(mockState.calibrationStatus)
}

export function StartCalibration(config: any): Promise<string> {
  __logCall('StartCalibration', [config])
  mockState.calibrationStatus = {
    status: 'running',
    currentIndex: 0,
    totalCount: (config?.points?.length) || 10,
  }
  emitStatus()
  return Promise.resolve('')
}

export function PauseCalibration(): Promise<void> {
  __logCall('PauseCalibration', [])
  mockState.calibrationStatus.status = 'paused'
  emitStatus()
  return Promise.resolve()
}

export function ResumeCalibration(): Promise<void> {
  __logCall('ResumeCalibration', [])
  mockState.calibrationStatus.status = 'running'
  emitStatus()
  return Promise.resolve()
}

export function StopCalibration(): Promise<void> {
  __logCall('StopCalibration', [])
  mockState.calibrationStatus = { status: 'idle', currentIndex: -1, totalCount: 0 }
  emitStatus()
  return Promise.resolve()
}
