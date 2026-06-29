/**
 * E2E mock 中央状态。
 *
 * 所有 service mock 共享此状态对象，测试通过 __resetMockState() 在每个用例前重置。
 * 状态字段尽量与后端真实结构对齐，使 mock 行为可预测。
 */
import type {
  DeviceProfile,
  DeviceStatus,
  MotionControllerProfile,
  MotionControllerStatus,
} from './bindings/types/models'

export interface MockState {
  // 设备管理
  deviceProfiles: DeviceProfile[]
  deviceStatuses: DeviceStatus[]
  // 运动控制
  motionProfiles: MotionControllerProfile[]
  motionStatuses: MotionControllerStatus[]
  // 三孔测试（按 probeID 索引）
  threeHoleCalibLoaded: Record<string, boolean>
  threeHoleCalibFiles: Record<string, string[]>
  threeHoleStatus: Record<string, any>
  threeHoleRealtimeRecording: Record<string, boolean>
  threeHoleConfigs: Record<string, any>
  // 五孔测试
  fiveHoleCalibLoaded: Record<string, boolean>
  fiveHoleCalibFiles: Record<string, string[]>
  fiveHoleStatus: any
  fiveHoleRealtimeRecording: boolean
  fiveHoleConfig: any
  // 校准
  calibrationStatus: any
  // 数据服务
  publishRate: number
  dataDir: string
  recording: boolean
  recordingFiles: string[]
  // 配置
  dataSavePath: string

  // 调用日志（测试可断言某方法是否被调用）
  calls: { method: string; args: any[] }[]
}

function createInitialState(): MockState {
  return {
    deviceProfiles: [],
    deviceStatuses: [],
    motionProfiles: [],
    motionStatuses: [],
    threeHoleCalibLoaded: {},
    threeHoleCalibFiles: {},
    threeHoleStatus: {},
    threeHoleRealtimeRecording: {},
    threeHoleConfigs: {},
    fiveHoleCalibLoaded: {},
    fiveHoleCalibFiles: {},
    fiveHoleStatus: { status: 'idle', currentPointIndex: -1, totalPoints: 0, completedPoints: 0 },
    fiveHoleRealtimeRecording: false,
    fiveHoleConfig: null,
    calibrationStatus: { status: 'idle', currentIndex: -1, totalCount: 0 },
    publishRate: 20,
    dataDir: 'C:/Users/test/.yx-daq',
    recording: false,
    recordingFiles: ['rec-001.csv', 'rec-002.csv'],
    dataSavePath: 'C:/Users/test/.yx-daq',
    calls: [],
  }
}

export const mockState: MockState = createInitialState()

/** 测试 helper：重置所有 mock 状态（在 beforeEach 中调用） */
export function __resetMockState(): void {
  Object.assign(mockState, createInitialState())
}

/** 测试 helper：记录方法调用 */
export function __logCall(method: string, args: any[]): void {
  mockState.calls.push({ method, args })
}

// ==================== 测试桥接 ====================
// 暴露到 window.__mock，供 Playwright 测试断言/重置
;(globalThis as any).__mock = {
  ...(globalThis as any).__mock,
  state: mockState,
  resetState: __resetMockState,
}
