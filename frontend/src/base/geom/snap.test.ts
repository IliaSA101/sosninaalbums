import { describe, expect, it } from 'vitest'
import { clampBox, gapTargets, snapAxis, snapBox, snapEdge, snapLines, snapPlaceMove, snapPlaceResize, snapThreshold } from './snap'
import type { Rect } from '../core/types'

const P = { pageW: 280, pageH: 280, margin: 12, rects: [{ x: 20, y: 40, w: 100, h: 60 }] }

describe('snapLines', () => {
  const l = snapLines(P)
  it('края, центры страниц, корешок и поля', () => {
    for (const x of [0, 140, 280, 420, 560, 12, 268, 292, 548]) expect(l.v).toContain(x)
    for (const y of [0, 140, 280, 12, 268]) expect(l.h).toContain(y)
  })
  it('края и центры фото', () => {
    for (const x of [20, 70, 120]) expect(l.v).toContain(x)
    for (const y of [40, 70, 100]) expect(l.h).toContain(y)
  })
  it('без повторов и по возрастанию', () => {
    expect(new Set(l.v).size).toBe(l.v.length)
    expect([...l.v].sort((a, b) => a - b)).toEqual(l.v)
  })
})

describe('snapThreshold', () => {
  it('6 px экрана в мм', () => expect(snapThreshold(1120, 280)).toBeCloseTo(3)) // 560 мм на 1120 px: 0.5 мм/px
  it('нулевая ширина — порога нет', () => expect(snapThreshold(0, 280)).toBe(0))
})

describe('snapAxis', () => {
  it('ближайшая линия в пороге', () => {
    expect(snapAxis([100], [98, 103], 3)).toEqual({ d: -2, line: 98 })
  })
  it('вне порога — без прилипания', () => {
    expect(snapAxis([100], [90], 3)).toEqual({ d: 0, line: null })
  })
  it('из нескольких точек побеждает меньший зазор', () => {
    expect(snapAxis([100, 150], [101, 150.5], 3)).toEqual({ d: 0.5, line: 150.5 })
  })
})

describe('snapBox', () => {
  const lines = snapLines(P)
  it('центр блока к центру страницы', () => {
    // блок 100×20, центр на 138 -> прилипает к 140
    const r = snapBox({ x: 88, y: 200, w: 100, h: 20 }, lines, 3)
    expect(r.x).toBeCloseTo(90)
    expect(r.guideX).toBe(140)
    expect(r.guideY).toBeNull()
  })
  it('правый край к корешку', () => {
    const r = snapBox({ x: 180.5, y: 200, w: 100, h: 20 }, lines, 3)
    expect(r.x + 100).toBeCloseTo(280)
    expect(r.guideX).toBe(280)
  })
  it('верх блока к низу фото и край к краю фото', () => {
    const r = snapBox({ x: 21, y: 101.5, w: 50, h: 10 }, lines, 3)
    expect(r.x).toBe(20)
    expect(r.y).toBe(100)
    expect(r.guideX).toBe(20)
    expect(r.guideY).toBe(100)
  })
  it('далеко от линий — без изменений', () => {
    const r = snapBox({ x: 150, y: 190, w: 30, h: 8 }, lines, 1)
    expect(r).toEqual({ x: 150, y: 190, guideX: null, guideY: null })
  })
})

describe('snapEdge', () => {
  it('ручка ширины прилипает к полю', () => {
    expect(snapEdge(266, snapLines(P), 3)).toEqual({ x: 268, guide: 268 })
  })
})

describe('clampBox', () => {
  it('не выходит за разворот', () => {
    expect(clampBox({ x: -5, y: 270, w: 100, h: 20 }, 560, 280)).toEqual({ x: 0, y: 260, w: 100, h: 20 })
    expect(clampBox({ x: 500, y: 10, w: 100, h: 20 }, 560, 280).x).toBe(460)
  })
})

describe('gapTargets', () => {
  const o: Rect = { x: 100, y: 100, w: 50, h: 50 }
  it('сосед слева и справа — только при пересечении по вертикали', () => {
    const t = gapTargets({ x: 200, y: 110, w: 30, h: 30 }, [o], 5)
    expect(t.left).toEqual([155])
    expect(t.right).toEqual([95])
    expect(t.top).toEqual([]) // по горизонтали с соседом не пересекается
  })
  it('сосед сверху и снизу — при пересечении по горизонтали', () => {
    const t = gapTargets({ x: 110, y: 200, w: 30, h: 30 }, [o], 5)
    expect(t.top).toEqual([155])
    expect(t.bottom).toEqual([95])
    expect(t.left).toEqual([])
  })
  it('касание краем — не пересечение', () => {
    const t = gapTargets({ x: 200, y: 150, w: 30, h: 30 }, [o], 5)
    expect(t.left).toEqual([])
  })
})

describe('snapPlaceMove', () => {
  const P2 = { pageW: 280, pageH: 280, margin: 0, gap: 4, rects: [] as Rect[] }
  const other: Rect = { x: 20, y: 40, w: 100, h: 60 }
  it('край к краю соседнего места', () => {
    const r = snapPlaceMove({ x: 20.5, y: 150, w: 50, h: 50 }, [other], P2, 3)
    expect(r.x).toBe(20)
    expect(r.guideX).toBe(20)
    expect(r.gapX).toBe(false)
  })
  it('центр к центру соседнего места', () => {
    const r = snapPlaceMove({ x: 45.5, y: 150, w: 50, h: 50 }, [other], P2, 3) // центр 70.5, у соседа 70
    expect(r.x).toBeCloseTo(45)
    expect(r.guideX).toBe(70)
  })
  it('тот же промежуток: справа от соседа на gap', () => {
    // сосед кончается на x = 120; левый край должен встать на 124
    const r = snapPlaceMove({ x: 125.5, y: 50, w: 50, h: 50 }, [other], P2, 3)
    expect(r.x).toBe(124)
    expect(r.gapX).toBe(true)
    expect(r.guideX).toBe(124)
  })
  it('тот же промежуток по y: под соседом', () => {
    const r = snapPlaceMove({ x: 30, y: 103.5, w: 50, h: 50 }, [other], P2, 3) // низ соседа 100 -> верх на 104
    expect(r.y).toBe(104)
    expect(r.gapY).toBe(true)
  })
  it('при равном зазоре обычная линия важнее промежутка', () => {
    // край на 122: до линии 120 и до промежутка 124 по 2 мм — побеждает линия (идёт раньше)
    const r = snapPlaceMove({ x: 122, y: 50, w: 50, h: 50 }, [other], P2, 3)
    expect(r.gapX).toBe(false)
    expect(r.x).toBe(120)
  })
  it('далеко — без прилипания', () => {
    const r = snapPlaceMove({ x: 300, y: 200, w: 30, h: 8 }, [other], P2, 1)
    expect(r).toMatchObject({ x: 300, y: 200, guideX: null, guideY: null, gapX: false, gapY: false })
  })
})

describe('snapPlaceResize', () => {
  const P2 = { pageW: 280, pageH: 280, margin: 0, gap: 4, rects: [] as Rect[] }
  const other: Rect = { x: 200, y: 40, w: 60, h: 60 }
  it('правый край ручки e прилипает к краю соседа', () => {
    const r = snapPlaceResize({ x: 50, y: 50, w: 149, h: 30 }, 'e', [other], P2, 3, 5)
    expect(r.box).toEqual({ x: 50, y: 50, w: 150, h: 30 })
    expect(r.guideX).toBe(200)
  })
  it('правый край — промежуток до соседа справа', () => {
    const r = snapPlaceResize({ x: 50, y: 50, w: 144.5, h: 30 }, 'e', [other], P2, 2, 5) // край 194.5 -> 196
    expect(r.box.w).toBeCloseTo(146)
    expect(r.guideX).toBe(196)
  })
  it('левый край w: правый остаётся на месте', () => {
    const r = snapPlaceResize({ x: 201, y: 50, w: 50, h: 30 }, 'w', [other], P2, 3, 5)
    expect(r.box.x).toBe(200)
    expect(r.box.x + r.box.w).toBe(251)
  })
  it('верхний край n и нижний s', () => {
    const o2: Rect = { x: 60, y: 100, w: 40, h: 40 }
    const n = snapPlaceResize({ x: 60, y: 101, w: 40, h: 30 }, 'n', [o2], P2, 3, 5)
    expect(n.box.y).toBe(100)
    expect(n.guideY).toBe(100)
    const s = snapPlaceResize({ x: 60, y: 10, w: 40, h: 89 }, 's', [o2], P2, 3, 5)
    expect(s.box.h).toBe(90)
  })
  it('не прилипает, если сторона стала бы меньше минимума', () => {
    const r = snapPlaceResize({ x: 195, y: 50, w: 6, h: 30 }, 'w', [other], P2, 3, 5)
    expect(r.box).toEqual({ x: 195, y: 50, w: 6, h: 30 }) // к 200 ширина стала бы 1 < 5
    expect(r.guideX).toBeNull()
  })
  it('края, которые ручка не двигает, не трогаются', () => {
    const r = snapPlaceResize({ x: 199, y: 50, w: 30, h: 30 }, 'e', [other], P2, 3, 5)
    expect(r.box.x).toBe(199)
  })
})
