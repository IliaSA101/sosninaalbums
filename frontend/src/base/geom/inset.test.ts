import { describe, expect, it } from 'vitest'
import vectors from '../../../../testdata/parity/inset.json'
import { MAX_INSET_PAIR, clampInset, insetBox } from './inset'
import type { Rect } from '../core/types'
import type { Outline } from './shape'

interface Vector {
  cell: Rect
  outline?: Outline
  ins: number[]
  clamped: number[]
  win: Rect
  winOutline?: Outline // нет — прямоугольник
}

describe('общие векторы с Go (inset.json)', () => {
  const list = vectors as Vector[]
  it('векторы прочитаны', () => expect(list.length).toBeGreaterThan(0))
  it('есть овал, скругление и зажим пары', () => {
    expect(list.some((v) => v.outline?.oval)).toBe(true)
    expect(list.some((v) => (v.outline?.radius ?? 0) > 0)).toBe(true)
    expect(list.some((v) => v.clamped.some((c, i) => c !== v.ins[i]))).toBe(true)
  })
  list.forEach((v, i) => {
    it(`вектор ${i}: зажим отступов`, () => {
      const c = clampInset(v.ins, v.cell.w, v.cell.h)
      c.forEach((n, k) => expect(n).toBeCloseTo(v.clamped[k], 9))
    })
    it(`вектор ${i}: окно и его форма`, () => {
      const { win, outline } = insetBox(v.cell, v.outline, v.ins)
      expect(win.x).toBeCloseTo(v.win.x, 9)
      expect(win.y).toBeCloseTo(v.win.y, 9)
      expect(win.w).toBeCloseTo(v.win.w, 9)
      expect(win.h).toBeCloseTo(v.win.h, 9)
      // Go пишет форму окна только непустой; без отступов вектор форму не повторяет — берётся форма места
      const want = v.winOutline ?? (v.ins.every((n) => n === 0) ? v.outline : undefined)
      expect(outline?.oval ?? false).toBe(want?.oval ?? false)
      expect(outline?.radius ?? 0).toBeCloseTo(want?.radius ?? 0, 9)
    })
  })
})

describe('clampInset', () => {
  it('мусор и минус — 0', () => {
    expect(clampInset([NaN, -3, Infinity, 2], 100, 100)).toEqual([0, 0, 0, 2])
  })
  it('короткий список — недостающие 0', () => expect(clampInset([1], 100, 100)).toEqual([1, 0, 0, 0]))
  it('идемпотентна', () => {
    const once = clampInset([30, 30, 30, 30], 40, 50)
    expect(clampInset(once, 40, 50)).toEqual(once)
  })
  it('пара оси не больше 70 % стороны', () => {
    const c = clampInset([30, 30, 30, 30], 40, 50)
    expect(c[0] + c[2]).toBeCloseTo(MAX_INSET_PAIR * 40, 9)
    expect(c[1] + c[3]).toBeCloseTo(MAX_INSET_PAIR * 50, 9)
  })
})

describe('insetBox', () => {
  const cell = { x: 0, y: 0, w: 100, h: 60 }
  it('без отступов — то же место и форма (тот же объект)', () => {
    const o = { radius: 5 }
    const r = insetBox(cell, o, [0, 0, 0, 0])
    expect(r.win).toBe(cell)
    expect(r.outline).toBe(o)
  })
  it('скругление меньше отступа — прямые углы окна', () => {
    expect(insetBox(cell, { radius: 2 }, [3, 3, 3, 3]).outline).toBeUndefined()
  })
})
