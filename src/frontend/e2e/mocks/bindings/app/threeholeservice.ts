/**
 * E2E mock: ThreeHoleService（三孔移位插值测试服务）
 * 维护 mockState.threeHole* 字段，状态变更后推送 three-hole:* 事件。
 *
 * 注意：所有返回给前端的方法都必须返回深拷贝，模拟 Wails v3 的 JSON 序列化行为，
 * 否则前端 store 持有同一对象引用，Vue 响应式无法检测到后续 mutation。
 */
import { mockState, __logCall } from '../../state'
import { __emitEvent } from '../../wails-runtime'

function emitStatus(probeID: string): void {
  __emitEvent(`three-hole:status-updated-${probeID}`, { ...mockState.threeHoleStatus[probeID] })
}

function ensureStatusEntry(probeID: string): void {
  if (!mockState.threeHoleStatus[probeID]) {
    mockState.threeHoleStatus[probeID] = {
      status: 'idle',
      currentPointIndex: -1,
      totalPoints: 0,
      completedPoints: 0,
      currentProbeIndex: 0,
      errorMessage: '',
    }
  }
}

/** 深拷贝 status（含 dataPoints 等嵌套字段） */
function cloneStatus(probeID: string): any {
  const s = mockState.threeHoleStatus[probeID]
  if (!s) return null
  return JSON.parse(JSON.stringify(s))
}

export function OpenTestWindow(probeID: string): Promise<string> {
  __logCall('OpenTestWindow', [probeID])
  return Promise.resolve(`test-window-opened-${probeID}`)
}

export function GetThreeHoleCalibInfo(probeID: string): Promise<any[]> {
  __logCall('GetThreeHoleCalibInfo', [probeID])
  return Promise.resolve([])
}

export function IsThreeHoleCalibLoaded(probeID: string): Promise<boolean> {
  __logCall('IsThreeHoleCalibLoaded', [probeID])
  return Promise.resolve(!!mockState.threeHoleCalibLoaded[probeID])
}

export function SelectThreeHoleCalibFiles(): Promise<string[]> {
  __logCall('SelectThreeHoleCalibFiles', [])
  // 模拟用户选择了 1 个校准文件
  const files = ['C:/data/probe1.cal']
  return Promise.resolve(files)
}

export function LoadThreeHoleCalibFiles(probeID: string, filePaths: string[]): Promise<void> {
  __logCall('LoadThreeHoleCalibFiles', [probeID, filePaths])
  mockState.threeHoleCalibLoaded[probeID] = true
  mockState.threeHoleCalibFiles[probeID] = filePaths
  __emitEvent(`three-hole:calib-loaded-${probeID}`, { probeID, loaded: true, files: filePaths })
  return Promise.resolve()
}

export function GetThreeHoleTraversalStatus(probeID: string): Promise<any> {
  __logCall('GetThreeHoleTraversalStatus', [probeID])
  ensureStatusEntry(probeID)
  // 返回深拷贝，避免前端 store 持有同一引用导致响应式失效
  return Promise.resolve(cloneStatus(probeID))
}

export function StartThreeHoleTraversal(probeID: string, config: any): Promise<string> {
  __logCall('StartThreeHoleTraversal', [probeID, config])
  ensureStatusEntry(probeID)
  mockState.threeHoleStatus[probeID] = {
    status: 'running',
    currentPointIndex: 0,
    totalPoints: (config?.points?.length) || 9,
    completedPoints: 0,
    currentProbeIndex: 0,
    errorMessage: '',
  }
  emitStatus(probeID)
  return Promise.resolve('')
}

export function PauseThreeHoleTraversal(probeID: string): Promise<void> {
  __logCall('PauseThreeHoleTraversal', [probeID])
  ensureStatusEntry(probeID)
  // 用新对象替换，确保前端 fetchStatus 拿到新引用
  mockState.threeHoleStatus[probeID] = { ...mockState.threeHoleStatus[probeID], status: 'paused' }
  emitStatus(probeID)
  return Promise.resolve()
}

export function ResumeThreeHoleTraversal(probeID: string): Promise<void> {
  __logCall('ResumeThreeHoleTraversal', [probeID])
  ensureStatusEntry(probeID)
  mockState.threeHoleStatus[probeID] = { ...mockState.threeHoleStatus[probeID], status: 'running' }
  emitStatus(probeID)
  return Promise.resolve()
}

export function StopThreeHoleTraversal(probeID: string): Promise<void> {
  __logCall('StopThreeHoleTraversal', [probeID])
  ensureStatusEntry(probeID)
  mockState.threeHoleStatus[probeID] = { ...mockState.threeHoleStatus[probeID], status: 'idle', currentPointIndex: -1 }
  emitStatus(probeID)
  return Promise.resolve()
}

export function StartThreeHoleRealtimeMonitor(probeID: string, config: any): Promise<void> {
  __logCall('StartThreeHoleRealtimeMonitor', [probeID, config])
  return Promise.resolve()
}

export function StopThreeHoleRealtimeMonitor(probeID: string): Promise<void> {
  __logCall('StopThreeHoleRealtimeMonitor', [probeID])
  return Promise.resolve()
}

export function IsThreeHoleRealtimeRecording(probeID: string): Promise<boolean> {
  __logCall('IsThreeHoleRealtimeRecording', [probeID])
  return Promise.resolve(!!mockState.threeHoleRealtimeRecording[probeID])
}

export function SelectAndStartThreeHoleRealtimeRecording(probeID: string): Promise<string> {
  __logCall('SelectAndStartThreeHoleRealtimeRecording', [probeID])
  mockState.threeHoleRealtimeRecording[probeID] = true
  const path = `C:/data/realtime-${probeID}.csv`
  return Promise.resolve(path)
}

export function StopThreeHoleRealtimeRecording(probeID: string): Promise<void> {
  __logCall('StopThreeHoleRealtimeRecording', [probeID])
  mockState.threeHoleRealtimeRecording[probeID] = false
  return Promise.resolve()
}
