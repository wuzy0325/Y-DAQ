import { describe, it, expect } from 'vitest'
import {
  getAxisUnit,
  getAxisKindText,
  getRunStateText,
  createDefaultAxisConfig,
  createDefaultAxisUIState,
} from '../helpers'

describe('motion/helpers', () => {
  describe('getAxisUnit', () => {
    it('LINEAR → mm', () => {
      expect(getAxisUnit('LINEAR')).toBe('mm')
    })
    it('ROTARY → °', () => {
      expect(getAxisUnit('ROTARY')).toBe('°')
    })
  })

  describe('getAxisKindText', () => {
    it('LINEAR → 平移轴', () => {
      expect(getAxisKindText('LINEAR')).toBe('平移轴')
    })
    it('ROTARY → 旋转轴', () => {
      expect(getAxisKindText('ROTARY')).toBe('旋转轴')
    })
  })

  describe('getRunStateText', () => {
    it('idle → 空闲', () => {
      expect(getRunStateText('idle')).toBe('空闲')
    })
    it('running → 运行中', () => {
      expect(getRunStateText('running')).toBe('运行中')
    })
    it('jogging_minus → 反向点动', () => {
      expect(getRunStateText('jogging_minus')).toBe('反向点动')
    })
    it('jogging_plus → 正向点动', () => {
      expect(getRunStateText('jogging_plus')).toBe('正向点动')
    })
    it('error → 错误', () => {
      expect(getRunStateText('error')).toBe('错误')
    })
  })

  describe('createDefaultAxisConfig', () => {
    it('LINEAR 轴默认值', () => {
      const cfg = createDefaultAxisConfig('X', 'LINEAR')
      expect(cfg.name).toBe('X')
      expect(cfg.enabled).toBe(true)
      expect(cfg.kind).toBe('LINEAR')
      expect(cfg.inverted).toBe(false)
      expect(cfg.stepAngleDeg).toBe(1.8)
      expect(cfg.microSteps).toBe(16)
      expect(cfg.lead).toBe(5.0) // LINEAR 导程 5mm
      expect(cfg.gearRatio).toBe(1)
      expect(cfg.maxSpeed).toBe(50) // LINEAR 最大速度 50
      expect(cfg.encoderScale).toBe(0.005)
      expect(cfg.encoderCompensation.enabled).toBe(false)
      expect(cfg.encoderCompensation.tolerance).toBe(0.01)
      expect(cfg.encoderCompensation.maxCycles).toBe(3)
      expect(cfg.encoderCompensation.settleMs).toBe(100)
      expect(cfg.encoderCompensation.minStep).toBe(0)
      expect(cfg.encoderCompensation.timeoutMs).toBe(5000)
    })

    it('ROTARY 轴默认值（lead=0, maxSpeed=30）', () => {
      const cfg = createDefaultAxisConfig('U', 'ROTARY')
      expect(cfg.name).toBe('U')
      expect(cfg.kind).toBe('ROTARY')
      expect(cfg.lead).toBe(0) // ROTARY 导程 0
      expect(cfg.maxSpeed).toBe(30) // ROTARY 最大速度 30
    })
  })

  describe('createDefaultAxisUIState', () => {
    it('LINEAR 轴 UI 默认值', () => {
      const st = createDefaultAxisUIState('Y', 'LINEAR')
      expect(st.name).toBe('Y')
      expect(st.kind).toBe('LINEAR')
      expect(st.currentPosition).toBe(0)
      expect(st.targetPosition).toBe(0)
      expect(st.relativeDistance).toBe(10) // LINEAR 相对距离 10
      expect(st.runState).toBe('idle')
      expect(st.isHomed).toBe(false)
      expect(st.posLimitActive).toBe(false)
      expect(st.negLimitActive).toBe(false)
      expect(st.config.name).toBe('Y')
      expect(st.config.kind).toBe('LINEAR')
    })

    it('ROTARY 轴 UI 默认值（relativeDistance=5）', () => {
      const st = createDefaultAxisUIState('U', 'ROTARY')
      expect(st.relativeDistance).toBe(5) // ROTARY 相对距离 5
      expect(st.config.lead).toBe(0)
    })

    it('config 应与 createDefaultAxisConfig 一致', () => {
      const st = createDefaultAxisUIState('X', 'LINEAR')
      const cfg = createDefaultAxisConfig('X', 'LINEAR')
      expect(st.config).toEqual(cfg)
    })
  })
})
