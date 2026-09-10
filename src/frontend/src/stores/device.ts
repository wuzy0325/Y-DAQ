import { ref, computed, shallowRef } from 'vue'
import { defineStore } from 'pinia'
import { DeviceService } from '@bindings/yx-daq/internal/app'
import { createWailsEventListener } from '../utils/wailsEvents'

export interface ChannelConfig {
  index: number
  name: string
  enabled: boolean
  unit: string
  precision: number
  rangeMin: number
  rangeMax: number
  thermocoupleType?: string
  tempSource?: string
  zeroOffset?: number
  zeroOffsetUnit?: string
  zeroCalibratedAt?: number
}

interface DeviceProfile {
  id: string
  name: string
  type: string
  host: string
  port: number
  streamId: number
  periodMs: number
  channels: ChannelConfig[]
}

interface DeviceStatus {
  id: string
  name: string
  type: string
  status: 'Connected' | 'Disconnected' | 'Connecting' | 'Error'
  acquiring: boolean
  lastError: string
}

interface DataPayload {
  deviceId: string
  timestamp: number
  channels: number[]
  channelIndices: number[]
}

// 批量校零单设备结果（与后端 types.ZeroCalibrateResult 对应）
// 导出供 View 复用，避免在 View 中重复声明（coding-standards 反"重复代码"）
export interface ZeroCalibrateResult {
  deviceId: string
  deviceName: string
  success: boolean
  channels: number
  error: string
}

/**
 * 确保指定设备列表已连接并启动采集
 * 三孔/五孔测试在选择校准文件后调用，自动恢复中断的设备状态。
 *
 * @param deviceIds 设备ID列表（可迭代，会自动去重、过滤空值）
 * @returns 错误消息数组（每个失败设备一条）；空数组表示全部成功
 */
export async function ensureDevicesAcquiring(deviceIds: Iterable<string>): Promise<string[]> {
  const errors: string[] = []
  const uniqueIds = [...new Set([...deviceIds].filter(Boolean))]
  if (uniqueIds.length === 0) return errors

  try {
    const statuses = await DeviceService.GetDeviceStatusAll() as DeviceStatus[]
    for (const deviceId of uniqueIds) {
      const ds = statuses.find(s => s.id === deviceId)
      if (!ds) continue
      if (ds.status !== 'Connected') {
        try {
          await DeviceService.ConnectDevice(deviceId)
        } catch (e) {
          errors.push(`自动连接设备 ${deviceId} 失败: ${e}`)
          continue
        }
      }
      const updated = (await DeviceService.GetDeviceStatusAll() as DeviceStatus[])
        .find(s => s.id === deviceId)
      if (updated && !updated.acquiring) {
        try {
          await DeviceService.StartAcquisition(deviceId)
        } catch (e) {
          errors.push(`自动启动采集 ${deviceId} 失败: ${e}`)
        }
      }
    }
  } catch (e) {
    console.error('ensureDevicesAcquiring failed:', e)
    errors.push(`ensureDevicesAcquiring 失败: ${e}`)
  }
  return errors
}

export const useDeviceStore = defineStore('device', () => {
  const profiles = ref<DeviceProfile[]>([])
  const statuses = ref<DeviceStatus[]>([])
  const snapshots = shallowRef<DataPayload[]>([])
  const latestData = ref<Map<string, DataPayload>>(new Map())
  // 正在连接中的设备ID集合（用于按钮 loading 状态）
  const connectingIds = ref<Set<string>>(new Set())

  const isConnected = computed(() => statuses.value.some(s => s.status === 'Connected'))
  const isAcquiring = computed(() => statuses.value.some(s => s.acquiring))

  // 获取指定设备的连接状态
  function getDeviceStatus(id: string): DeviceStatus | undefined {
    return statuses.value.find(s => s.id === id)
  }

  // 判断设备是否正在连接中（store 状态或正在调用连接API）
  function isDeviceConnecting(id: string): boolean {
    return connectingIds.value.has(id) || getDeviceStatus(id)?.status === 'Connecting'
  }

  async function fetchProfiles() {
    try {
      profiles.value = await DeviceService.GetDeviceProfiles() as DeviceProfile[]
    } catch (e) {
      console.warn('fetchProfiles failed:', e)
    }
  }

  async function fetchStatuses() {
    try {
      statuses.value = await DeviceService.GetDeviceStatusAll() as DeviceStatus[]
    } catch (e) {
      console.warn('fetchStatuses failed:', e)
    }
  }

  // 统一的 action 包装器：执行操作 → 后续刷新 → 错误处理
  async function withDeviceAction(
    name: string,
    action: () => Promise<unknown>,
    after?: () => Promise<void>,
  ): Promise<string | null> {
    try {
      await action()
      if (after) await after()
      return null
    } catch (e: any) {
      const msg = e?.message || String(e)
      console.error(`${name} failed:`, msg)
      return msg
    }
  }

  async function updateProfile(profile: DeviceProfile): Promise<string | null> {
    return withDeviceAction('updateProfile', () => DeviceService.UpdateDeviceProfile(profile as any), fetchProfiles)
  }

  async function setUnit(id: string, unit: string): Promise<string | null> {
    return withDeviceAction('setUnit', () => DeviceService.SetUnit(id, unit), fetchProfiles)
  }

  async function setThermocoupleType(id: string, tcTypes: string): Promise<string | null> {
    return withDeviceAction('setThermocoupleType', () => DeviceService.SetThermocoupleType(id, tcTypes), fetchProfiles)
  }

  async function setSingleThermocoupleType(id: string, channelIndex: number, tcType: string): Promise<string | null> {
    return withDeviceAction('setSingleThermocoupleType', () => DeviceService.SetSingleThermocoupleType(id, channelIndex, tcType), fetchProfiles)
  }

  async function setTempSource(id: string, source: string): Promise<string | null> {
    return withDeviceAction('setTempSource', () => DeviceService.SetTempSource(id, source), fetchProfiles)
  }

  async function setTempThermocoupleType(id: string, tcType: string): Promise<string | null> {
    return withDeviceAction('setTempThermocoupleType', () => DeviceService.SetTempThermocoupleType(id, tcType), fetchProfiles)
  }

  async function zeroCalibrate(id: string): Promise<string | null> {
    return withDeviceAction('zeroCalibrate', () => DeviceService.ZeroCalibrate(id), fetchProfiles)
  }

  async function zeroCalibrateChannel(id: string, channelIndex: number): Promise<string | null> {
    return withDeviceAction('zeroCalibrateChannel', () => DeviceService.ZeroCalibrateChannel(id, channelIndex), fetchProfiles)
  }

  async function clearZeroOffset(id: string, channelIndex: number): Promise<string | null> {
    return withDeviceAction('clearZeroOffset', () => DeviceService.ClearZeroOffset(id, channelIndex), fetchProfiles)
  }

  async function clearAllZeroOffsets(id: string): Promise<string | null> {
    return withDeviceAction('clearAllZeroOffsets', () => DeviceService.ClearAllZeroOffsets(id), fetchProfiles)
  }

  // 批量校零所有符合条件的设备（已连接+采集中+压力DAQ设备）
  // 遵循 coding-standards §2.1 复杂操作返回约定：{ success, results?, error? }
  // - success=true：整体调用成功，results 为各设备结果数组（可能为空=无符合条件设备）
  // - success=false：整体调用失败（如后端异常），error 为错误消息；results 仍可能包含已采集结果
  // 单个设备失败仅记录在对应 results[i].error 中，不影响 success
  async function zeroCalibrateAll(): Promise<{ success: boolean; results?: ZeroCalibrateResult[]; error?: string }> {
    try {
      const results = await DeviceService.ZeroCalibrateAll() as ZeroCalibrateResult[]
      await fetchProfiles()
      return { success: true, results: results || [] }
    } catch (e: any) {
      const msg = e?.message || String(e)
      return { success: false, error: msg }
    }
  }

  async function connectDevice(id: string): Promise<string | null> {
    connectingIds.value = new Set([...connectingIds.value, id])
    try {
      return await withDeviceAction('connectDevice', async () => {
        await DeviceService.ConnectDevice(id)
        await new Promise(resolve => setTimeout(resolve, 1000))
      }, async () => { await fetchProfiles(); await fetchStatuses() })
    } finally {
      const newSet = new Set(connectingIds.value)
      newSet.delete(id)
      connectingIds.value = newSet
    }
  }

  async function disconnectDevice(id: string): Promise<string | null> {
    if (connectingIds.value.has(id)) return null
    return withDeviceAction('disconnectDevice', () => DeviceService.DisconnectDevice(id), fetchStatuses)
  }

  async function startAcquisition(id: string): Promise<string | null> {
    return withDeviceAction('startAcquisition', () => DeviceService.StartAcquisition(id), fetchStatuses)
  }

  async function stopAcquisition(id: string): Promise<string | null> {
    return withDeviceAction('stopAcquisition', () => DeviceService.StopAcquisition(id), fetchStatuses)
  }

  // 单设备采集启停 busy 状态（按设备ID隔离，避免并发点击）
  const acqBusyIds = ref<Set<string>>(new Set())

  function isAcqBusy(id: string): boolean {
    return acqBusyIds.value.has(id)
  }

  // 切换单设备采集状态：根据当前 acquiring 决定 start/stop，内部管理 busy
  // 返回 { error, name, wasAcquiring } 供调用方组装提示
  async function toggleAcquisition(id: string): Promise<{ error: string | null; name: string; wasAcquiring: boolean }> {
    const ds = getDeviceStatus(id)
    const name = ds?.name ?? id
    const wasAcquiring = ds?.acquiring ?? false
    if (acqBusyIds.value.has(id)) return { error: null, name, wasAcquiring }
    acqBusyIds.value = new Set([...acqBusyIds.value, id])
    try {
      const err = wasAcquiring
        ? await stopAcquisition(id)
        : await startAcquisition(id)
      return { error: err, name, wasAcquiring }
    } finally {
      const newSet = new Set(acqBusyIds.value)
      newSet.delete(id)
      acqBusyIds.value = newSet
    }
  }

  const eventListener = createWailsEventListener('device', [
    {
      channel: 'daq:data-snapshot',
      handler: (event: any) => {
        const data = event.data as DataPayload[]
        snapshots.value = data
        for (const payload of data) {
          latestData.value.set(payload.deviceId, payload)
        }
      },
    },
    { channel: 'device:status-updated', handler: (event: any) => { statuses.value = event.data as DeviceStatus[] } },
    // 温度校准写入/清除后刷新 profiles（DeviceView 编辑对话框的温度校准字段随之更新）
    { channel: 'device:temp-calib-updated', handler: () => { fetchProfiles() } },
  ])

  function startListening() {
    eventListener.start()
    fetchProfiles()
    fetchStatuses()
    // 兜底：IPC 首次调用可能因 runtime 未完全就绪而失败，延迟 800ms 单次重试
    // （800ms 经验值，略大于 Wails v3 runtime 典型就绪时间）
    // 注意：statuses 即使本次仍失败，也会被 broadcastDeviceStatus 事件推送恢复；
    //       profiles 无广播兜底，若本次仍失败需用户手动刷新
    setTimeout(() => {
      if (profiles.value.length === 0) fetchProfiles()
      if (statuses.value.length === 0) fetchStatuses()
    }, 800)
  }

  const stopListening = eventListener.stop

  return {
    profiles, statuses, snapshots, latestData, connectingIds,
    isConnected, isAcquiring,
    getDeviceStatus, isDeviceConnecting,
    fetchProfiles, fetchStatuses, updateProfile, setUnit,
    setThermocoupleType, setSingleThermocoupleType,
    setTempSource, setTempThermocoupleType,
    zeroCalibrate, zeroCalibrateChannel, clearZeroOffset, clearAllZeroOffsets,
    zeroCalibrateAll,
    connectDevice, disconnectDevice,
    startAcquisition, stopAcquisition,
    acqBusyIds, isAcqBusy, toggleAcquisition,
    startListening, stopListening,
  }
})
