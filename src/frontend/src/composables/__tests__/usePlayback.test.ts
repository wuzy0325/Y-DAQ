import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { effectScope, defineComponent } from 'vue'
import { mount } from '@vue/test-utils'
import { usePlayback } from '../usePlayback'

describe('composables/usePlayback', () => {
  beforeEach(() => {
    vi.useFakeTimers()
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  function setup() {
    const scope = effectScope()
    const result = scope.run(() => usePlayback())!
    return { ...result, scope }
  }

  const CSV_CONTENT = [
    'Timestamp,DeviceID,ChannelIndex,ChannelName,Value,Unit',
    '2024-01-01 10:00:00.000,d1,0,CH1,100.5,Pa',
    '2024-01-01 10:00:00.000,d1,1,CH2,200.3,Pa',
    '2024-01-01 10:00:00.100,d1,0,CH1,101.5,Pa',
    '2024-01-01 10:00:00.100,d1,1,CH2,201.3,Pa',
  ].join('\n')

  describe('初始状态', () => {
    it('所有状态应为初始值', () => {
      const { playbackData, playbackIndex, isPlaying, playbackSpeed, playbackProgress, currentTimeLabel } = setup()
      expect(playbackData.value).toEqual([])
      expect(playbackIndex.value).toBe(0)
      expect(isPlaying.value).toBe(false)
      expect(playbackSpeed.value).toBe(1)
      expect(playbackProgress.value).toBe(0)
      expect(currentTimeLabel.value).toBe('--')
    })
  })

  describe('parseAndLoadCSV', () => {
    it('正常解析：4 行数据 → playbackData 长度 4', () => {
      const { playbackData, playbackIndex, isPlaying } = setup()
      // parseAndLoadCSV 在 setup 内调用
      const { parseAndLoadCSV } = setup()
      parseAndLoadCSV(CSV_CONTENT)
      // 重新 setup 后状态独立，需在同一实例验证
      const inst = setup()
      inst.parseAndLoadCSV(CSV_CONTENT)
      expect(inst.playbackData.value).toHaveLength(4)
      expect(inst.playbackIndex.value).toBe(0)
      expect(inst.isPlaying.value).toBe(false)
    })

    it('正确解析每行字段（timestamp/deviceId/channelIndex/channelName/value/unit）', () => {
      const inst = setup()
      inst.parseAndLoadCSV(CSV_CONTENT)
      const row0 = inst.playbackData.value[0]
      expect(row0.timestamp).toBe('2024-01-01 10:00:00.000')
      expect(row0.deviceId).toBe('d1')
      expect(row0.channelIndex).toBe(0)
      expect(row0.channelName).toBe('CH1')
      expect(row0.value).toBeCloseTo(100.5)
      expect(row0.unit).toBe('Pa')
    })

    it('处理 BOM 头（0xFEFF）', () => {
      const inst = setup()
      const bomContent = '\uFEFF' + CSV_CONTENT
      inst.parseAndLoadCSV(bomContent)
      expect(inst.playbackData.value).toHaveLength(4)
    })

    it('空内容：不修改状态', () => {
      const inst = setup()
      inst.parseAndLoadCSV('')
      expect(inst.playbackData.value).toEqual([])
    })

    it('仅表头（无数据行）：不修改状态', () => {
      const inst = setup()
      inst.parseAndLoadCSV('Timestamp,DeviceID,ChannelIndex,ChannelName,Value,Unit')
      expect(inst.playbackData.value).toEqual([])
    })

    it('过滤空行', () => {
      const inst = setup()
      const contentWithEmptyLines = CSV_CONTENT + '\n\n\n'
      inst.parseAndLoadCSV(contentWithEmptyLines)
      expect(inst.playbackData.value).toHaveLength(4)
    })

    it('列数 < 6 的行被跳过', () => {
      const inst = setup()
      const content = [
        'Timestamp,DeviceID,ChannelIndex,ChannelName,Value,Unit',
        '2024-01-01,d1,0,CH1,100.5,Pa',  // 6 列 ✓
        'incomplete,row',                 // 2 列 ✗
        '2024-01-01,d1,1,CH2,200.3,Pa',  // 6 列 ✓
      ].join('\n')
      inst.parseAndLoadCSV(content)
      expect(inst.playbackData.value).toHaveLength(2)
    })

    it('非数字 channelIndex/value → 0', () => {
      const inst = setup()
      const content = [
        'Timestamp,DeviceID,ChannelIndex,ChannelName,Value,Unit',
        '2024-01-01,d1,abc,CH1,xyz,Pa',
      ].join('\n')
      inst.parseAndLoadCSV(content)
      expect(inst.playbackData.value[0].channelIndex).toBe(0)
      expect(inst.playbackData.value[0].value).toBe(0)
    })
  })

  describe('playback 控制', () => {
    it('startPlayback：无数据时不启动', () => {
      const inst = setup()
      inst.startPlayback()
      expect(inst.isPlaying.value).toBe(false)
    })

    it('startPlayback：有数据时 isPlaying=true', () => {
      const inst = setup()
      inst.parseAndLoadCSV(CSV_CONTENT)
      inst.startPlayback()
      expect(inst.isPlaying.value).toBe(true)
    })

    it('startPlayback 后推进 timer：playbackIndex 递增', () => {
      const inst = setup()
      inst.parseAndLoadCSV(CSV_CONTENT)
      inst.startPlayback()
      // 4 条数据，索引 0→1→2→3
      vi.advanceTimersByTime(50) // 1 tick
      expect(inst.playbackIndex.value).toBe(1)
      vi.advanceTimersByTime(50)
      expect(inst.playbackIndex.value).toBe(2)
    })

    it('到末尾自动 pausePlayback', () => {
      const inst = setup()
      inst.parseAndLoadCSV(CSV_CONTENT)
      inst.startPlayback()
      // 4 条数据：50ms→idx1, 100ms→idx2, 150ms→idx3, 200ms→idx==len-1 触发 pause
      vi.advanceTimersByTime(200)
      expect(inst.playbackIndex.value).toBe(3)
      expect(inst.isPlaying.value).toBe(false)
    })

    it('pausePlayback：停止 timer + isPlaying=false', () => {
      const inst = setup()
      inst.parseAndLoadCSV(CSV_CONTENT)
      inst.startPlayback()
      vi.advanceTimersByTime(50)
      inst.pausePlayback()
      expect(inst.isPlaying.value).toBe(false)
      // 再推进时间，index 不变
      vi.advanceTimersByTime(200)
      expect(inst.playbackIndex.value).toBe(1)
    })

    it('togglePlayback：未播放 → 播放', () => {
      const inst = setup()
      inst.parseAndLoadCSV(CSV_CONTENT)
      inst.togglePlayback()
      expect(inst.isPlaying.value).toBe(true)
    })

    it('togglePlayback：播放中 → 暂停', () => {
      const inst = setup()
      inst.parseAndLoadCSV(CSV_CONTENT)
      inst.startPlayback()
      inst.togglePlayback()
      expect(inst.isPlaying.value).toBe(false)
    })

    it('resetPlayback：回到 index=0 + 暂停', () => {
      const inst = setup()
      inst.parseAndLoadCSV(CSV_CONTENT)
      inst.startPlayback()
      vi.advanceTimersByTime(100)
      inst.resetPlayback()
      expect(inst.playbackIndex.value).toBe(0)
      expect(inst.isPlaying.value).toBe(false)
    })

    it('playbackSpeed 影响间隔（speed=2 → 间隔 25ms）', () => {
      const inst = setup()
      inst.parseAndLoadCSV(CSV_CONTENT)
      inst.playbackSpeed.value = 2
      inst.startPlayback()
      // 25ms 后应已推进 1 步
      vi.advanceTimersByTime(25)
      expect(inst.playbackIndex.value).toBe(1)
      // 50ms 时（原 25ms 的 2 倍），应推进 2 步
      vi.advanceTimersByTime(25)
      expect(inst.playbackIndex.value).toBe(2)
    })
  })

  describe('computed 属性', () => {
    it('playbackProgress：0% → 100% 随 index 变化', () => {
      const inst = setup()
      inst.parseAndLoadCSV(CSV_CONTENT)
      expect(inst.playbackProgress.value).toBe(0) // index=0, 4 条数据
      inst.playbackIndex.value = 1
      expect(inst.playbackProgress.value).toBe(Math.round((1 / 3) * 100)) // 33
      inst.playbackIndex.value = 2
      expect(inst.playbackProgress.value).toBe(Math.round((2 / 3) * 100)) // 67
      inst.playbackIndex.value = 3
      expect(inst.playbackProgress.value).toBe(100)
    })

    it('currentTimeLabel：返回当前行 timestamp', () => {
      const inst = setup()
      inst.parseAndLoadCSV(CSV_CONTENT)
      expect(inst.currentTimeLabel.value).toBe('2024-01-01 10:00:00.000')
      inst.playbackIndex.value = 2
      expect(inst.currentTimeLabel.value).toBe('2024-01-01 10:00:00.100')
    })

    it('playbackChartOption：无数据时返回等待提示', () => {
      const inst = setup()
      const opt = inst.playbackChartOption.value as any
      expect(opt.backgroundColor).toBe('transparent')
      expect(opt.title.text).toContain('等待回放数据')
    })

    it('playbackChartOption：有数据时返回 series + xAxis', () => {
      const inst = setup()
      inst.parseAndLoadCSV(CSV_CONTENT)
      const opt = inst.playbackChartOption.value as any
      expect(opt.series).toBeDefined()
      expect(opt.series.length).toBeGreaterThan(0)
      expect(opt.xAxis).toBeDefined()
    })
  })

  describe('生命周期', () => {
    it('unmount 后清理 timer（onUnmounted 触发 clearInterval）', () => {
      // onUnmounted 仅在组件上下文触发，effectScope 中不会触发
      // 因此用真实组件 mount/unmount 验证清理逻辑
      const TestComp = defineComponent({
        setup() {
          const playback = usePlayback()
          return { ...playback }
        },
      })
      const wrapper = mount(TestComp)
      const vm = wrapper.vm as any
      vm.parseAndLoadCSV(CSV_CONTENT)
      vm.startPlayback()
      // 卸载前 timer 应在运行
      vi.advanceTimersByTime(50)
      expect(vm.playbackIndex).toBe(1)
      // 卸载 —— 触发 onUnmounted → clearInterval
      wrapper.unmount()
      // 推进时间，index 不应再变化
      vi.advanceTimersByTime(500)
      expect(vm.playbackIndex).toBe(1)
    })
  })
})
