import { describe, expect, it } from 'vitest'
import { openLine, openPercent } from './openprogress'

describe('ход открытия альбома (073)', () => {
  it('«Раскладки: 120 из 200» и доля', () => {
    expect(openLine({ done: 120, total: 200 })).toBe('Раскладки: 120 из 200')
    expect(openPercent({ done: 120, total: 200 })).toBe(60)
    expect(openPercent({ done: 200, total: 200 })).toBe(100)
  })
  it('хода нет или total неизвестен — запасная строка, полоса неопределённая', () => {
    expect(openLine(null)).toBe('Считаем раскладки…')
    expect(openLine({ done: 0, total: 0 })).toBe('Считаем раскладки…')
    expect(openPercent(null)).toBeNull()
    expect(openPercent({ done: 0, total: 0 })).toBeNull()
  })
  it('перелёт done > total не выводит за 100 и за total', () => {
    expect(openPercent({ done: 205, total: 200 })).toBe(100)
    expect(openLine({ done: 205, total: 200 })).toBe('Раскладки: 200 из 200')
  })
})
