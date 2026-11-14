import { describe, expect, it } from 'vitest'
import vectors from '../../../../testdata/parity/textbox.json'
import { moveBox } from '../../editor/text/textedit'
import {
  aabb,
  angleAt,
  anchor,
  center,
  cursorDeg,
  dragAngle,
  editorFrame,
  boxOf,
  corners,
  moveRotated,
  normDeg,
  resizeLocal,
  rotate,
  rotateAbout,
  scaleCorner,
  toSpread,
  type Align,
  type Pt,
  type TextBox,
} from './textbox'

// Общие векторы с Go (textbox.json) — первым блоком; остальное — случаи, выведенные вручную по design §3.

interface Vector {
  x: number
  y: number
  w: number
  h: number
  rot: number
  align: Align
  deg: number
  aabb: { x: number; y: number; w: number; h: number }
  rotated: Pt
  anchor: Pt
  norm: number
}

describe('общие векторы с Go (textbox.json)', () => {
  const list = vectors as Vector[]
  it('векторы прочитаны', () => expect(list.length).toBeGreaterThan(0))
  list.forEach((v, i) => {
    it(`вектор ${i}: aabb, rotateAbout, anchor, normDeg`, () => {
      const t: TextBox = { x: v.x, y: v.y, w: v.w, h: v.h, rot: v.rot, align: v.align }
      const a = aabb(t)
      for (const f of ['x', 'y', 'w', 'h'] as const) expect(a[f]).toBeCloseTo(v.aabb[f], 6)
      const r = rotateAbout(t, normDeg(v.deg)) // Go поворачивает на norm
      expect(r.x).toBeCloseTo(v.rotated.x, 6)
      expect(r.y).toBeCloseTo(v.rotated.y, 6)
      const an = anchor(t)
      expect(an.x).toBeCloseTo(v.anchor.x, 6)
      expect(an.y).toBeCloseTo(v.anchor.y, 6)
      expect(normDeg(v.deg)).toBe(v.norm)
    })
  })
})

const box = (o: Partial<TextBox> = {}): TextBox => ({ x: 100, y: 50, w: 40, h: 10, rot: 0, ...o })
const close = (a: Pt, b: Pt) => {
  expect(a.x).toBeCloseTo(b.x, 9)
  expect(a.y).toBeCloseTo(b.y, 9)
}
// Точка на экране в направлении угла d (по часовой от «вверх») на расстоянии r от c.
const at = (c: Pt, d: number, r = 100): Pt => ({
  x: c.x + r * Math.sin((d * Math.PI) / 180),
  y: c.y - r * Math.cos((d * Math.PI) / 180),
})

describe('normDeg', () => {
  it('в (−180, 180]', () => {
    expect(normDeg(190)).toBe(-170)
    expect(normDeg(-190)).toBe(170)
    expect(normDeg(180)).toBe(180)
    expect(normDeg(-180)).toBe(180)
    expect(normDeg(540)).toBe(180)
    expect(normDeg(-540)).toBe(180)
    expect(normDeg(720)).toBe(0)
  })
  it('шаг 0.1, малые углы — 0, округление от нуля как в Go', () => {
    expect(normDeg(12.34)).toBe(12.3)
    expect(normDeg(0.04)).toBe(0)
    expect(normDeg(-0.04)).toBe(0)
    expect(Object.is(normDeg(-0.04), 0)).toBe(true)
    expect(normDeg(0.25)).toBe(0.3)
    expect(normDeg(-0.25)).toBe(-0.3)
    expect(normDeg(359.96)).toBe(0)
    expect(normDeg(-179.96)).toBe(180)
  })
  it('NaN и Inf — 0', () => {
    expect(normDeg(NaN)).toBe(0)
    expect(normDeg(Infinity)).toBe(0)
  })
})

describe('corners / aabb', () => {
  it('при 0° aabb — сама рамка', () => {
    expect(aabb(box())).toEqual({ x: 100, y: 50, w: 40, h: 10 })
  })
  it('90° по часовой: строка идёт вниз, высота уходит влево от P', () => {
    const [p0, p1, p2, p3] = corners(box({ rot: 90 }))
    expect(p0).toEqual({ x: 100, y: 50 })
    expect(p1).toEqual({ x: 100, y: 90 })
    expect(p2).toEqual({ x: 90, y: 50 })
    expect(p3).toEqual({ x: 90, y: 90 })
    expect(aabb(box({ rot: 90 }))).toEqual({ x: 90, y: 50, w: 10, h: 40 })
  })
  it('180°: рамка зеркалится вокруг P', () => {
    expect(aabb(box({ rot: 180 }))).toEqual({ x: 60, y: 40, w: 40, h: 10 })
  })
  it('30°: описанный прямоугольник', () => {
    const t = box({ w: 100, h: 20, rot: 30 })
    const a = aabb(t)
    const s = Math.sin(Math.PI / 6)
    const c = Math.cos(Math.PI / 6)
    expect(a.w).toBeCloseTo(100 * c + 20 * s, 9)
    expect(a.h).toBeCloseTo(100 * s + 20 * c, 9)
    expect(a.x).toBeCloseTo(100 - 20 * s, 9) // левее P на проекцию высоты
    expect(a.y).toBeCloseTo(50, 9) // верхний угол — сам P
  })
  it('все углы внутри aabb', () => {
    for (const rot of [-135, -45, 17.5, 60, 100]) {
      const t = box({ w: 73, h: 21, rot })
      const a = aabb(t)
      for (const p of corners(t)) {
        expect(p.x).toBeGreaterThanOrEqual(a.x - 1e-9)
        expect(p.x).toBeLessThanOrEqual(a.x + a.w + 1e-9)
        expect(p.y).toBeGreaterThanOrEqual(a.y - 1e-9)
        expect(p.y).toBeLessThanOrEqual(a.y + a.h + 1e-9)
      }
    }
  })
})

describe('center / rotateAbout / anchor', () => {
  it('центр: P + R(θ)·(W/2, H/2)', () => {
    close(center(box()), { x: 120, y: 55 })
    close(center(box({ rot: 90 })), { x: 95, y: 70 })
  })
  it('поворот вокруг центра: центр стоит', () => {
    const t = box({ rot: 20 })
    for (const deg of [0, 33, 90, -135, 180]) {
      const r = rotateAbout(t, deg)
      expect(r.rot).toBe(deg)
      close(center(r), center(t))
    }
  })
  it('туда и обратно возвращает блок', () => {
    const t = box({ rot: 20 })
    const back = rotateAbout(rotateAbout(t, 77), 20)
    expect(back.x).toBeCloseTo(t.x, 9)
    expect(back.y).toBeCloseTo(t.y, 9)
  })
  it('0 → 90 вокруг центра: P уходит вправо-вверх', () => {
    const r = rotateAbout(box(), 90)
    close({ x: r.x, y: r.y }, { x: 125, y: 35 })
  })
  it('anchor: левый — P, средний и правый — вдоль строки', () => {
    close(anchor(box()), { x: 100, y: 50 })
    close(anchor(box({ align: 'center' })), { x: 120, y: 50 })
    close(anchor(box({ align: 'right' })), { x: 140, y: 50 })
    close(anchor(box({ align: 'right', rot: 90 })), { x: 100, y: 90 })
    close(anchor(box({ align: 'center', rot: 180 })), { x: 80, y: 50 })
  })
})

describe('angleAt', () => {
  const c = { x: 500, y: 400 }
  it('курсор над центром — 0°, справа — 90°, снизу — 180°, слева — −90°', () => {
    expect(angleAt(c, { x: 500, y: 300 }, false)).toBe(0)
    expect(angleAt(c, { x: 600, y: 400 }, false)).toBe(90)
    expect(angleAt(c, { x: 500, y: 500 }, false)).toBe(180)
    expect(angleAt(c, { x: 400, y: 400 }, false)).toBe(-90)
  })
  it('по часовой на экране — угол растёт', () => {
    expect(angleAt(c, at(c, 45), false)).toBe(45)
    expect(angleAt(c, at(c, -45), false)).toBe(-45)
    expect(angleAt(c, at(c, 135), false)).toBe(135)
  })
  it('прилипание к 0/±90/180 в пределах 2°, дальше — свободно', () => {
    expect(angleAt(c, at(c, 1.8), false)).toBe(0)
    expect(angleAt(c, at(c, -1.8), false)).toBe(0)
    expect(angleAt(c, at(c, 2.5), false)).toBe(2.5)
    expect(angleAt(c, at(c, 88.5), false)).toBe(90)
    expect(angleAt(c, at(c, -91.5), false)).toBe(-90)
    expect(angleAt(c, at(c, 179), false)).toBe(180)
    expect(angleAt(c, at(c, -179), false)).toBe(180)
    expect(angleAt(c, at(c, 177), false)).toBe(177)
  })
  it('Shift — шаг 15°', () => {
    expect(angleAt(c, at(c, 22), true)).toBe(15)
    expect(angleAt(c, at(c, 23), true)).toBe(30)
    expect(angleAt(c, at(c, -50), true)).toBe(-45)
    expect(angleAt(c, at(c, 179), true)).toBe(180)
    expect(angleAt(c, at(c, 190), true)).toBe(-165)
  })
  it('курсор в самом центре не даёт NaN', () => {
    expect(Number.isFinite(angleAt(c, c, false))).toBe(true)
  })
})

describe('resizeLocal', () => {
  it('0°: правая — только ширина, левая — ширина и x', () => {
    expect(resizeLocal(box(), 'r', 20, 99, 600)).toEqual({ x: 100, y: 50, w: 60 })
    expect(resizeLocal(box(), 'l', -20, 99, 600)).toEqual({ x: 80, y: 50, w: 60 })
  })
  it('90°: ось строки — вниз, берётся dy', () => {
    expect(resizeLocal(box({ rot: 90 }), 'r', 99, 15, 600)).toEqual({ x: 100, y: 50, w: 55 })
    const l = resizeLocal(box({ rot: 90 }), 'l', 99, 15, 600)
    expect(l.w).toBe(25)
    close({ x: l.x, y: l.y }, { x: 100, y: 65 })
  })
  it('левая ручка: правый край стоит на месте при любом угле', () => {
    for (const rot of [0, 30, 90, -135, 180]) {
      const t = box({ rot })
      const edge = toSpread(t, t.w, 0)
      const r = resizeLocal(t, 'l', 7, -4, 600)
      close(toSpread({ ...t, x: r.x, y: r.y, w: r.w }, r.w, 0), edge)
    }
  })
  it('проекция: движение поперёк строки ширину не меняет', () => {
    const t = box({ rot: 30 })
    // (−sin, cos)·k — перпендикуляр к оси строки
    const r = resizeLocal(t, 'r', -0.5 * 10, 0.866025403784 * 10, 600)
    expect(r.w).toBeCloseTo(t.w, 6)
  })
  it('ширина 10..ширина листа', () => {
    expect(resizeLocal(box(), 'r', -500, 0, 600).w).toBe(10)
    expect(resizeLocal(box(), 'r', 5000, 0, 600).w).toBe(600)
    const l = resizeLocal(box(), 'l', 500, 0, 600)
    expect(l).toEqual({ x: 130, y: 50, w: 10 })
    expect(resizeLocal(box(), 'l', -5000, 0, 600).w).toBe(600)
  })
})

describe('moveRotated', () => {
  const none = { v: [], h: [] }
  it('0° — как moveBox', () => {
    const t = box()
    const r = moveRotated(t, 12, -3, none, 0, 600, 300)
    const m = moveBox(t, 12, -3, none, 0, 600, 300)
    expect(r).toEqual(m)
  })
  it('90°: сдвиг описанного прямоугольника переносится на P', () => {
    const r = moveRotated(box({ rot: 90 }), 5, 7, none, 0, 600, 300)
    expect(r.x).toBe(105)
    expect(r.y).toBe(57)
  })
  it('у края разворота прижимается описанный прямоугольник, не P', () => {
    // aabb.x = P.x − 10, значит при упоре в 0 P.x = 10
    const r = moveRotated(box({ rot: 90 }), -500, 0, none, 0, 600, 300)
    expect(r.x).toBe(10)
    const b = moveRotated(box({ rot: 90 }), 0, 9999, none, 0, 600, 300)
    expect(b.y).toBe(300 - 40) // aabb.y = P.y, высота aabb = 40
  })
  it('прилипание идёт по краю описанного прямоугольника', () => {
    // правый край aabb = 100, линия 102, порог 3 → сдвиг +2
    const r = moveRotated(box({ rot: 90 }), 0, 0, { v: [102], h: [] }, 3, 600, 300)
    expect(r.x).toBe(102)
    expect(r.guideX).toBe(102)
  })
})

describe('editorFrame / boxOf', () => {
  it('закреплённая ширина: угол P и ширина W', () => {
    expect(editorFrame(box({ rot: 30 }), false, 284)).toEqual({ x: 100, y: 50, w: 40, maxW: 284, shift: 0, rot: 30 })
  })
  it('по тексту: якорь по выравниванию, ширина по содержимому', () => {
    const f = editorFrame(box({ align: 'right', rot: 90 }), true, 284)
    expect(f.w).toBeNull()
    expect(f.shift).toBe(1)
    close({ x: f.x, y: f.y }, { x: 100, y: 90 })
    expect(editorFrame(box({ align: 'center' }), true, 284).shift).toBe(0.5)
    expect(editorFrame(box({ align: 'left' }), true, 284).shift).toBe(0)
  })
  it('boxOf: неизвестное выравнивание — по центру, как в Go', () => {
    const t = { x: 1, y: 2, w: 3, h: 4, rot: 5, style: { align: '' } }
    expect(boxOf(t)).toEqual({ x: 1, y: 2, w: 3, h: 4, rot: 5, align: 'center' })
    expect(boxOf({ ...t, style: { align: 'right' } }).align).toBe('right')
  })
})

describe('dragAngle', () => {
  const c = { x: 500, y: 400 }
  it('захват точно по оси: как angleAt', () => {
    expect(dragAngle(c, at(c, 40), 0, 0, false)).toBe(40)
  })
  it('захват со смещением: сразу после захвата угол блока не меняется', () => {
    const grab = cursorDeg(c, at(c, 6)) // курсор на 6° в стороне от оси
    expect(dragAngle(c, at(c, 6), grab, 0, false)).toBe(0)
    expect(dragAngle(c, at(c, 46), grab, 0, false)).toBe(40)
  })
  it('Shift — шаг 15° от абсолютного угла', () => {
    expect(dragAngle(c, at(c, 52), 0, 30, true)).toBe(75) // 52 + (30 − 0) = 82 → 75
  })
})

describe('scaleCorner', () => {
  it('правый нижний, 0°: вдоль диагонали растёт, опора (x, y) стоит', () => {
    const r = scaleCorner(box(), 'br', 40, 10) // диагональ (40, 10) → ×2
    expect(r.k).toBeCloseTo(2, 9)
    expect([r.x, r.y]).toEqual([100, 50])
    expect(r.w).toBeCloseTo(80, 9)
    expect(r.h).toBeCloseTo(20, 9)
  })
  it('поперёк диагонали кегль не меняется', () => {
    expect(scaleCorner(box(), 'br', -10, 40).k).toBeCloseTo(1, 9)
  })
  it('левый верхний: наружу (вверх-влево) — растёт, внутрь — уменьшается', () => {
    expect(scaleCorner(box(), 'tl', -40, -10).k).toBeCloseTo(2, 9)
    expect(scaleCorner(box(), 'tl', 20, 5).k).toBeCloseTo(0.5, 9)
  })
  it('левый верхний: правый нижний угол стоит', () => {
    const r = scaleCorner(box(), 'tl', -40, -10) // ×2: (100, 50) → (60, 40), угол (140, 60) на месте
    expect(r.x).toBeCloseTo(60, 9)
    expect(r.y).toBeCloseTo(40, 9)
    expect(r.x + r.w).toBeCloseTo(140, 9)
    expect(r.y + r.h).toBeCloseTo(60, 9)
  })
  it('повёрнутый левый верхний: противоположный угол стоит на экране', () => {
    const b = box({ rot: 30 })
    const far = (t: { x: number; y: number; w: number; h: number }) => {
      const v = rotate(30, t.w, t.h)
      return [t.x + v.x, t.y + v.y]
    }
    const r = scaleCorner(b, 'tl', -25, -20)
    const [ax, ay] = far(b)
    const [bx, by] = far(r)
    expect(bx).toBeCloseTo(ax, 9)
    expect(by).toBeCloseTo(ay, 9)
  })
  it('правый верхний и левый нижний', () => {
    expect(scaleCorner(box(), 'tr', 40, -10).k).toBeCloseTo(2, 9)
    expect(scaleCorner(box(), 'bl', -40, 10).k).toBeCloseTo(2, 9)
  })
  it('повёрнутый: сдвиг переводится в оси блока', () => {
    // 90°: ось строки идёт вниз, «вниз» блока — влево; нижний правый угол: (w, h) → экран (−h, w)
    const r = scaleCorner(box({ rot: 90 }), 'br', -10, 40)
    expect(r.k).toBeCloseTo(2, 9)
    expect([r.x, r.y]).toEqual([100, 50])
  })
  it('пределы k и минимальная ширина', () => {
    expect(scaleCorner(box(), 'br', 400, 100, 0, 3).k).toBe(3)
    expect(scaleCorner(box(), 'br', -39, -9.75, 0.5).k).toBe(0.5)
    expect(scaleCorner(box(), 'br', -40, -10).w).toBe(10) // MIN_BOX_W
  })
  it('нулевой блок не даёт NaN', () => {
    expect(scaleCorner(box({ w: 0, h: 0 }), 'br', 5, 5).k).toBe(1)
  })
})
