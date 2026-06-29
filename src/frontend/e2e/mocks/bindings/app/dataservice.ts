/** E2E mock: DataService（数据录制与导出服务） */
import { mockState, __logCall } from '../../state'

export function GetDataDir(): Promise<string> {
  __logCall('GetDataDir', [])
  return Promise.resolve(mockState.dataDir)
}

export function GetPublishRate(): Promise<number> {
  __logCall('GetPublishRate', [])
  return Promise.resolve(mockState.publishRate)
}

export function SetPublishRate(hz: number): Promise<void> {
  __logCall('SetPublishRate', [hz])
  mockState.publishRate = hz
  return Promise.resolve()
}

export function IsRecording(): Promise<boolean> {
  __logCall('IsRecording', [])
  return Promise.resolve(mockState.recording)
}

export function StartRecording(): Promise<void> {
  __logCall('StartRecording', [])
  mockState.recording = true
  return Promise.resolve()
}

export function StopRecording(): Promise<void> {
  __logCall('StopRecording', [])
  mockState.recording = false
  return Promise.resolve()
}

export function ListRecordingFiles(): Promise<string[]> {
  __logCall('ListRecordingFiles', [])
  return Promise.resolve([...mockState.recordingFiles])
}

export function ReadRecordingFile(fileName: string): Promise<string> {
  __logCall('ReadRecordingFile', [fileName])
  return Promise.resolve('timestamp,ch1,ch2\n0,1.0,2.0\n1,1.1,2.1\n')
}

export function LoadCSVFile(): Promise<string> {
  __logCall('LoadCSVFile', [])
  return Promise.resolve('C:/data/loaded.csv')
}

export function ExportCalibrationPDF(): Promise<void> {
  __logCall('ExportCalibrationPDF', [])
  return Promise.resolve()
}
