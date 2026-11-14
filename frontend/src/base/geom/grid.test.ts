import { describe, expect, it } from 'vitest'
import { arrowDelta, moveBox, moveBoxes, nudge, resizeBox, round2, roundBox } from './grid'
import type { Rect } from '../core/types'

const B = { w: 560, h: 280 }
const box: Rect = { x: 100, y: 100, w: 100, h: 50 } // пропорции 2:1

describe('resizeBox без замка', () => {
  it('угол se тянет правый и нижний края', () => {
    expect(resizeBox(box, 'se', 20, 10, false)).toEqual({ x: 100, y: 100, w: 120, h: 60 })
  })
  it('угол nw двигает левый и верхний, правый-нижний на месте', () => {
    expect(resizeBox(box, 'nw', -10, -5, false)).toEqual({ x: 90, y: 95, w: 110, h: 55 })
  })
  it('боковая ручка меняет одну ось', () => {
    expect(resizeBox(box, 'e', 30, 99, false)).toEqual({ x: 100, y: 100, w: 130, h: 50 })
    expect(resizeBox(box, 'n', 99, -20, false)).toEqual({ x: 100, y: 80, w: 100, h: 70 })
  })
  it('не меньше минимума, противоположный край неподвижен', () => {
    expect(resizeBox(box, 'e', -500, 0, false, 5)).toEqual({ x: 100, y: 100, w: 5, h: 50 })
    expect(resizeBox(box, 'w', 500, 0, false, 5)).toEqual({ x: 195, y: 100, w: 5, h: 50 })
    expect(resizeBox(box, 's', 0, -500, false, 5).h).toBe(5)
  })
  it('не выходит за разворот', () => {
    expect(resizeBox(box, 'se', 9999, 9999, false, 5, B)).toEqual({ x: 100, y: 100, w: 460, h: 180 })
    expect(resizeBox(box, 'nw', -9999, -9999, false, 5, B)).toEqual({ x: 0, y: 0, w: 200, h: 150 })
  })
})

describe('resizeBox с замком пропорций', () => {
  it('угол: пропорции сохраняются, неподвижен противоположный угол', () => {
    expect(resizeBox(box, 'se', 50, 0, true)).toEqual({ x: 100, y: 100, w: 150, h: 75 })
    const n = resizeBox(box, 'nw', -50, 0, true)
    expect(n.x + n.w).toBe(200)
    expect(n.y + n.h).toBe(150)
    expect(n.w / n.h).toBeCloseTo(2)
  })
  it('угол: побеждает больший относительный сдвиг', () => {
    // по y +50 % (25 мм), по x +10 % (10 мм): масштаб 1.5
    const r = resizeBox(box, 'se', 10, 25, true)
    expect(r.w).toBeCloseTo(150)
    expect(r.h).toBeCloseTo(75)
  })
  it('боковая ручка: вторая ось меняется вокруг центра', () => {
    const r = resizeBox(box, 'e', 100, 0, true)
    expect(r.w).toBe(200)
    expect(r.h).toBe(100)
    expect(r.x).toBe(100)
    expect(r.y + r.h / 2).toBe(125)
    const s = resizeBox(box, 's', 0, 50, true)
    expect(s.h).toBe(100)
    expect(s.w).toBe(200)
    expect(s.x + s.w / 2).toBe(150)
  })
  it('минимум: меньшая сторона не меньше min', () => {
    const r = resizeBox(box, 'se', -500, -500, true, 5)
    expect(r.h).toBeCloseTo(5)
    expect(r.w).toBeCloseTo(10)
  })
  it('предел: растёт до края разворота с теми же пропорциями', () => {
    const r = resizeBox(box, 'se', 9999, 9999, true, 5, B)
    expect(r.x).toBe(100)
    expect(r.y).toBe(100)
    expect(r.h).toBeCloseTo(180) // упёрлись в низ
    expect(r.w).toBeCloseTo(360)
    const w = resizeBox(box, 'nw', -9999, -9999, true, 5, B)
    expect(w.x + w.w).toBe(200)
    expect(w.y).toBeGreaterThanOrEqual(0)
    expect(w.x).toBeGreaterThanOrEqual(0)
  })
  it('боковая ручка у края: вторая ось не вылетает', () => {
    const r = resizeBox({ x: 0, y: 5, w: 100, h: 50 }, 'e', 9999, 0, true, 5, B)
    expect(r.y).toBeGreaterThanOrEqual(0)
    expect(r.y + r.h).toBeLessThanOrEqual(280)
  })
  it('нулевая рамка не ломается', () => {
    expect(resizeBox({ x: 1, y: 1, w: 0, h: 0 }, 'se', 5, 5, true)).toEqual({ x: 1, y: 1, w: 0, h: 0 })
  })
})

describe('moveBox / moveBoxes / nudge', () => {
  it('перенос не выходит за разворот', () => {
    expect(moveBox(box, 10, -500, B)).toEqual({ x: 110, y: 0, w: 100, h: 50 })
    expect(moveBox(box, 9999, 9999, B)).toEqual({ x: 460, y: 230, w: 100, h: 50 })
  })
  it('группа двигается вместе: упёрлось первое — стоят все', () => {
    const g: Rect[] = [
      { x: 10, y: 20, w: 50, h: 50 },
      { x: 200, y: 20, w: 50, h: 50 },
    ]
    const r = moveBoxes(g, -100, 5, B)
    expect(r.map((b) => b.x)).toEqual([0, 190])
    expect(r.map((b) => b.y)).toEqual([25, 25])
    expect(moveBoxes([], 5, 5, B)).toEqual([])
  })
  it('стрелки: 1 мм, с Shift 10; не стрелка — ноль', () => {
    expect(arrowDelta('ArrowLeft', false)).toEqual({ dx: -1, dy: 0 })
    expect(arrowDelta('ArrowDown', true)).toEqual({ dx: 0, dy: 10 })
    expect(arrowDelta('a', false)).toEqual({ dx: 0, dy: 0 })
    expect(nudge([box], 'ArrowRight', true, B)).toEqual([{ x: 110, y: 100, w: 100, h: 50 }])
    expect(nudge([{ ...box, x: 0 }], 'ArrowLeft', false, B)[0].x).toBe(0)
  })
})

describe('округление', () => {
  it('до 0.01 мм', () => {
    expect(round2(1.006)).toBe(1.01)
    expect(roundBox({ x: 0.123, y: 1.999, w: 3.333, h: 4 })).toEqual({ x: 0.12, y: 2, w: 3.33, h: 4 })
  })
})
