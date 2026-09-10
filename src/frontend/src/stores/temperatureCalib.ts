import { ref, computed, triggerRef } from 'vue'
import { defineStore } from 'pinia'
import { DeviceService } from '@bindings/yx-daq/internal/app'
import * as types from '@bindings/yx-daq/internal/types'

// 本地派生的通道配置（仅取温度校准相关字段，避免依赖完整 ChannelConfig 接口）
export interface TempChannelInfo {
  index: number
  name: string
  enabled: boolean
  thermocoupleType?: string
  // 已写入的校准信息（来自 profile.Channels[i]，写入后通过 refreshProfiles 刷新）
  tempCalibA?: number
  tempCalibB?: number
  tempCalibR2?: number
  tempCalibPoints?: number
  tempCalibratedAt?: number
}

// 单通道校准点表格行（前端临时状态，不持久化）
export interface TempPointRow {
  measured: number  // 实测温度（采点均值）
  reference: number // 参考温度（手填）
}

// 校准点数范围（与后端 Validate 一致）
export const MIN_POINTS = 2
export const MAX_POINTS = 10

// 设备摘要（仅温度设备）
interface TempDeviceSummary {
  id: string
  name: string
  type: string
}

// 通道原始数据（从 profile.channels 直接取，仅取需要的字段）
interface RawChannel {
  index: number
  name: string
  enabled: boolean
  thermocoupleType?: string
  tempCalibA?: number
  tempCalibB?: number
  tempCalibR2?: number
  tempCalibPoints?: number
  tempCalibratedAt?: number
}

interface RawProfile {
  id: string
  name: string
  type: string
  channels: RawChannel[]
}

// 操作结果统一格式：对齐 coding-standards §2.1/§2.13 约定 { success, error? }
// success=true 时 message 为成功提示（用于 toast）；success=false 时 message 为错误消息
export interface ActionResult {
  success: boolean
  message: string
}

export const useTemperatureCalibStore = defineStore('temperatureCalib', () => {
  // 选中的温度设备 ID
  const selectedDeviceId = ref<string>('')
  // 当前设备所有通道（仅温度设备的启用通道）
  const channels = ref<TempChannelInfo[]>([])
  // 用户勾选要参与采点的通道索引
  const selectedChannelIndices = ref<number[]>([])
  // 校准点数 N（2~10）
  const pointCount = ref<number>(2)
  // 每通道独立的校准点表格 channelIndex -> rows
  // 用 ref<Map> 时，set/delete 后必须 triggerRef 触发响应式
  const pointsByChannel = ref<Map<number, TempPointRow[]>>(new Map())
  // 每通道最近一次拟合结果（未写入前为预演值）
  const fitResultByChannel = ref<Map<number, types.TempCalibResult | null>>(new Map())
  // 采点中
  const sampling = ref(false)

  // 可选设备：仅温度设备（EA2516T），从 DeviceService.GetDeviceProfiles 派生
  const tempDevices = ref<TempDeviceSummary[]>([])

  // 当前选中通道（用于面板下方表格区域显示，默认第一个勾选通道）
  const activeChannelIndex = ref<number>(-1)

  // 派生：所有温度设备 profile 列表
  async function refreshProfiles() {
    const profiles = await DeviceService.GetDeviceProfiles() as RawProfile[]
    tempDevices.value = profiles
      .filter(p => p.type === 'EA2516T')
      .map(p => ({ id: p.id, name: p.name, type: p.type }))
    // 若当前选中设备不在列表中，自动切到第一个
    if (tempDevices.value.length > 0) {
      const exists = tempDevices.value.some(d => d.id === selectedDeviceId.value)
      if (!exists) {
        await doSelectDevice(tempDevices.value[0].id)
      } else {
        await loadChannels()
      }
    } else {
      selectedDeviceId.value = ''
      channels.value = []
      selectedChannelIndices.value = []
    }
  }

  // selectDevice 不做去重短路：el-select 用单向 :value + @change，调用方已确保 id 变化。
  // 即使重复调用同一 id 也只是清空草稿+重载通道，行为可预测。
  async function selectDevice(id: string) {
    if (!id) return
    await doSelectDevice(id)
  }

  // 实际执行切换：清空草稿 → load channels
  async function doSelectDevice(id: string) {
    selectedDeviceId.value = id
    // 切换设备时清空所有草稿（避免跨设备语义混淆）
    pointsByChannel.value = new Map()
    fitResultByChannel.value = new Map()
    selectedChannelIndices.value = []
    activeChannelIndex.value = -1
    await loadChannels()
  }

  async function loadChannels() {
    if (!selectedDeviceId.value) {
      channels.value = []
      return
    }
    const profiles = await DeviceService.GetDeviceProfiles() as RawProfile[]
    const profile = profiles.find(p => p.id === selectedDeviceId.value)
    if (!profile) {
      channels.value = []
      return
    }
    channels.value = (profile.channels || [])
      .filter((c: RawChannel) => c.enabled)
      .map((c: RawChannel) => ({
        index: c.index,
        name: c.name,
        enabled: c.enabled,
        thermocoupleType: c.thermocoupleType,
        tempCalibA: c.tempCalibA,
        tempCalibB: c.tempCalibB,
        tempCalibR2: c.tempCalibR2,
        tempCalibPoints: c.tempCalibPoints,
        tempCalibratedAt: c.tempCalibratedAt,
      }))
    // 若当前 activeChannelIndex 不在新通道列表中，重置为 -1
    if (activeChannelIndex.value >= 0 && !channels.value.some(c => c.index === activeChannelIndex.value)) {
      activeChannelIndex.value = -1
    }
  }

  // 派生：activeChannel 的点表格
  const activePoints = computed<TempPointRow[]>(() => {
    if (activeChannelIndex.value < 0) return []
    return pointsByChannel.value.get(activeChannelIndex.value) ?? []
  })

  // 派生：activeChannel 的拟合结果
  const activeFitResult = computed<types.TempCalibResult | null>(() => {
    if (activeChannelIndex.value < 0) return null
    return fitResultByChannel.value.get(activeChannelIndex.value) ?? null
  })

  // 派生：activeChannel 已持久化的校准信息
  const activePersisted = computed<TempChannelInfo | null>(() => {
    if (activeChannelIndex.value < 0) return null
    return channels.value.find(c => c.index === activeChannelIndex.value) ?? null
  })

  // 派生：每通道的点数进度
  function pointsProgress(channelIndex: number): { current: number; total: number } {
    const rows = pointsByChannel.value.get(channelIndex) ?? []
    return { current: rows.length, total: pointCount.value }
  }

  // 设置 active 通道
  function setActiveChannel(channelIndex: number) {
    activeChannelIndex.value = channelIndex
  }

  // 设置勾选通道（替代组件直接写 selectedChannelIndices）
  function setSelectedChannels(indices: number[]) {
    selectedChannelIndices.value = indices
    // 若当前 active 通道被取消勾选，自动选第一个
    if (activeChannelIndex.value < 0 && indices.length > 0) {
      activeChannelIndex.value = indices[0]
    }
  }

  // 设置点数（替代组件直接写 pointCount）
  function setPointCount(n: number) {
    if (n < MIN_POINTS || n > MAX_POINTS) return
    pointCount.value = n
  }

  // 内部：设置某通道点表格（响应式触发）
  function setPoints(channelIndex: number, rows: TempPointRow[]) {
    pointsByChannel.value.set(channelIndex, rows)
    triggerRef(pointsByChannel)
  }

  // 内部：使某通道拟合结果失效（点变了，旧结果作废）
  function invalidateFit(channelIndex: number) {
    fitResultByChannel.value.set(channelIndex, null)
    triggerRef(fitResultByChannel)
  }

  // 内部：使 active 通道拟合结果失效（点变了，旧结果作废）
  function invalidateActiveFit() {
    if (activeChannelIndex.value < 0) return
    invalidateFit(activeChannelIndex.value)
  }

  // 内部：设置某通道拟合结果
  function setFitResult(channelIndex: number, result: types.TempCalibResult | null) {
    fitResultByChannel.value.set(channelIndex, result)
    triggerRef(fitResultByChannel)
  }

  // 批量采点：调用后端 TempCalibSample，结果追加到每勾选通道的点表格
  async function samplePoints(reference: number): Promise<ActionResult> {
    if (!selectedDeviceId.value) {
      return { success: false, message: '请先选择温度设备' }
    }
    if (selectedChannelIndices.value.length === 0) {
      return { success: false, message: '请至少勾选一个通道' }
    }
    sampling.value = true
    try {
      const results = await DeviceService.TempCalibSample(
        selectedDeviceId.value,
        selectedChannelIndices.value,
      ) as types.TempCalibSampleResult[]
      let okCount = 0
      let errMsg = ''
      for (const r of results) {
        if (r.error) {
          errMsg += `CH${r.channelIndex}: ${r.error}; `
          continue
        }
        const rows = pointsByChannel.value.get(r.channelIndex) ?? []
        // 采点上限：不允许超过 pointCount
        if (rows.length >= pointCount.value) {
          errMsg += `CH${r.channelIndex} 已采满 ${pointCount.value} 点; `
          continue
        }
        rows.push({ measured: r.meanValue, reference })
        setPoints(r.channelIndex, rows)
        // 采到新点后清除该通道的旧预演结果（点变了，旧结果失效）
        invalidateFit(r.channelIndex)
        okCount++
      }
      // 自动选中第一个采到点的通道
      if (activeChannelIndex.value < 0 && okCount > 0) {
        for (const idx of selectedChannelIndices.value) {
          if ((pointsByChannel.value.get(idx) ?? []).length > 0) {
            activeChannelIndex.value = idx
            break
          }
        }
      }
      if (okCount === 0) {
        return { success: false, message: errMsg || '采点失败' }
      }
      return { success: true, message: okCount === results.length ? `已采点 ${okCount} 通道` : `已采点 ${okCount}/${results.length} 通道，${errMsg}` }
    } catch (e: unknown) {
      const msg = e instanceof Error ? e.message : String(e)
      return { success: false, message: `采点失败: ${msg}` }
    } finally {
      sampling.value = false
    }
  }

  // 删除单点
  function removePoint(channelIndex: number, rowIndex: number) {
    const rows = pointsByChannel.value.get(channelIndex) ?? []
    rows.splice(rowIndex, 1)
    setPoints(channelIndex, rows)
    invalidateFit(channelIndex)
  }

  // 拟合当前 active 通道
  async function fitActive(): Promise<ActionResult> {
    if (activeChannelIndex.value < 0) {
      return { success: false, message: '请先选择通道' }
    }
    const rows = pointsByChannel.value.get(activeChannelIndex.value) ?? []
    if (rows.length < MIN_POINTS) {
      return { success: false, message: `至少需要 ${MIN_POINTS} 个点才能拟合，当前 ${rows.length} 个` }
    }
    try {
      const points: types.TempCalibPoint[] = rows.map(r => types.TempCalibPoint.createFrom({
        measured: r.measured,
        reference: r.reference,
      }))
      const result = await DeviceService.TempCalibFit(points) as types.TempCalibResult
      setFitResult(activeChannelIndex.value, result)
      let warn = ''
      if (result.r2 < 0.99) {
        warn = `（R²=${result.r2.toFixed(4)} < 0.99，请检查点质量）`
      }
      return { success: true, message: `拟合完成：a=${result.a.toFixed(6)} b=${result.b.toFixed(6)} R²=${result.r2.toFixed(4)}${warn}` }
    } catch (e: unknown) {
      const msg = e instanceof Error ? e.message : String(e)
      return { success: false, message: `拟合失败: ${msg}` }
    }
  }

  // 写入当前 active 通道的拟合结果到后端
  async function writeActive(): Promise<ActionResult> {
    if (activeChannelIndex.value < 0) {
      return { success: false, message: '请先选择通道' }
    }
    const result = fitResultByChannel.value.get(activeChannelIndex.value)
    if (!result) {
      return { success: false, message: '请先点拟合按钮' }
    }
    try {
      await DeviceService.TempCalibWrite(selectedDeviceId.value, activeChannelIndex.value, result)
      // 写入成功后刷新 channels（读取持久化的 tempCalibAt 等字段）
      await loadChannels()
      return { success: true, message: `通道 ${activeChannelIndex.value} 校准已写入` }
    } catch (e: unknown) {
      const msg = e instanceof Error ? e.message : String(e)
      return { success: false, message: `写入失败: ${msg}` }
    }
  }

  // 清除指定通道的校准（不依赖 active 状态）
  async function clearChannel(channelIndex: number): Promise<ActionResult> {
    if (channelIndex < 0) {
      return { success: false, message: '通道索引无效' }
    }
    try {
      await DeviceService.ClearTempCalib(selectedDeviceId.value, channelIndex)
      await loadChannels()
      invalidateFit(channelIndex)
      return { success: true, message: `通道 ${channelIndex} 已去校准` }
    } catch (e: unknown) {
      const msg = e instanceof Error ? e.message : String(e)
      return { success: false, message: `去校准失败: ${msg}` }
    }
  }

  // 清除所有通道的校准
  async function clearAll(): Promise<ActionResult> {
    if (!selectedDeviceId.value) {
      return { success: false, message: '请先选择设备' }
    }
    try {
      await DeviceService.ClearAllTempCalib(selectedDeviceId.value)
      await loadChannels()
      // 清空所有通道的预演结果
      fitResultByChannel.value = new Map()
      triggerRef(fitResultByChannel)
      return { success: true, message: '所有通道已去校准' }
    } catch (e: unknown) {
      const msg = e instanceof Error ? e.message : String(e)
      return { success: false, message: `去校准失败: ${msg}` }
    }
  }

  return {
    // state
    selectedDeviceId,
    channels,
    selectedChannelIndices,
    pointCount,
    pointsByChannel,
    fitResultByChannel,
    sampling,
    tempDevices,
    activeChannelIndex,
    // computed
    activePoints,
    activeFitResult,
    activePersisted,
    // actions
    refreshProfiles,
    selectDevice,
    loadChannels,
    setActiveChannel,
    setSelectedChannels,
    setPointCount,
    pointsProgress,
    samplePoints,
    removePoint,
    fitActive,
    writeActive,
    clearChannel,
    clearAll,
    // 内部 action 暴露给组件
    invalidateActiveFit,
  }
})
