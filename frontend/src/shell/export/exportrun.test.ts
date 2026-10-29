import { describe, expect, it } from 'vitest'
import { runLine, runPercent } from './exportrun'

describe('ход выгрузки (073)', () => {
  it('«Выгружено 3 из 12» и доля', () => {
    expect(runLine({ done: 3, total: 12 })).toBe('Выгружено 3 из 12')
    expect(runPercent({ done: 3, total: 12 })).toBe(25)
    expect(runPercent({ done: 12, total: 12 })).toBe(100)
  })
  it('total ещё неизвестен или хода нет — «Готовим…», полоса неопределённая', () => {
    expect(runLine({ done: 0, total: 0 })).toBe('Готовим…')
    expect(runPercent({ done: 0, total: 0 })).toBeNull()
    expect(runLine(null)).toBe('Готовим…')
    expect(runPercent(null)).toBeNull()
  })
  it('перелёт done > total не выводит долю за 100', () => {
    expect(runPercent({ done: 13, total: 12 })).toBe(100)
  })
})
