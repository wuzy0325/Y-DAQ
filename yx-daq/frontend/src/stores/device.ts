import { ref, computed, shallowRef } from 'vue'
import { defineStore } from 'pinia'
import {
  GetDeviceProfiles, UpdateDeviceProfile,
  ConnectDevice, DisconnectDevice,
  StartAcquisition, StopAcquisition,
  StartAcquisitionAll, StopAcquisitionAll,
  GetDeviceStatusAll, ScanDevices,
  SetUnit, SetThermocoupleType, SetSingleThermocoupleType,
} from '../../wailsjs/go/main/App'
import { EventsOn } from '../../wailsjs/runtime/runtime'

interface ChannelConfig {
  index: number
  name: string
  enabled: boolean
  unit: string
  precision: number
  rangeMin: number
  rangeMax: number
  thermocoupleType?: string
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

export const useDeviceStore = defineStore('device', () => {
  const profiles = ref<DeviceProfile[]>([])
  const statuses = ref<DeviceStatus[]>([])
  const snapshots = shallowRef<DataPayload[]>([])
  const latestData = ref<Map<string, DataPayload>>(new Map())
  // 正在连接中的设备ID集合（用于按钮 loading 状态）
  const connectingIds = ref<Set<string>>(new Set())
  // 响应式快照，确保模板能追踪 connectingIds 变化
  const connectingIdSet = computed(() => connectingIds.value)

  const isConnected = computed(() => statuses.value.some(s => s.status === 'Connected'))
  const isAcquiring = computed(() => statuses.value.some(s => s.acquiring))

  // 获取指定设备的连接状态
  function getDeviceStatus(id: string): DeviceStatus | undefined {
    return statuses.value.find(s => s.id === id)
  }

  // 判断设备是否正在连接中（store 状态或正在调用连接API）
  // 注意：访问 connectingIdSet.value 确保响应式追踪
  function isDeviceConnecting(id: string): boolean {
    return connectingIdSet.value.has(id) || getDeviceStatus(id)?.status === 'Connecting'
  }

  async function fetchProfiles() {
    try {
      profiles.value = await GetDeviceProfiles() as DeviceProfile[]
    } catch (e) {
      console.warn('fetchProfiles failed:', e)
    }
  }

  async function updateProfile(profile: DeviceProfile): Promise<string | null> {
    try {
      await UpdateDeviceProfile(profile as any)
      await fetchProfiles()
      return null
    } catch (e: any) {
      const msg = e?.message || String(e)
      console.error('updateProfile failed:', msg)
      return msg
    }
  }

  async function setUnit(id: string, unit: string): Promise<string | null> {
    try {
      await SetUnit(id, unit)
      await fetchProfiles()
      return null
    } catch (e: any) {
      const msg = e?.message || String(e)
      console.error('setUnit failed:', msg)
      return msg
    }
  }

  async function setThermocoupleType(id: string, tcTypes: string): Promise<string | null> {
    try {
      await SetThermocoupleType(id, tcTypes)
      await fetchProfiles()
      return null
    } catch (e: any) {
      const msg = e?.message || String(e)
      console.error('setThermocoupleType failed:', msg)
      return msg
    }
  }

  async function setSingleThermocoupleType(id: string, channelIndex: number, tcType: string): Promise<string | null> {
    try {
      await SetSingleThermocoupleType(id, channelIndex, tcType)
      await fetchProfiles()
      return null
    } catch (e: any) {
      const msg = e?.message || String(e)
      console.error('setSingleThermocoupleType failed:', msg)
      return msg
    }
  }

  async function fetchStatuses() {
    try {
      statuses.value = await GetDeviceStatusAll() as DeviceStatus[]
    } catch (e) {
      console.warn('fetchStatuses failed:', e)
    }
  }

  async function connectDevice(id: string): Promise<string | null> {
    connectingIds.value = new Set([...connectingIds.value, id])
    try {
      await ConnectDevice(id)
      // 等待配置同步完成（驱动内部有 300ms 延迟 + 命令交互时间）
      await new Promise(resolve => setTimeout(resolve, 1000))
      await fetchProfiles()
      await fetchStatuses()
      return null
    } catch (e: any) {
      const msg = e?.message || String(e)
      console.error('connectDevice failed:', msg)
      return msg
    } finally {
      const newSet = new Set(connectingIds.value)
      newSet.delete(id)
      connectingIds.value = newSet
    }
  }

  async function disconnectDevice(id: string): Promise<string | null> {
    if (connectingIds.value.has(id)) return null // 正在连接中不允许断开
    try {
      await DisconnectDevice(id)
      await fetchStatuses()
      return null
    } catch (e: any) {
      const msg = e?.message || String(e)
      console.error('disconnectDevice failed:', msg)
      return msg
    }
  }

  async function startAcquisition(id: string): Promise<string | null> {
    try {
      await StartAcquisition(id)
      await fetchStatuses()
      return null
    } catch (e: any) {
      const msg = e?.message || String(e)
      console.error('startAcquisition failed:', msg)
      return msg
    }
  }

  async function stopAcquisition(id: string): Promise<string | null> {
    try {
      await StopAcquisition(id)
      await fetchStatuses()
      return null
    } catch (e: any) {
      const msg = e?.message || String(e)
      console.error('stopAcquisition failed:', msg)
      return msg
    }
  }

  function startListening() {
    try {
      EventsOn('daq:data-snapshot', (data: DataPayload[]) => {
        snapshots.value = data
        for (const payload of data) {
          latestData.value.set(payload.deviceId, payload)
        }
      })
      EventsOn('device:status-updated', (data: DeviceStatus[]) => {
        statuses.value = data
      })
    } catch (e) {
      console.warn('startListening failed:', e)
    }

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

  return {
    profiles, statuses, snapshots, latestData, connectingIds,
    isConnected, isAcquiring,
    getDeviceStatus, isDeviceConnecting,
    fetchProfiles, fetchStatuses, updateProfile, setUnit,
    setThermocoupleType, setSingleThermocoupleType,
    connectDevice, disconnectDevice,
    startAcquisition, stopAcquisition,
    startListening,
  }
})
