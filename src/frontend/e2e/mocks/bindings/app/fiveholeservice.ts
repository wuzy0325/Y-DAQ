/**
 * E2E mock: FiveHoleService（五孔移位插值测试服务，单实例管理 1-3 探针）
 *
 * 注意：所有返回给前端的方法都必须返回深拷贝，模拟 Wails v3 的 JSON 序列化行为，
 * 否则前端 store 持有同一对象引用，Vue 响应式无法检测到后续 mutation。
 */
import { mockState, __logCall } from '../../state'

/** 深拷贝 status（含 probeStatuses/dataPoints 等嵌套字段） */
function cloneStatus(): any {
  const s = mockState.fiveHoleStatus
  if (!s) return null
  return JSON.parse(JSON.stringify(s))
}

export function OpenTestWindow(): Promise<string> {
  __logCall('OpenTestWindow', [])
  return Promise.resolve('five-hole-window-opened')
}

export function GetFiveHoleCalibInfo(probeID: string): Promise<any[]> {
  __logCall('GetFiveHoleCalibInfo', [probeID])
  // 返回 1 个校准文件信息，cMa=0.5
  return Promise.resolve([{ cMa: 0.5, validRange: { alphaMin: -30, alphaMax: 30, betaMin: -30, betaMax: 30, machMin: 0, machMax: 0.8 } }])
}

export function IsFiveHoleCalibLoaded(probeID: string): Promise<boolean> {
  __logCall('IsFiveHoleCalibLoaded', [probeID])
  return Promise.resolve(!!mockState.fiveHoleCalibLoaded[probeID])
}

export function SelectFiveHoleCalibFiles(): Promise<string[]> {
  __logCall('SelectFiveHoleCalibFiles', [])
  return Promise.resolve(['C:/data/probe.prb'])
}

export function LoadFiveHoleCalibFiles(probeID: string, filePaths: string[]): Promise<void> {
  __logCall('LoadFiveHoleCalibFiles', [probeID, filePaths])
  mockState.fiveHoleCalibLoaded[probeID] = true
  mockState.fiveHoleCalibFiles[probeID] = filePaths
  return Promise.resolve()
}

export function GetFiveHoleTraversalStatus(): Promise<any> {
  __logCall('GetFiveHoleTraversalStatus', [])
  // 返回深拷贝，避免前端 store 持有同一引用导致响应式失效
  return Promise.resolve(cloneStatus())
}

/**
 * 返回 null 让 store 走默认配置（defaultConfig），避免字段不匹配导致视图崩溃。
 * 真实场景中后端首次启动也无保存的配置，store 会用 defaultConfig 初始化。
 */
export function LoadFiveHoleConfig(): Promise<any> {
  __logCall('LoadFiveHoleConfig', [])
  return Promise.resolve(null)
}

export function SaveFiveHoleConfig(config: any): Promise<void> {
  __logCall('SaveFiveHoleConfig', [config])
  mockState.fiveHoleConfig = config
  return Promise.resolve()
}

export function StartFiveHoleTraversal(config: any): Promise<string> {
  __logCall('StartFiveHoleTraversal', [config])
  const total = config?.layout?.rectangle ? 9 : 0
  mockState.fiveHoleStatus = {
    status: 'running',
    currentPointIndex: 0,
    totalPoints: total,
    completedPoints: 0,
    progress: 0,
    probeStatuses: (config?.probes || []).map((p: any, i: number) => ({
      probeId: p.probeId,
      probeIndex: i,
      status: 'running',
      completedPoints: 0,
    })),
    errorMessage: '',
  }
  return Promise.resolve('')
}

export function PauseFiveHoleTraversal(): Promise<void> {
  __logCall('PauseFiveHoleTraversal', [])
  // 用新对象替换，确保前端 fetchStatus 拿到新引用
  mockState.fiveHoleStatus = { ...mockState.fiveHoleStatus, status: 'paused' }
  return Promise.resolve()
}

export function ResumeFiveHoleTraversal(): Promise<void> {
  __logCall('ResumeFiveHoleTraversal', [])
  mockState.fiveHoleStatus = { ...mockState.fiveHoleStatus, status: 'running' }
  return Promise.resolve()
}

export function StopFiveHoleTraversal(): Promise<void> {
  __logCall('StopFiveHoleTraversal', [])
  mockState.fiveHoleStatus = {
    status: 'idle',
    currentPointIndex: -1,
    totalPoints: 0,
    completedPoints: 0,
    progress: 0,
    probeStatuses: [],
    errorMessage: '',
  }
  return Promise.resolve()
}

export function StartFiveHoleRealtimeMonitor(config: any): Promise<void> {
  __logCall('StartFiveHoleRealtimeMonitor', [config])
  return Promise.resolve()
}

export function StopFiveHoleRealtimeMonitor(): Promise<void> {
  __logCall('StopFiveHoleRealtimeMonitor', [])
  return Promise.resolve()
}

export function IsFiveHoleRealtimeRecording(): Promise<boolean> {
  __logCall('IsFiveHoleRealtimeRecording', [])
  return Promise.resolve(mockState.fiveHoleRealtimeRecording)
}

export function SelectAndStartFiveHoleRealtimeRecording(): Promise<string> {
  __logCall('SelectAndStartFiveHoleRealtimeRecording', [])
  mockState.fiveHoleRealtimeRecording = true
  return Promise.resolve('C:/data/five-hole-realtime.csv')
}

export function StopFiveHoleRealtimeRecording(): Promise<void> {
  __logCall('StopFiveHoleRealtimeRecording', [])
  mockState.fiveHoleRealtimeRecording = false
  return Promise.resolve()
}
