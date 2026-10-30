import { describe, expect, it } from 'vitest'
import vectors from '../../../../testdata/parity/groupxf.json'
import { anchorOf, bounds, commonAngle, cornerK, groupBox, groupXf, type GxIn, type GxOut } from './groupxf'

interface Vector {
  name: string
  in: GxIn
  out: GxOut
}

const near = (a: number, b: number) => expect(Math.abs(a - b)).toBeLessThan(1e-9)

describe('общие векторы с Go (groupxf.json)', () => {
  const list = vectors as Vector[]
  it('векторы прочитаны', () => expect(list.length).toBeGreaterThan(0))
  it('есть векторы с повёрнутым местом', () => expect(list.some((v) => v.in.places.some((p) => (p.rot ?? 0) !== 0))).toBe(true))
  list.forEach((v) => {
    it(v.name, () => {
      const o = groupXf(v.in)
      near(o.k, v.out.k)
      near(o.dx, v.out.dx)
      near(o.dy, v.out.dy)
      expect(o.places.length).toBe(v.out.places.length)
      o.places.forEach((p, i) => {
        near(p.x, v.out.places[i].x)
        near(p.y, v.out.places[i].y)
        near(p.w, v.out.places[i].w)
        near(p.h, v.out.places[i].h)
        near(p.rot ?? 0, v.out.places[i].rot ?? 0) // угол стоит
      })
      expect(o.texts.length).toBe(v.out.texts.length)
      o.texts.forEach((t, i) => {
        const w = v.out.texts[i]
        near(t.x, w.x)
        near(t.y, w.y)
        near(t.w, w.w)
        near(t.h, w.h)
        near(t.rot, w.rot)
        near(t.size, w.size)
      })
    })
  })
})

describe('повёрнутое место входит в группу своим hold', () => {
  const inp = (rot: number): GxIn => ({
    sw: 562,
    ph: 284,
    places: [{ x: 100, y: 100, w: 100, h: 60, rot }],
    texts: [],
    k: 1,
    ax: 0,
    ay: 0,
    dx: 1000,
    dy: 0,
  })
  it('90° у правого края: сдвиг до hold, а не до неповёрнутого прямоугольника', () => {
    const o = groupXf(inp(90))
    // hold — квадрат 60×60 вокруг центра: центр упирается в 562 − 30, значит x = 532 − 50
    near(o.places[0].x, 482)
    expect(o.places[0].rot).toBe(90)
  })
  it('без поворота — сам прямоугольник', () => near(groupXf(inp(0)).places[0].x, 462))
})

describe('bounds', () => {
  it('пусто — null', () => expect(bounds([])).toBeNull())
  it('описанный прямоугольник', () => {
    expect(
      bounds([
        { x: 10, y: 20, w: 5, h: 5 },
        { x: 30, y: 10, w: 10, h: 10 },
      ]),
    ).toEqual({ x: 10, y: 10, w: 30, h: 15 })
  })
})

describe('масштаб за угол', () => {
  const bb = { x: 10, y: 10, w: 100, h: 50 }
  it('неподвижный угол — противоположный', () => {
    expect(anchorOf(bb, 'nw')).toEqual({ x: 110, y: 60 })
    expect(anchorOf(bb, 'se')).toEqual({ x: 10, y: 10 })
    expect(anchorOf(bb, 'ne')).toEqual({ x: 10, y: 60 })
    expect(anchorOf(bb, 'sw')).toEqual({ x: 110, y: 10 })
  })
  it('сдвиг угла вдоль диагонали даёт k, поперёк — 1', () => {
    near(cornerK(bb, 'se', 100, 50), 2)
    near(cornerK(bb, 'se', -50, -25), 0.5)
    near(cornerK(bb, 'se', 50, -100), 1) // перпендикулярно диагонали (100, 50)
  })
  it('угол nw: тянуть наружу — больше', () => {
    expect(cornerK(bb, 'nw', -10, -5)).toBeGreaterThan(1)
  })
  it('вырожденная рамка — 1', () => near(cornerK({ x: 5, y: 5, w: 0, h: 0 }, 'se', 10, 10), 1))
})

describe('рамка и общий угол', () => {
  it('groupBox: пусто — null', () => expect(groupBox([], [])).toBeNull())
  it('groupBox: повёрнутое место — описанным, текст — textRect', () => {
    const r = groupBox([{ x: 100, y: 100, w: 40, h: 20, rot: 90 }], [{ x: 10, y: 10, w: 30, h: 10, rot: 0, size: 12 }])
    // 90°: aabb места — 20×40 вокруг центра (120, 110) → x 110..130, y 90..130; текст x 10..40, y 10..20.
    expect(r).toEqual({ x: 10, y: 10, w: 120, h: 120 })
  })
  it('commonAngle: одинаковые — угол, разные или пусто — null', () => {
    expect(commonAngle([15, 15, 15])).toBe(15)
    expect(commonAngle([0, 0])).toBe(0)
    expect(commonAngle([15, 0])).toBeNull()
    expect(commonAngle([])).toBeNull()
  })
})
