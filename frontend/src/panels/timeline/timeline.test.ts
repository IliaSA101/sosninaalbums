import { describe, expect, it } from 'vitest'
import { B, GAP, H, PAD, cellW, centerScroll, chipW, groupW, isBlank, mergeCount, tipLeft } from './timeline'
import type { SpreadView } from '../../base/core/types'

const sp = (n: number, w = 2, h = 1): SpreadView =>
  ({
    id: 's' + n,
    photos: Array.from({ length: n }, (_, i) => 'p' + i),
    rects: Array.from({ length: n }, () => ({ x: 0, y: 0, w, h })),
  }) as SpreadView

describe('chipW', () => {
  it('ширина по аспекту плюс рамка', () => {
    expect(chipW(1)).toBe(H)
    expect(chipW(2)).toBe(2 * (H - 2 * B) + 2 * B)
  })
  it('мусорный аспект — как квадрат', () => {
    for (const a of [0, -1, NaN, Infinity]) expect(chipW(a)).toBe(H)
  })
  it('cellW без прямоугольника — квадрат', () => {
    expect(cellW(undefined)).toBe(H)
  })
})

describe('groupW', () => {
  it('сумма ячеек, зазоры и отступы', () => {
    const s = sp(3)
    expect(groupW(s)).toBe(3 * cellW(s.rects[0]) + 2 * GAP + 2 * PAD)
  })
  it('пустые места сетки добавляют ячейки и зазоры', () => {
    const s = sp(1)
    const pl = { id: 'p1', x: 0, y: 0, w: 100, h: 100, photo: '', fit: '' as const, spine: false, rot: 0, z: 1 }
    s.places = [{ ...pl, id: 'p0', photo: 'a' }, pl]
    expect(groupW(s)).toBe(cellW(s.rects[0]) + cellW(pl) + GAP + 2 * PAD)
  })
  it('пустой разворот — одна заглушка', () => {
    expect(groupW(sp(0))).toBe(chipW(1) + 2 * PAD)
  })
  it('isBlank: только без фото и без пустых мест', () => {
    expect(isBlank(sp(0))).toBe(true)
    expect(isBlank(sp(1))).toBe(false)
    const pl = { id: 'p', x: 0, y: 0, w: 1, h: 1, photo: '', fit: '' as const, spine: false, rot: 0, z: 1 }
    expect(isBlank({ ...sp(0), places: [pl] })).toBe(false)
  })
})

describe('mergeCount', () => {
  it('сумма двух соседних, у последнего — только он сам', () => {
    const list = [sp(2), sp(3), sp(4)]
    expect(mergeCount(list, 0)).toBe(5)
    expect(mergeCount(list, 2)).toBe(4)
  })
})

describe('tipLeft и centerScroll', () => {
  it('подсказка не вылезает за окно', () => {
    expect(tipLeft(500, 100, 1000)).toBe(450)
    expect(tipLeft(10, 100, 1000)).toBe(8)
    expect(tipLeft(990, 100, 1000)).toBe(892)
  })
  it('группа по центру', () => {
    expect(centerScroll(500, 400, 100)).toBe(350)
  })
})
