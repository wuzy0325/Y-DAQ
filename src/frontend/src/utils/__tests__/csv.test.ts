import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { downloadCSV } from '../csv'

describe('utils/csv', () => {
  let createObjectURLSpy: ReturnType<typeof vi.spyOn>
  let revokeObjectURLSpy: ReturnType<typeof vi.spyOn>
  let clickSpy: ReturnType<typeof vi.spyOn>

  beforeEach(() => {
    // mock URL API
    createObjectURLSpy = vi.spyOn(URL, 'createObjectURL').mockReturnValue('blob:mock-url')
    revokeObjectURLSpy = vi.spyOn(URL, 'revokeObjectURL').mockImplementation(() => {})

    // mock <a>.click() —— happy-dom 默认无 click 行为
    clickSpy = vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => {})
  })

  afterEach(() => {
    // 关键：还原所有 spyOn，避免 spy 在测试间叠加导致 mock 状态泄漏
    vi.restoreAllMocks()
  })

  it('调用 createObjectURL + click + revokeObjectURL', () => {
    downloadCSV('test', ['a', 'b'], [[1, 2]])
    expect(createObjectURLSpy).toHaveBeenCalledTimes(1)
    expect(clickSpy).toHaveBeenCalledTimes(1)
    expect(revokeObjectURLSpy).toHaveBeenCalledTimes(1)
  })

  it('文件名格式：{prefix}-{ISO时间，冒号替换为短横}.csv', () => {
    downloadCSV('recording', ['h'], [[1]])
    const anchor = clickSpy.mock.instances[0] as HTMLAnchorElement
    expect(anchor.download).toMatch(/^recording-\d{4}-\d{2}-\d{2}T\d{2}-\d{2}-\d{2}\.csv$/)
    expect(anchor.href).toBe('blob:mock-url')
  })

  it('CSV 内容包含 BOM + 表头 + 数据行', () => {
    downloadCSV('test', ['Timestamp', 'Value'], [['2024-01-01', 42.5]])
    const blob = createObjectURLSpy.mock.calls[0][0] as Blob
    // 注意：happy-dom 的 Blob 是异步读取，但 Blob.text() 返回 Promise
    return blob.text().then((text) => {
      expect(text.startsWith('\uFEFF')).toBe(true) // BOM
      expect(text).toContain('Timestamp,Value')
      expect(text).toContain('2024-01-01,42.5')
    })
  })

  it('多行数据应按行拼接', () => {
    downloadCSV('test', ['h1', 'h2'], [[1, 'a'], [2, 'b'], [3, 'c']])
    const blob = createObjectURLSpy.mock.calls[0][0] as Blob
    return blob.text().then((text) => {
      // BOM + 表头 + 3 行数据 = 4 行
      const lines = text.replace(/^\uFEFF/, '').split('\n')
      expect(lines).toHaveLength(4)
      expect(lines[0]).toBe('h1,h2')
      expect(lines[1]).toBe('1,a')
      expect(lines[2]).toBe('2,b')
      expect(lines[3]).toBe('3,c')
    })
  })

  it('空数据也应生成 CSV（仅表头）', () => {
    downloadCSV('empty', ['only-header'], [])
    const blob = createObjectURLSpy.mock.calls[0][0] as Blob
    return blob.text().then((text) => {
      expect(text.replace(/^\uFEFF/, '')).toBe('only-header\n')
    })
  })
})
