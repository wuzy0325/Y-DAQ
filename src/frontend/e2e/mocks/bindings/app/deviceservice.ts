/**
 * E2E mock: DeviceService（设备管理服务）
 *
 * 维护 mockState.deviceProfiles / deviceStatuses，方法签名与真实 bindings 一致。
 * 状态变更后通过 __emitEvent 推送 device:status-updated，使前端 store 监听器更新 UI。
 */
import { mockState, __logCall, __resetMockState as _reset } from '../../state'
import { __emitEvent } from '../../wails-runtime'

function emitStatusUpdate(): void {
  // 传递副本以模拟 Wails v3 的 JSON 序列化行为，避免前端 store 持有同一引用导致响应式失效
  __emitEvent('device:status-updated', mockState.deviceStatuses.map(s => ({ ...s })))
}

function ensureStatus(id: string, name: string, type: string): void {
  let s = mockState.deviceStatuses.find(x => x.id === id)
  if (!s) {
    s = { id, name, type, status: 'Disconnected', acquiring: false, lastError: '' }
    mockState.deviceStatuses.push(s)
  }
}

export function AddDeviceProfile(profile: any): Promise<void> {
  __logCall('AddDeviceProfile', [profile])
  mockState.deviceProfiles.push(profile)
  ensureStatus(profile.id, profile.name, profile.type)
  emitStatusUpdate()
  return Promise.resolve()
}

export function UpdateDeviceProfile(profile: any): Promise<void> {
  __logCall('UpdateDeviceProfile', [profile])
  const idx = mockState.deviceProfiles.findIndex(p => p.id === profile.id)
  if (idx >= 0) mockState.deviceProfiles[idx] = profile
  const s = mockState.deviceStatuses.find(x => x.id === profile.id)
  if (s) {
    s.name = profile.name
    s.type = profile.type
  }
  emitStatusUpdate()
  return Promise.resolve()
}

export function RemoveDeviceProfile(id: string): Promise<void> {
  __logCall('RemoveDeviceProfile', [id])
  mockState.deviceProfiles = mockState.deviceProfiles.filter(p => p.id !== id)
  mockState.deviceStatuses = mockState.deviceStatuses.filter(s => s.id !== id)
  emitStatusUpdate()
  return Promise.resolve()
}

export function GetDeviceProfiles(): Promise<any[]> {
  __logCall('GetDeviceProfiles', [])
  return Promise.resolve(mockState.deviceProfiles.map(p => ({ ...p })))
}

export function GetDeviceStatusAll(): Promise<any[]> {
  __logCall('GetDeviceStatusAll', [])
  return Promise.resolve(mockState.deviceStatuses.map(s => ({ ...s })))
}

export function ConnectDevice(id: string): Promise<void> {
  __logCall('ConnectDevice', [id])
  const s = mockState.deviceStatuses.find(x => x.id === id)
  if (s) {
    s.status = 'Connected'
    s.lastError = ''
  }
  emitStatusUpdate()
  return Promise.resolve()
}

export function DisconnectDevice(id: string): Promise<void> {
  __logCall('DisconnectDevice', [id])
  const s = mockState.deviceStatuses.find(x => x.id === id)
  if (s) {
    s.status = 'Disconnected'
    s.acquiring = false
  }
  emitStatusUpdate()
  return Promise.resolve()
}

export function StartAcquisition(id: string): Promise<void> {
  __logCall('StartAcquisition', [id])
  const s = mockState.deviceStatuses.find(x => x.id === id)
  if (s) s.acquiring = true
  emitStatusUpdate()
  return Promise.resolve()
}

export function StopAcquisition(id: string): Promise<void> {
  __logCall('StopAcquisition', [id])
  const s = mockState.deviceStatuses.find(x => x.id === id)
  if (s) s.acquiring = false
  emitStatusUpdate()
  return Promise.resolve()
}

export function StartAcquisitionAll(): Promise<number> {
  __logCall('StartAcquisitionAll', [])
  let count = 0
  for (const s of mockState.deviceStatuses) {
    if (s.status === 'Connected') {
      s.acquiring = true
      count++
    }
  }
  emitStatusUpdate()
  return Promise.resolve(count)
}

export function StopAcquisitionAll(): Promise<void> {
  __logCall('StopAcquisitionAll', [])
  for (const s of mockState.deviceStatuses) s.acquiring = false
  emitStatusUpdate()
  return Promise.resolve()
}

export function GetLatestData(): Promise<any[]> {
  return Promise.resolve([])
}

export function ScanDevices(): Promise<any[]> {
  __logCall('ScanDevices', [])
  // 返回 2 个模拟发现设备
  return Promise.resolve([
    { ip: '192.168.3.101', mac: 'AA:BB:CC:DD:00:01', type: 'XY-DAQ16' },
    { ip: '192.168.3.102', mac: 'AA:BB:CC:DD:00:02', type: 'XY-DAQ8' },
  ])
}

export function SetUnit(id: string, unit: string): Promise<void> {
  __logCall('SetUnit', [id, unit])
  const p = mockState.deviceProfiles.find(x => x.id === id)
  if (p && p.channels) {
    for (const ch of p.channels) ch.unit = unit
  }
  return Promise.resolve()
}

export function SetThermocoupleType(id: string, tcTypes: string): Promise<void> {
  __logCall('SetThermocoupleType', [id, tcTypes])
  return Promise.resolve()
}

export function SetSingleThermocoupleType(id: string, channelIndex: number, tcType: string): Promise<void> {
  __logCall('SetSingleThermocoupleType', [id, channelIndex, tcType])
  return Promise.resolve()
}
