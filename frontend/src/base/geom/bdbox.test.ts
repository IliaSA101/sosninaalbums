import { describe, expect, it } from 'vitest'
import { bdAngle, bdLines, moveFrame, nudgeBox, panBox, resizeSide, scaleFrame } from './bdbox'
import { corners, toSpread } from './textbox'

const area = { x: 0, y: 0, w: 300, h: 300 }
const close = (a: { x: number; y: number }, b: { x: number; y: number }) => {
  expect(a.x).toBeCloseTo(b.x, 6)
  expect(a.y).toBeCloseTo(b.y, 6)
}

describe('bdLines', () => {
  it('края и центр области, корешок', () => {
    const l = bdLines({ x: 300, y: 0, w: 300, h: 300 }, 300)
    expect(l.v).toEqual([300, 450, 600])
    expect(l.h).toEqual([0, 150, 300])
  })
  it('обложка: вместо шва — края и центры сторонок', () => {
    const l = bdLines({ x: 0, y: 0, w: 600, h: 300 }, 300, [
      [0, 293],
      [307, 600],
    ])
    expect(l.v).toEqual([0, 146.5, 293, 300, 307, 453.5, 600])
    expect(l.v).toContain(300) // центр области остаётся
  })
})

describe('moveFrame', () => {
  const lines = bdLines(area, 300)
  it('прилипает к краю области, Alt (thr = 0) — нет', () => {
    const t = { x: 10, y: 10, w: 100, h: 100, rot: 0 }
    const m = moveFrame(t, -8, 50, area, lines, 3)
    expect(m.x).toBe(0)
    expect(m.guideX).toBe(0)
    expect(moveFrame(t, -8, 50, area, lines, 0).x).toBe(2)
  })
  it('центр рамки не выходит из области', () => {
    const t = { x: 100, y: 100, w: 100, h: 100, rot: 0 }
    const m = moveFrame(t, 500, 0, area, lines, 0)
    expect(m.x + 50).toBe(300)
    expect(m.guideX).toBeNull()
  })
  it('повёрнутая рамка: прилипает описанный прямоугольник', () => {
    const t = { x: 100, y: 50, w: 100, h: 100, rot: 45 }
    const m = moveFrame(t, 0, 0, area, lines, 3)
    // описанный квадрат 45° со стороной ≈141: левый край был в 100 − 70.7 ≈ 29.3
    expect(m.x).toBeCloseTo(100, 6)
    const far = moveFrame(t, 0, 0, area, { v: [30], h: [] }, 3)
    expect(far.x).toBeCloseTo(100 + (30 - (100 - 100 / Math.SQRT2)), 6)
  })
})

describe('resizeSide', () => {
  it('прямая рамка: правая и нижняя растут, левая и верхняя уводят угол', () => {
    const t = { x: 10, y: 10, w: 100, h: 50, rot: 0 }
    expect(resizeSide(t, 'r', 20, 99, area)).toEqual({ ...t, w: 120 })
    expect(resizeSide(t, 'b', 99, 20, area)).toEqual({ ...t, h: 70 })
    expect(resizeSide(t, 'l', 20, 0, area)).toEqual({ ...t, x: 30, w: 80 })
    expect(resizeSide(t, 't', 0, -10, area)).toEqual({ ...t, y: 0, h: 60 })
  })
  it('повёрнутая на 90°: сдвиг мыши вниз — это ось рамки u', () => {
    const t = { x: 100, y: 100, w: 100, h: 50, rot: 90 }
    expect(resizeSide(t, 'r', 0, 30, area).w).toBeCloseTo(130)
    expect(resizeSide(t, 'r', 30, 0, area).w).toBeCloseTo(100)
  })
  it('противоположная сторона стоит при любом угле', () => {
    const t = { x: 100, y: 100, w: 100, h: 60, rot: 30 }
    const l = resizeSide(t, 'l', 15, -7, area)
    close(toSpread(l, l.w, 0), toSpread(t, t.w, 0))
    close(toSpread(l, l.w, l.h), toSpread(t, t.w, t.h))
    const top = resizeSide(t, 't', 4, 12, area)
    close(toSpread(top, 0, top.h), toSpread(t, 0, t.h))
    close(toSpread(top, top.w, top.h), toSpread(t, t.w, t.h))
  })
  it('пределы: 10 мм и 3 области', () => {
    const t = { x: 0, y: 0, w: 100, h: 100, rot: 0 }
    expect(resizeSide(t, 'r', -500, 0, area).w).toBe(10)
    expect(resizeSide(t, 'b', 0, 5000, area).h).toBe(900)
    const l = resizeSide(t, 'l', 500, 0, area)
    expect(l.w).toBe(10)
    expect(l.x).toBe(90)
  })
})

describe('scaleFrame', () => {
  it('угол: пропорции и противоположный угол стоят', () => {
    const t = { x: 50, y: 50, w: 100, h: 50, rot: 20 }
    const s = scaleFrame(t, 'br', 30, 10, area)
    expect(s.w / s.h).toBeCloseTo(2, 6)
    close(corners(s)[0], corners(t)[0])
    const tl = scaleFrame(t, 'tl', -10, -10, area)
    close(corners(tl)[3], corners(t)[3])
  })
  it('пределы: меньшая сторона не меньше 10 мм, рамка не больше 3 областей', () => {
    const t = { x: 0, y: 0, w: 100, h: 50, rot: 0 }
    expect(scaleFrame(t, 'br', -500, -500, area).h).toBeCloseTo(10)
    expect(scaleFrame(t, 'br', 5000, 2500, area).w).toBeCloseTo(900)
  })
})

describe('bdAngle', () => {
  const c = { x: 0, y: 0 }
  it('прилипание к 0, ±90, 180 в пределах 2°; Alt — без', () => {
    const p = { x: Math.sin((1.5 * Math.PI) / 180) * 100, y: -Math.cos((1.5 * Math.PI) / 180) * 100 }
    expect(bdAngle(c, p, 0, 0, false, false)).toBe(0)
    expect(bdAngle(c, p, 0, 0, false, true)).toBe(1.5)
  })
  it('Shift — шаг 15°, и с Alt тоже', () => {
    const p = { x: Math.sin((22 * Math.PI) / 180) * 100, y: -Math.cos((22 * Math.PI) / 180) * 100 }
    expect(bdAngle(c, p, 0, 0, true, false)).toBe(15)
    expect(bdAngle(c, p, 0, 0, true, true)).toBe(15)
  })
  it('захват не по оси: рамка не прыгает под курсор', () => {
    // взяли ручку на 10°, рамка стояла на 0° — курсор на 40° даёт 30°
    const p = { x: Math.sin((40 * Math.PI) / 180) * 100, y: -Math.cos((40 * Math.PI) / 180) * 100 }
    expect(bdAngle(c, p, 10, 0, false, true)).toBeCloseTo(30, 1)
  })
})

describe('nudgeBox', () => {
  it('стрелки: 1 мм, с Shift — 10 мм; другая клавиша — null', () => {
    const t = { x: 10, y: 10, w: 100, h: 50, rot: 15 }
    expect(nudgeBox(t, 'ArrowLeft', false)).toEqual({ ...t, x: 9 })
    expect(nudgeBox(t, 'ArrowDown', true)).toEqual({ ...t, y: 20 })
    expect(nudgeBox(t, 'a', false)).toBeNull()
  })
})

describe('panBox', () => {
  it('сдвиг мыши переводится в оси рамки', () => {
    const img = { x: 0, y: -50, w: 100, h: 200 } // ход по v = 100
    const f = { s: 1, x: 0, y: 0 }
    // рамка повёрнута на 90°: мышь влево (−x) = вдоль оси v рамки
    const r = panBox(f, { x: 0, y: 0, w: 100, h: 100, rot: 90 }, img, -50, 0)
    expect(r.x).toBe(0)
    expect(r.y).toBeCloseTo(-1)
    const straight = panBox(f, { x: 0, y: 0, w: 100, h: 100, rot: 0 }, img, -50, 0)
    expect(straight.y).toBe(0)
  })
})
