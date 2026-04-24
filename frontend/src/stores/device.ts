import { defineStore } from 'pinia'
import { ref, computed, shallowRef } from 'vue'

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
      const { GetDeviceProfiles } = await import('../../wailsjs/go/main/App')
      profiles.value = await GetDeviceProfiles() as DeviceProfile[]
    } catch (e) {
      console.warn('fetchProfiles failed:', e)
    }
  }

  async function updateProfile(profile: DeviceProfile): Promise<string | null> {
    try {
      const { UpdateDeviceProfile } = await import('../../wailsjs/go/main/App')
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
      const { GetDeviceStatusAll } = await import('../../wailsjs/go/main/App')
      statuses.value = await GetDeviceStatusAll() as DeviceStatus[]
    } catch (e) {
      console.warn('fetchStatuses failed:', e)
    }
  }

  async function connectDevice(id: string): Promise<string | null> {
    try {
      const { ConnectDevice } = await import('../../wailsjs/go/main/App')
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
      const { DisconnectDevice } = await import('../../wailsjs/go/main/App')
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
      const { StartAcquisition } = await import('../../wailsjs/go/main/App')
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
      const { StopAcquisition } = await import('../../wailsjs/go/main/App')
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
      import('../../wailsjs/runtime/runtime').then(({ EventsOn }) => {
        EventsOn('daq:data-snapshot', (data: DataPayload[]) => {
          snapshots.value = data
          for (const payload of data) {
            latestData.value.set(payload.deviceId, payload)
          }
        })
        EventsOn('device:status-updated', (data: DeviceStatus[]) => {
          statuses.value = data
        })
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
