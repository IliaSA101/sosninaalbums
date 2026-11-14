import { describe, expect, it } from 'vitest'
import { formatRanges, parseRanges } from './ranges'

describe('parseRanges', () => {
  it('«3, 5-8» -> номера', () => {
    expect(parseRanges('3, 5-8', 10)).toEqual({ nums: [3, 5, 6, 7, 8], error: null })
  })
  it('разделители и тире', () => {
    expect(parseRanges('1;2 4', 5).nums).toEqual([1, 2, 4])
    expect(parseRanges('2 – 4', 5).nums).toEqual([2, 3, 4])
    expect(parseRanges('2—3,', 5).nums).toEqual([2, 3])
  })
  it('повторы схлопываются, порядок по возрастанию', () => {
    expect(parseRanges('5, 3, 3-4, 4', 9).nums).toEqual([3, 4, 5])
  })
  it('один номер и граница', () => {
    expect(parseRanges('1', 1).nums).toEqual([1])
    expect(parseRanges('1-10', 10).nums).toHaveLength(10)
  })
  it('пустой ввод', () => {
    expect(parseRanges('', 5).error).toMatch(/Укажите/)
    expect(parseRanges('  ,, ', 5).nums).toEqual([])
  })
  it('мусор', () => {
    expect(parseRanges('a, 3', 5)).toEqual({ nums: [], error: 'Не понимаю «a»' })
    expect(parseRanges('3.5', 9).error).toMatch(/Не понимаю/)
    expect(parseRanges('-3', 9).error).toMatch(/Не понимаю/)
    expect(parseRanges('1-2-3', 9).error).toMatch(/Не понимаю/)
    expect(parseRanges('12345678901', 9).error).toMatch(/Не понимаю/)
  })
  it('пределы 1..N', () => {
    expect(parseRanges('0', 5).error).toBe('Номера начинаются с 1')
    expect(parseRanges('0-3', 5).error).toBe('Номера начинаются с 1')
    expect(parseRanges('3, 6', 5).error).toBe('Разворота 6 нет (всего 5)')
    expect(parseRanges('4-9', 5).error).toBe('Разворота 9 нет (всего 5)')
  })
  it('диапазон наоборот', () => {
    expect(parseRanges('8-5', 9).error).toBe('Диапазон «8-5» задан наоборот')
  })
  it('нет разворотов', () => {
    expect(parseRanges('1', 0).error).toBe('Нет разворотов')
  })
  it('при ошибке номеров нет', () => {
    expect(parseRanges('1, x', 5).nums).toEqual([])
  })
})

describe('formatRanges', () => {
  it('сворачивает подряд идущие от трёх', () => {
    expect(formatRanges([3, 5, 6, 7, 8])).toBe('3, 5-8')
    expect(formatRanges([1, 2])).toBe('1, 2')
    expect(formatRanges([4, 2, 3, 2])).toBe('2-4')
    expect(formatRanges([])).toBe('')
  })
  it('круг с parseRanges', () => {
    expect(parseRanges(formatRanges([1, 3, 4, 5, 9]), 9).nums).toEqual([1, 3, 4, 5, 9])
  })
})
