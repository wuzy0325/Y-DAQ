import { describe, it, expect, vi, beforeEach } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'

// ---- mock Wails 绑定 + runtime（createWailsEventListener 依赖 Events.On/Off）----
const { mockDeviceService, mockEventsOn, mockEventsOff } = vi.hoisted(() => ({
  mockDeviceService: {
    GetDeviceStatusAll: vi.fn(),
    ConnectDevice: vi.fn(),
    StartAcquisition: vi.fn(),
    StopAcquisition: vi.fn(),
    DisconnectDevice: vi.fn(),
    GetDeviceProfiles: vi.fn(),
    UpdateDeviceProfile: vi.fn(),
    SetUnit: vi.fn(),
    SetThermocoupleType: vi.fn(),
    SetSingleThermocoupleType: vi.fn(),
  },
  mockEventsOn: vi.fn(() => () => {}),
  mockEventsOff: vi.fn(),
}))

vi.mock('@bindings/yx-daq/internal/app', () => ({
  DeviceService: mockDeviceService,
}))

vi.mock('@wailsio/runtime', () => ({
  Events: { On: mockEventsOn, Off: mockEventsOff },
}))

import { ensureDevicesAcquiring, useDeviceStore } from '../device'

describe('stores/device :: ensureDevicesAcquiring（编排逻辑）', () => {
  beforeEach(() => {
    mockDeviceService.GetDeviceStatusAll.mockReset()
    mockDeviceService.ConnectDevice.mockReset()
    mockDeviceService.StartAcquisition.mockReset()
  })

  it('空输入 → 返回空数组且不调用任何 service', async () => {
    const errors = await ensureDevicesAcquiring([])
    expect(errors).toEqual([])
    expect(mockDeviceService.GetDeviceStatusAll).not.toHaveBeenCalled()
  })

  it('过滤空字符串 + 去重（uniqueIds 仅保留非空唯一值）', async () => {
    mockDeviceService.GetDeviceStatusAll.mockResolvedValue([
      { id: 'd1', status: 'Connected', acquiring: true },
    ])
    const errors = await ensureDevicesAcquiring(['', 'd1', 'd1', '', '  '])
    // '  ' 非空字符串被保留但不在 statuses 中 → if(!ds) continue（跳过 per-device 查询）
    expect(errors).toEqual([])
    // 1 次初始 + d1 的 1 次 per-device 查询 = 2 次（'  ' 在查询前被 continue 跳过）
    expect(mockDeviceService.GetDeviceStatusAll).toHaveBeenCalledTimes(2)
    expect(mockDeviceService.ConnectDevice).not.toHaveBeenCalled()
    expect(mockDeviceService.StartAcquisition).not.toHaveBeenCalled()
  })

  it('设备不在 statuses 中 → continue，不连接不采集', async () => {
    mockDeviceService.GetDeviceStatusAll.mockResolvedValue([
      { id: 'd1', status: 'Connected', acquiring: true },
    ])
    const errors = await ensureDevicesAcquiring(['unknown'])
    expect(errors).toEqual([])
    expect(mockDeviceService.ConnectDevice).not.toHaveBeenCalled()
  })

  it('已 Connected + acquiring → 不调用 Connect/Start', async () => {
    mockDeviceService.GetDeviceStatusAll.mockResolvedValue([
      { id: 'd1', status: 'Connected', acquiring: true },
    ])
    const errors = await ensureDevicesAcquiring(['d1'])
    expect(errors).toEqual([])
    expect(mockDeviceService.ConnectDevice).not.toHaveBeenCalled()
    expect(mockDeviceService.StartAcquisition).not.toHaveBeenCalled()
  })

  it('未 Connected → 调用 ConnectDevice', async () => {
    // 初始状态 Disconnected，连接后状态查询返回 Connected + acquiring
    mockDeviceService.GetDeviceStatusAll.mockResolvedValueOnce([
      { id: 'd1', status: 'Disconnected', acquiring: false },
    ])
    mockDeviceService.GetDeviceStatusAll.mockResolvedValueOnce([
      { id: 'd1', status: 'Connected', acquiring: true },
    ])
    mockDeviceService.ConnectDevice.mockResolvedValue(undefined)

    const errors = await ensureDevicesAcquiring(['d1'])
    expect(errors).toEqual([])
    expect(mockDeviceService.ConnectDevice).toHaveBeenCalledWith('d1')
    // 连接后已 acquiring，不再 StartAcquisition
    expect(mockDeviceService.StartAcquisition).not.toHaveBeenCalled()
  })

  it('Connected 但未 acquiring → 调用 StartAcquisition（不调用 Connect）', async () => {
    mockDeviceService.GetDeviceStatusAll.mockResolvedValue([
      { id: 'd1', status: 'Connected', acquiring: false },
    ])
    mockDeviceService.StartAcquisition.mockResolvedValue(undefined)

    const errors = await ensureDevicesAcquiring(['d1'])
    expect(errors).toEqual([])
    expect(mockDeviceService.ConnectDevice).not.toHaveBeenCalled()
    expect(mockDeviceService.StartAcquisition).toHaveBeenCalledWith('d1')
  })

  it('ConnectDevice 抛错 → 收集错误且跳过 StartAcquisition', async () => {
    mockDeviceService.GetDeviceStatusAll.mockResolvedValueOnce([
      { id: 'd1', status: 'Disconnected', acquiring: false },
    ])
    mockDeviceService.ConnectDevice.mockRejectedValue(new Error('连接超时'))

    const errors = await ensureDevicesAcquiring(['d1'])
    expect(errors).toHaveLength(1)
    expect(errors[0]).toContain('自动连接设备 d1 失败')
    expect(errors[0]).toContain('连接超时')
    expect(mockDeviceService.StartAcquisition).not.toHaveBeenCalled()
  })

  it('StartAcquisition 抛错 → 收集错误', async () => {
    mockDeviceService.GetDeviceStatusAll.mockResolvedValue([
      { id: 'd1', status: 'Connected', acquiring: false },
    ])
    mockDeviceService.StartAcquisition.mockRejectedValue(new Error('采集已运行'))

    const errors = await ensureDevicesAcquiring(['d1'])
    expect(errors).toHaveLength(1)
    expect(errors[0]).toContain('自动启动采集 d1 失败')
    expect(errors[0]).toContain('采集已运行')
  })

  it('多设备：部分成功部分失败，错误分别收集', async () => {
    // 稳定状态：d1 Connected+acquiring（无需操作），d2 Connected 未 acquiring（需 Start）
    mockDeviceService.GetDeviceStatusAll.mockResolvedValue([
      { id: 'd1', status: 'Connected', acquiring: true },
      { id: 'd2', status: 'Connected', acquiring: false },
    ])
    // d2 的 StartAcquisition 抛错
    mockDeviceService.StartAcquisition.mockImplementation((id: string) =>
      id === 'd2' ? Promise.reject(new Error('d2 启动失败')) : Promise.resolve(undefined)
    )

    const errors = await ensureDevicesAcquiring(['d1', 'd2'])
    // d1 无操作；d2 StartAcquisition 失败
    expect(errors).toHaveLength(1)
    expect(errors[0]).toContain('自动启动采集 d2 失败')
    expect(errors[0]).toContain('d2 启动失败')
    // 不应包含 d1
    expect(errors[0]).not.toContain('d1')
  })

  it('GetDeviceStatusAll 初始查询抛错 → 返回顶层错误', async () => {
    mockDeviceService.GetDeviceStatusAll.mockRejectedValue(new Error('IPC 不可用'))

    const errors = await ensureDevicesAcquiring(['d1'])
    expect(errors).toHaveLength(1)
    expect(errors[0]).toContain('ensureDevicesAcquiring 失败')
    expect(errors[0]).toContain('IPC 不可用')
  })
})

describe('stores/device :: useDeviceStore（computed + getters）', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    mockDeviceService.GetDeviceProfiles.mockReset()
    mockDeviceService.GetDeviceStatusAll.mockReset()
  })

  it('isConnected：任一 status===Connected → true', () => {
    const store = useDeviceStore()
    store.statuses = [
      { id: 'd1', status: 'Disconnected', acquiring: false },
    ] as any
    expect(store.isConnected).toBe(false)
    store.statuses = [
      { id: 'd1', status: 'Disconnected', acquiring: false },
      { id: 'd2', status: 'Connected', acquiring: false },
    ] as any
    expect(store.isConnected).toBe(true)
  })

  it('isAcquiring：任一 acquiring===true → true', () => {
    const store = useDeviceStore()
    store.statuses = [{ id: 'd1', status: 'Connected', acquiring: false }] as any
    expect(store.isAcquiring).toBe(false)
    store.statuses = [{ id: 'd1', status: 'Connected', acquiring: true }] as any
    expect(store.isAcquiring).toBe(true)
  })

  it('getDeviceStatus：按 id 查找', () => {
    const store = useDeviceStore()
    store.statuses = [
      { id: 'd1', status: 'Connected', acquiring: true },
      { id: 'd2', status: 'Disconnected', acquiring: false },
    ] as any
    expect(store.getDeviceStatus('d2')?.status).toBe('Disconnected')
    expect(store.getDeviceStatus('unknown')).toBeUndefined()
  })

  it('isDeviceConnecting：connectingIds 包含或 status===Connecting → true', () => {
    const store = useDeviceStore()
    store.statuses = [{ id: 'd1', status: 'Connecting', acquiring: false }] as any
    expect(store.isDeviceConnecting('d1')).toBe(true)

    store.statuses = [{ id: 'd1', status: 'Connected', acquiring: false }] as any
    store.connectingIds = new Set(['d1'])
    expect(store.isDeviceConnecting('d1')).toBe(true)

    store.connectingIds = new Set()
    expect(store.isDeviceConnecting('d1')).toBe(false)
  })
})
