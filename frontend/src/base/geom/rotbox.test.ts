import { describe, expect, it } from 'vitest'
import vectors from '../../../../testdata/parity/rotplace.json'
import {
  aabbOf,
  clampMoveIn,
  clampPlaceIn,
  holdOf,
  moveRotated,
  resizeCursor,
  resizeRotated,
  rotHandleAt,
  rotHandleIn,
  uprightAt,
} from './rotbox'
import type { RotRect } from '../core/types'

interface Vector {
  name: string
  sw: number
  ph: number
  prev?: RotRect
  in: RotRect
  out: RotRect
}

const near = (a: number, b: number, eps = 1e-9) => expect(Math.abs(a - b)).toBeLessThan(eps)
const SW = 562
const PH = 284

describe('общие векторы с Go (rotplace.json)', () => {
  const list = vectors as Vector[]
  it('векторы прочитаны, есть с prev и без', () => {
    expect(list.length).toBeGreaterThan(0)
    expect(list.some((v) => v.prev)).toBe(true)
    expect(list.some((v) => !v.prev)).toBe(true)
  })
  list.forEach((v) => {
    it(v.name, () => {
      const o = v.prev ? clampMoveIn(v.in, v.prev, v.sw, v.ph) : clampPlaceIn(v.in, v.sw, v.ph)
      near(o.x, v.out.x)
      near(o.y, v.out.y)
      expect(o.w).toBe(v.out.w)
      expect(o.h).toBe(v.out.h)
      expect(o.rot ?? 0).toBe(v.out.rot ?? 0)
      // Итог любого зажима — неподвижная точка clampPlace (инвариант Load).
      const again = clampPlaceIn(o, v.sw, v.ph)
      near(again.x, o.x)
      near(again.y, o.y)
    })
  })
})

describe('aabb и hold', () => {
  it('без поворота — само место', () => {
    expect(aabbOf({ x: 1, y: 2, w: 3, h: 4 })).toEqual({ x: 1, y: 2, w: 3, h: 4 })
    expect(holdOf({ x: 1, y: 2, w: 3, h: 4 })).toEqual({ x: 1, y: 2, w: 3, h: 4 })
  })
  it('90°: aabb меняет стороны вокруг центра, hold — квадрат по меньшей', () => {
    const a = aabbOf({ x: 100, y: 100, w: 100, h: 60, rot: 90 })
    near(a.x, 120)
    near(a.y, 80)
    near(a.w, 60)
    near(a.h, 100)
    expect(holdOf({ x: 100, y: 100, w: 100, h: 60, rot: 90 })).toEqual({ x: 120, y: 100, w: 60, h: 60 })
  })
})

describe('поворот у края не сдвигает', () => {
  it('30°, 90°, 145° у угла — те же x, y, w, h', () => {
    for (const rot of [30, 90, 145, -90]) {
      const o = clampPlaceIn({ x: 0, y: 0, w: 100, h: 60, rot }, SW, PH)
      expect([o.x, o.y, o.w, o.h]).toEqual([0, 0, 100, 60])
    }
    for (const rot of [30, 90, 145]) {
      const o = clampPlaceIn({ x: 462, y: 224, w: 100, h: 60, rot }, SW, PH)
      expect([o.x, o.y]).toEqual([462, 224])
    }
  })
})

describe('перенос повёрнутого', () => {
  const base: RotRect = { x: 200, y: 100, w: 100, h: 60, rot: 30 }
  it('внутри — на сдвиг, без привязки', () => {
    const m = moveRotated(base, 10, -5, null, null, SW, PH)
    near(m.box.x, 210)
    near(m.box.y, 95)
    expect(m.guideX).toBeNull()
  })
  it('вылет aabb не растёт: у края не уходит дальше прежнего', () => {
    const edge: RotRect = { x: 462, y: 100, w: 100, h: 60, rot: 0.5 }
    const m = moveRotated(edge, 80, 0, null, null, SW, PH)
    const was = aabbOf(edge)
    const a = aabbOf(m.box)
    expect(a.x + a.w).toBeLessThanOrEqual(was.x + was.w + 1e-6)
  })
  it('привязка по aabb: край описанного прямоугольника встаёт на линию', () => {
    const a = aabbOf(base)
    const snap = {
      others: [{ x: 400, y: 0, w: 50, h: 50 }],
      params: { w: 562, h: PH, mid: 281, margin: 0, gap: 0, rects: [] },
      thr: 5,
    }
    const dx = 400 - (a.x + a.w) - 2 // правый край aabb в 2 мм от края соседа
    const m = moveRotated(base, dx, 0, snap, null, SW, PH)
    const g = aabbOf(m.box)
    expect(m.guideX).not.toBeNull()
    // к линии встал левый край, центр или правый край описанного прямоугольника (не неповёрнутого)
    expect([g.x, g.x + g.w / 2, g.x + g.w].some((v) => Math.abs(v - m.guideX!) < 0.011)).toBe(true)
  })
  it('ось x: по y не сдвигается', () => {
    const m = moveRotated(base, 10, 0, null, 'x', SW, PH)
    near(m.box.y, 100)
  })
})

describe('размер в осях места', () => {
  const base: RotRect = { x: 200, y: 100, w: 100, h: 60, rot: 90 }
  it('правая ручка при 90°: сдвиг мыши вниз растит ширину, сторона места напротив стоит', () => {
    const o = resizeRotated(base, 'e', 0, 20, false, SW, PH)
    near(o.w, 120)
    near(o.h, 60)
    // левая сторона в осях места (при 90° это верхняя на экране) остаётся на месте
    near(aabbOf(o).y, aabbOf(base).y)
    near(aabbOf(o).h, 120)
  })
  it('левая ручка: противоположная сторона стоит', () => {
    const o = resizeRotated({ ...base, rot: 0 }, 'w', -20, 0, false, SW, PH)
    near(o.w, 120)
    near(o.x + o.w, 300)
  })
  it('замок: угол — пропорционально от противоположного', () => {
    const o = resizeRotated({ ...base, rot: 0 }, 'se', 20, 0, true, SW, PH)
    near(o.w / o.h, 100 / 60, 1e-2)
    near(o.x, 200)
    near(o.y, 100)
  })
  it('не меньше 5 мм', () => {
    const o = resizeRotated({ ...base, rot: 0 }, 'e', -500, 0, false, SW, PH)
    expect(o.w).toBe(5)
  })
})

describe('курсор ручки на повёрнутой рамке', () => {
  it('без поворота — обычные', () => {
    expect(resizeCursor('n', 0)).toBe('ns-resize')
    expect(resizeCursor('e', 0)).toBe('ew-resize')
    expect(resizeCursor('nw', 0)).toBe('nwse-resize')
    expect(resizeCursor('ne', 0)).toBe('nesw-resize')
  })
  it('90°: верхняя стала боковой, угол — другой диагональю', () => {
    expect(resizeCursor('n', 90)).toBe('ew-resize')
    expect(resizeCursor('nw', 90)).toBe('nesw-resize')
  })
  it('45°: верхняя — по диагонали', () => expect(resizeCursor('n', 45)).toBe('nesw-resize'))
})

describe('метка DpiMark по aabb', () => {
  it('без поворота — отступ от угла', () => {
    const u = uprightAt({ x: 10, y: 10, w: 100, h: 60 }, 0, { x: 10, y: 10, w: 100, h: 60 }, 6)
    expect(u.left).toBe('calc(0% + 6px)')
    expect(u.top).toBe('calc(0% + 6px)')
  })
  it('поворот гасится обратным', () => expect(uprightAt({ x: 0, y: 0, w: 10, h: 10 }, 30, { x: 0, y: 0, w: 10, h: 10 }, 6).rot).toBe(-30))
})

describe('⟳ над верхней стороной', () => {
  it('у верхнего края не помещается — внутрь', () => {
    expect(rotHandleIn({ x: 100, y: 5, w: 50, h: 50 }, 8, SW, PH)).toBe(true)
    expect(rotHandleIn({ x: 100, y: 100, w: 50, h: 50 }, 8, SW, PH)).toBe(false)
  })
  it('повёрнутое на 180° уходит вниз', () => expect(rotHandleIn({ x: 100, y: 200, w: 50, h: 80, rot: 180 }, 8, SW, PH)).toBe(true))
})

describe('rotHandleAt', () => {
  it('над серединой верхней стороны', () => {
    expect(rotHandleAt({ x: 100, y: 50, w: 40, h: 20 }, 5)).toEqual({ x: 120, y: 45 })
  })
  it('внутрь — отрицательный вынос; на 180° переезжает вниз', () => {
    const p = rotHandleAt({ x: 100, y: 50, w: 40, h: 20, rot: 180 }, 5)
    expect(p.x).toBeCloseTo(120)
    expect(p.y).toBeCloseTo(75)
    expect(rotHandleAt({ x: 100, y: 50, w: 40, h: 20 }, -5).y).toBe(55)
  })
})
