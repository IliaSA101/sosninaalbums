import { describe, expect, it } from 'vitest'
import type { CoverSide } from '../../base/types/print'
import { barGroups, coverAct, groupLeft, NO_OTHER, openN, sideWho } from './coverbar'

const side = (o: Partial<CoverSide> = {}): CoverSide => ({
  scope: 'right',
  label: 'Лицевая',
  box: { x: 200, y: 30, w: 100, h: 200 },
  places: 3,
  known: true,
  alone: false,
  ...o,
})

describe('полоса сторонок обложки: группы', () => {
  it('по одной группе на сторонку из Sides, в порядке Go; левая — «задней»', () => {
    const g = barGroups([side({ scope: 'left', label: 'Задняя', box: { x: 10, y: 30, w: 100, h: 200 } }), side()], 320)
    expect(g.map((x) => x.side.scope)).toEqual(['left', 'right'])
    expect(g.map((x) => x.who)).toEqual(['задней', 'лицевой'])
    expect(sideWho('right')).toBe('лицевой')
  })
  it('пустая зона — группы нет (Go не отдаёт сторонку); нет Sides — пусто', () => {
    expect(barGroups([side()], 320)).toHaveLength(1)
    expect(barGroups([], 320)).toEqual([])
    expect(barGroups(undefined, 320)).toEqual([])
  })
  it('центр группы — центр безопасной зоны в % ширины листа', () => {
    expect(groupLeft(side(), 320)).toBeCloseTo(((200 + 50) / 320) * 100, 6)
    expect(barGroups([side()], 320)[0].left).toBeCloseTo(78.125, 6)
  })
  it('ширина листа 0 — по центру, без NaN', () => {
    expect(groupLeft(side(), 0)).toBe(50)
  })
  it('«· Ручная ·» только при местах и !known; без мест и при known — нет', () => {
    const by = (o: Partial<CoverSide>) => barGroups([side(o)], 320)[0].manual
    expect(by({ known: false })).toBe(true)
    expect(by({ known: true })).toBe(false)
    expect(by({ known: false, places: 0 })).toBe(false)
  })
  it('alone (форма узнана, раскладка одна) — стрелки недоступны; без known не бывает', () => {
    const by = (o: Partial<CoverSide>) => barGroups([side(o)], 320)[0].still
    expect(by({ alone: true })).toBe(true)
    expect(by({ alone: false })).toBe(false)
    expect(by({ alone: true, known: false })).toBe(false)
  })
})

describe('что делает › на сторонке', () => {
  it('форма узнана — шаг', () => {
    expect(coverAct(side())).toEqual({ kind: 'step' })
  })
  it('не узнана — открыть ▦ на числе мест, 7+ → 7', () => {
    expect(coverAct(side({ known: false, places: 4 }))).toEqual({ kind: 'open', n: 4 })
    expect(coverAct(side({ known: false, places: 12 }))).toEqual({ kind: 'open', n: 7 })
  })
  it('0 мест — открыть ▦ на «1», места сами не появляются', () => {
    expect(coverAct(side({ known: false, places: 0 }))).toEqual({ kind: 'open', n: 1 })
    expect(openN({ places: 0 })).toBe(1)
  })
  it('одна раскладка, и она стоит — тост', () => {
    expect(coverAct(side({ alone: true }))).toEqual({ kind: 'toast', text: NO_OTHER })
  })
  it('крайнее: 1 место и 16 мест', () => {
    expect(coverAct(side({ places: 1, known: false }))).toEqual({ kind: 'open', n: 1 })
    expect(openN({ places: 16 })).toBe(7)
  })
})
