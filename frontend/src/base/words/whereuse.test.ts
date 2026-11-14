import { describe, expect, it } from 'vitest'
import { lineOf, lineText, useLines, type UseSrc } from './whereuse'

const mk = (n: number): UseSrc[] =>
  Array.from({ length: n }, (_, i) => ({
    sheet: i + 1,
    kind: 'backdrop',
    spread: `s${i}`,
    page: `Разворот ${i + 1}`,
    what: 'фон',
    pick: 'spread',
  }))

describe('useLines', () => {
  it('не больше max строк, остальные — «ещё N»', () => {
    const r = useLines(mk(9), 6)
    expect(r.shown).toHaveLength(6)
    expect(r.more).toBe(3)
  })
  it('ровно max и меньше — без «ещё»; пусто — пусто', () => {
    expect(useLines(mk(6), 6).more).toBe(0)
    expect(useLines(mk(1), 6).shown).toHaveLength(1)
    expect(useLines([], 6)).toEqual({ shown: [], more: 0 })
  })
  it('порядок Go сохраняется', () => {
    expect(useLines(mk(3), 10).shown.map((l) => l.page)).toEqual(['Разворот 1', 'Разворот 2', 'Разворот 3'])
  })
})

describe('lineOf и lineText', () => {
  it('поля берутся как есть, без разбора подписи; нет pick — пустая строка', () => {
    const l = lineOf({ sheet: -1, kind: 'albumBg', spread: '', page: 'Фон альбома', what: '' })
    expect(l).toEqual({ kind: 'albumBg', spread: '', sheet: -1, page: 'Фон альбома', what: '', pick: '' })
  })
  it('текст строки: с видом через тире, без вида — только страница', () => {
    expect(lineText({ page: 'Обложка', what: 'место' })).toBe('Обложка — место')
    expect(lineText({ page: 'Фон альбома', what: '' })).toBe('Фон альбома')
  })
})
