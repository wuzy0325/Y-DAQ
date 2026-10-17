import { describe, it, expect } from 'vitest'
import { formatDuration, formatSpan, formatSamplingRate } from '../formatDuration'

describe('utils/formatDuration', () => {
  describe('formatDuration', () => {
    it('毫秒 → 秒（不足 1 分钟）', () => {
      expect(formatDuration(0)).toBe('--')
      expect(formatDuration(1)).toBe('0秒') // 四舍五入到秒
      expect(formatDuration(999)).toBe('1秒')
      expect(formatDuration(45000)).toBe('45秒')
    })

    it('分钟档补零到秒', () => {
      expect(formatDuration(60000)).toBe('1分00秒')
      expect(formatDuration(125000)).toBe('2分05秒')
    })

    it('小时档补零到分秒', () => {
      expect(formatDuration(3600000)).toBe('1时00分00秒')
      expect(formatDuration(3725000)).toBe('1时02分05秒')
    })

    it('空值/非法值返回占位符', () => {
      expect(formatDuration(null)).toBe('--')
      expect(formatDuration(undefined)).toBe('--')
      expect(formatDuration(-1)).toBe('--')
      expect(formatDuration(NaN)).toBe('--')
      expect(formatDuration(Infinity)).toBe('--')
    })
  })

  describe('formatSpan', () => {
    it('1 分钟内保留 1 位小数', () => {
      expect(formatSpan(0)).toBe('0.0s')
      expect(formatSpan(500)).toBe('0.5s')
      expect(formatSpan(12345)).toBe('12.3s')
    })

    it('≥1 分钟复用 formatDuration', () => {
      expect(formatSpan(60000)).toBe('1分00秒')
      expect(formatSpan(185000)).toBe('3分05秒')
    })

    it('非法值返回占位符', () => {
      expect(formatSpan(null)).toBe('--')
      expect(formatSpan(-5)).toBe('--')
      expect(formatSpan(NaN)).toBe('--')
    })
  })

  describe('formatSamplingRate', () => {
    it('整数频率不带小数', () => {
      expect(formatSamplingRate(20)).toBe('20 Hz')
      expect(formatSamplingRate(1000)).toBe('1000 Hz')
    })

    it('非整数频率保留 1 位小数', () => {
      expect(formatSamplingRate(16.6667)).toBe('16.7 Hz')
    })

    it('非法值返回占位符', () => {
      expect(formatSamplingRate(0)).toBe('--')
      expect(formatSamplingRate(null)).toBe('--')
      expect(formatSamplingRate(NaN)).toBe('--')
    })
  })
})
