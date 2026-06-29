/**
 * E2E mock: MotionService（运动控制服务）
 *
 * 维护 mockState.motionProfiles / motionStatuses，方法签名与真实 bindings 一致。
 * 状态变更后通过 __emitEvent 推送 motion:status-updated。
 *
 * 注意：axes 字段为 AxisStatus[]（数组，每项含 name 字段），与前端 store 的
 * `for (const ax of ctrl.axes)` 迭代方式匹配；切勿改成对象 keyed by name。
 */
import { mockState, __logCall } from '../../state'
import { __emitEvent } from '../../wails-runtime'

interface MockAxisStatus {
  name: string
  position: number
  moving: boolean
  homed: boolean
  posLimit: boolean
  negLimit: boolean
  compensating: boolean
}

function defaultAxes(): MockAxisStatus[] {
  return [
    { name: 'X', position: 0, moving: false, homed: false, posLimit: false, negLimit: false, compensating: false },
    { name: 'Y', position: 0, moving: false, homed: false, posLimit: false, negLimit: false, compensating: false },
    { name: 'Z', position: 0, moving: false, homed: false, posLimit: false, negLimit: false, compensating: false },
    { name: 'U', position: 0, moving: false, homed: false, posLimit: false, negLimit: false, compensating: false },
  ]
}

function cloneStatuses() {
  // 深拷贝状态（含 axes 数组），模拟 Wails v3 JSON 序列化，避免前端 store 持有同一引用导致响应式失效
  return mockState.motionStatuses.map(s => ({
    ...s,
    axes: s.axes.map(a => ({ ...a })),
  }))
}

function emitStatusUpdate(): void {
  __emitEvent('motion:status-updated', cloneStatuses())
}

function ensureStatus(id: string, name: string, type: string): void {
  let s = mockState.motionStatuses.find(x => x.id === id)
  if (!s) {
    s = {
      id,
      name,
      type,
      status: 'Disconnected',
      axes: defaultAxes(),
      lastError: '',
    }
    mockState.motionStatuses.push(s)
  }
}

function findAxis(s: any, axis: string): MockAxisStatus | undefined {
  return s.axes.find((a: MockAxisStatus) => a.name === axis)
}

export function AddMotionProfile(profile: any): Promise<void> {
  __logCall('AddMotionProfile', [profile])
  mockState.motionProfiles.push(profile)
  ensureStatus(profile.id, profile.name, profile.type)
  emitStatusUpdate()
  return Promise.resolve()
}

export function UpdateMotionProfile(profile: any): Promise<void> {
  __logCall('UpdateMotionProfile', [profile])
  const idx = mockState.motionProfiles.findIndex(p => p.id === profile.id)
  if (idx >= 0) mockState.motionProfiles[idx] = profile
  const s = mockState.motionStatuses.find(x => x.id === profile.id)
  if (s) {
    s.name = profile.name
    s.type = profile.type
  }
  emitStatusUpdate()
  return Promise.resolve()
}

export function RemoveMotionProfile(id: string): Promise<void> {
  __logCall('RemoveMotionProfile', [id])
  mockState.motionProfiles = mockState.motionProfiles.filter(p => p.id !== id)
  mockState.motionStatuses = mockState.motionStatuses.filter(s => s.id !== id)
  emitStatusUpdate()
  return Promise.resolve()
}

export function GetMotionProfiles(): Promise<any[]> {
  __logCall('GetMotionProfiles', [])
  return Promise.resolve(mockState.motionProfiles.map(p => ({ ...p })))
}

export function GetMotionStatusAll(): Promise<any[]> {
  __logCall('GetMotionStatusAll', [])
  return Promise.resolve(cloneStatuses())
}

export function ConnectMotion(id: string): Promise<void> {
  __logCall('ConnectMotion', [id])
  const s = mockState.motionStatuses.find(x => x.id === id)
  if (s) {
    s.status = 'Connected'
    s.lastError = ''
  }
  emitStatusUpdate()
  return Promise.resolve()
}

export function DisconnectMotion(id: string): Promise<void> {
  __logCall('DisconnectMotion', [id])
  const s = mockState.motionStatuses.find(x => x.id === id)
  if (s) {
    s.status = 'Disconnected'
    for (const ax of s.axes) {
      ;(ax as MockAxisStatus).moving = false
    }
  }
  emitStatusUpdate()
  return Promise.resolve()
}

export function MotionMoveTo(id: string, axis: string, position: number): Promise<void> {
  __logCall('MotionMoveTo', [id, axis, position])
  const s = mockState.motionStatuses.find(x => x.id === id)
  const ax = s && findAxis(s, axis)
  if (ax) {
    ax.moving = true
    emitStatusUpdate()
    // 模拟运动完成
    setTimeout(() => {
      ax.moving = false
      ax.position = position
      emitStatusUpdate()
    }, 100)
  }
  return Promise.resolve()
}

export function MotionMoveBy(id: string, axis: string, delta: number): Promise<void> {
  __logCall('MotionMoveBy', [id, axis, delta])
  const s = mockState.motionStatuses.find(x => x.id === id)
  const ax = s && findAxis(s, axis)
  if (ax) {
    ax.moving = true
    emitStatusUpdate()
    setTimeout(() => {
      ax.moving = false
      ax.position += delta
      emitStatusUpdate()
    }, 100)
  }
  return Promise.resolve()
}

export function MotionJog(id: string, axis: string, direction: number, distance: number, speed: number): Promise<void> {
  __logCall('MotionJog', [id, axis, direction, distance, speed])
  return Promise.resolve()
}

export function MotionHome(id: string, axis: string): Promise<void> {
  __logCall('MotionHome', [id, axis])
  const s = mockState.motionStatuses.find(x => x.id === id)
  const ax = s && findAxis(s, axis)
  if (ax) {
    ax.moving = true
    emitStatusUpdate()
    setTimeout(() => {
      ax.moving = false
      ax.position = 0
      ax.homed = true
      emitStatusUpdate()
    }, 100)
  }
  return Promise.resolve()
}

export function MotionStop(id: string, axis: string): Promise<void> {
  __logCall('MotionStop', [id, axis])
  const s = mockState.motionStatuses.find(x => x.id === id)
  const ax = s && findAxis(s, axis)
  if (ax) ax.moving = false
  emitStatusUpdate()
  return Promise.resolve()
}

export function MotionStopAll(id: string): Promise<void> {
  __logCall('MotionStopAll', [id])
  const s = mockState.motionStatuses.find(x => x.id === id)
  if (s) for (const ax of s.axes) (ax as MockAxisStatus).moving = false
  emitStatusUpdate()
  return Promise.resolve()
}

export function MotionEmergencyStop(id: string): Promise<void> {
  __logCall('MotionEmergencyStop', [id])
  const s = mockState.motionStatuses.find(x => x.id === id)
  if (s) for (const ax of s.axes) (ax as MockAxisStatus).moving = false
  emitStatusUpdate()
  return Promise.resolve()
}

export function MotionMotorOff(id: string): Promise<void> {
  __logCall('MotionMotorOff', [id])
  emitStatusUpdate()
  return Promise.resolve()
}

export function MotionDefinePosition(id: string, axis: string, position: number): Promise<void> {
  __logCall('MotionDefinePosition', [id, axis, position])
  const s = mockState.motionStatuses.find(x => x.id === id)
  const ax = s && findAxis(s, axis)
  if (ax) {
    ax.position = position
    if (position === 0) ax.homed = true
  }
  emitStatusUpdate()
  return Promise.resolve()
}

export function MotionIsMoving(id: string): Promise<boolean> {
  const s = mockState.motionStatuses.find(x => x.id === id)
  if (!s) return Promise.resolve(false)
  return Promise.resolve(s.axes.some((a: MockAxisStatus) => a.moving))
}

export function MotionIsAxisMoving(id: string, axis: string): Promise<boolean> {
  const s = mockState.motionStatuses.find(x => x.id === id)
  const ax = s && findAxis(s, axis)
  return Promise.resolve(!!ax && ax.moving)
}

export function MotionGetLimitStatus(id: string, axis: string): Promise<any> {
  return Promise.resolve({ posLimit: false, negLimit: false })
}

export function MotionSetAcceleration(id: string, axis: string, accel: number): Promise<void> {
  __logCall('MotionSetAcceleration', [id, axis, accel])
  return Promise.resolve()
}

export function MotionSetDeceleration(id: string, axis: string, decel: number): Promise<void> {
  __logCall('MotionSetDeceleration', [id, axis, decel])
  return Promise.resolve()
}

export function MotionSetAxisDirection(id: string, axis: string, reverse: boolean): Promise<void> {
  __logCall('MotionSetAxisDirection', [id, axis, reverse])
  return Promise.resolve()
}

export function MotionWaitForComplete(id: string, axis: string, timeoutMs: number): Promise<void> {
  return Promise.resolve()
}

export function OpenMotionWindow(): Promise<string> {
  __logCall('OpenMotionWindow', [])
  return Promise.resolve('motion-window-opened')
}
