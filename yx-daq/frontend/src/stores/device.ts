import { ref, computed, shallowRef } from 'vue'
import { defineStore } from 'pinia'
import {
  GetDeviceProfiles, UpdateDeviceProfile,
  ConnectDevice, DisconnectDevice,
  StartAcquisition, StopAcquisition,
  StartAcquisitionAll, StopAcquisitionAll,
  GetDeviceStatusAll, ScanDevices,
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
  status: string
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

  const isConnected = computed(() => statuses.value.some(s => s.status === 'Connected'))
  const isAcquiring = computed(() => statuses.value.some(s => s.acquiring))

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

  async function fetchStatuses() {
    try {
      statuses.value = await GetDeviceStatusAll() as DeviceStatus[]
    } catch (e) {
      console.warn('fetchStatuses failed:', e)
    }
  }

  async function connectDevice(id: string): Promise<string | null> {
    try {
      await ConnectDevice(id)
      await fetchStatuses()
      return null
    } catch (e: any) {
      const msg = e?.message || String(e)
      console.error('connectDevice failed:', msg)
      return msg
    }
  }

  async function disconnectDevice(id: string): Promise<string | null> {
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
  }

  return {
    profiles, statuses, snapshots, latestData,
    isConnected, isAcquiring,
    fetchProfiles, fetchStatuses, updateProfile,
    connectDevice, disconnectDevice,
    startAcquisition, stopAcquisition,
    startListening,
  }
})
