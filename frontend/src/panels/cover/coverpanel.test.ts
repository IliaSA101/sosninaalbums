import { describe, expect, it } from 'vitest'
import type { CoverPreset, CoverSpec, CoverView } from '../../base/core/types'
import { customSpec, fieldValue, hasTable, overLine, parseNum, presetGroups, sideSize, specWith, spineLine, totalLine } from './coverpanel'

const spec = (o: Partial<CoverSpec> = {}): CoverSpec => ({
  preset: 'fsfera-thin',
  wrap: 23.5,
  steps: [
    { upto: 5, mm: 6 },
    { upto: 57, mm: 30 },
  ],
  ...o,
})
const preset = (id: string, group: string): CoverPreset => ({ id, name: id, group, spec: spec({ preset: id }) })
// Сценарий из спеки: Фотосфера, тонкие листы, 20 разворотов — 623 × 331 мм, при 300 dpi 7358 × 3909 px.
const cv = (o: Partial<CoverView> = {}) =>
  ({
    sheet: { w: 623, h: 331 },
    spec: spec(),
    spine: 14,
    n: 20,
    over: false,
    pxW: 7358,
    pxH: 3909,
    zones: [
      { kind: 'back', rect: { x: 23.5, y: 23.5, w: 281, h: 284 }, label: 'Задняя' },
      { kind: 'front', rect: { x: 318.5, y: 23.5, w: 281, h: 284 }, label: 'Лицевая' },
    ],
    ...o,
  }) as unknown as CoverView

describe('группы типографий', () => {
  it('в порядке появления, пресеты группы вместе', () => {
    const g = presetGroups([preset('a', 'Фотосфера'), preset('b', 'Нетпринт'), preset('c', 'Фотосфера'), preset('custom', 'Свои размеры')])
    expect(g.map((x) => x.group)).toEqual(['Фотосфера', 'Нетпринт', 'Свои размеры'])
    expect(g[0].items.map((p) => p.id)).toEqual(['a', 'c'])
  })
  it('пустой список — нет групп', () => {
    expect(presetGroups([])).toEqual([])
  })
})

describe('строки карточки', () => {
  it('итог по сценарию 11', () => {
    expect(totalLine(cv(), 300)).toBe('Итог: 623 × 331 мм — 7358 × 3909 px при 300 dpi')
  })
  it('дробные миллиметры — с запятой', () => {
    expect(totalLine(cv({ sheet: { w: 618.5, h: 326 } as never }), 300)).toContain('618,5 × 326 мм')
  })
  it('корешок при n разворотах: падеж по числу', () => {
    expect(spineLine(cv())).toBe('Корешок 14 мм при 20 разворотах')
    expect(spineLine(cv({ n: 1, spine: 6 }))).toBe('Корешок 6 мм при 1 развороте')
    expect(spineLine(cv({ n: 21, spine: 13.2 }))).toBe('Корешок 13,2 мм при 21 развороте')
    expect(spineLine(cv({ n: 22 }))).toBe('Корешок 14 мм при 22 разворотах')
  })
  it('«вне таблицы» — только при over и с последней строкой таблицы', () => {
    expect(overLine(cv())).toBe('')
    expect(overLine(cv({ over: true }))).toBe('Больше 57 разворотов — вне таблицы типографии')
    expect(overLine(cv({ over: true, spec: spec({ steps: [{ upto: 21, mm: 10 }] }) }))).toBe('Больше 21 разворота — вне таблицы типографии')
    expect(overLine(cv({ over: true, spec: spec({ steps: [] }) }))).toBe('')
  })
})

describe('поля размеров', () => {
  it('сторонка — из зоны Go, а не из spec (0 = «по странице»)', () => {
    expect(sideSize(cv())).toEqual({ w: 281, h: 284 })
    expect(fieldValue(cv(), 'w')).toBe(281)
    expect(fieldValue(cv(), 'h')).toBe(284)
    expect(fieldValue(cv(), 'wrap')).toBe(23.5)
    expect(fieldValue(cv(), 'hinge')).toBe(0)
  })
  it('нет зон — число из spec', () => {
    expect(fieldValue(cv({ zones: [], spec: spec({ w: 250 }) }), 'w')).toBe(250)
  })
  it('разбор ввода: запятая, пусто, мусор, минус', () => {
    expect(parseNum('9,5')).toBe(9.5)
    expect(parseNum(' 15 ')).toBe(15)
    expect(parseNum('0')).toBe(0)
    expect(parseNum('')).toBeNull()
    expect(parseNum('abc')).toBeNull()
    expect(parseNum('-3')).toBeNull()
  })
  it('правка поля не трогает остальное', () => {
    expect(specWith(spec(), 'wrap', 20)).toEqual({ ...spec(), wrap: 20 })
  })
  it('то же число — правки нет', () => {
    expect(specWith(spec(), 'wrap', 23.5)).toBeNull()
    expect(specWith(spec(), 'hinge', 0)).toBeNull()
  })
  it('a и b переводят корешок на формулу', () => {
    const s = specWith(spec(), 'b', 0.4)!
    expect(s.steps).toEqual([])
    expect(s.b).toBe(0.4)
    expect(hasTable(s)).toBe(false)
    expect(hasTable(spec())).toBe(true)
  })
})

describe('«Свои размеры»', () => {
  it('из таблицы: формула a = корешок сейчас, b = 0, steps пусты; остальные числа остаются', () => {
    const s = customSpec(cv({ spec: spec({ wrap: 23.5, hinge: 6 }), spine: 14 }))
    expect(s).toEqual({ ...spec({ wrap: 23.5, hinge: 6 }), preset: 'custom', steps: [], a: 14, b: 0 })
    expect(hasTable(s)).toBe(false)
  })
  it('из формулы: числа остаются, меняется только пометка пресета', () => {
    const f = spec({ preset: 'netprint', steps: [], a: 3, b: 0.7 })
    expect(customSpec(cv({ spec: f, spine: 17 }))).toEqual({ ...f, preset: 'custom' })
  })
})
