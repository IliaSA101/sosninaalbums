import { describe, expect, it } from 'vitest'
import { countLabel } from './topbar'

describe('сводка в шапке', () => {
  const album = { sheet: false }
  it('склоняет разворот', () => {
    expect(countLabel(1, 2, album)).toBe('1 разворот (2 стр.)')
    expect(countLabel(2, 4, album)).toBe('2 разворота (4 стр.)')
    expect(countLabel(5, 10, album)).toBe('5 разворотов (10 стр.)')
    expect(countLabel(11, 22, album)).toBe('11 разворотов (22 стр.)')
    expect(countLabel(21, 42, album)).toBe('21 разворот (42 стр.)')
  })
  it('у листа — «листа», без страниц', () => {
    expect(countLabel(3, 3, { sheet: true })).toBe('3 листа')
    expect(countLabel(1, 1, { sheet: true })).toBe('1 лист')
  })
})
