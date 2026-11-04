import { describe, expect, it } from 'vitest'
import vectors from '../../../../testdata/parity/grouprot.json'
import { aabb } from '../geom/textbox'
import { groupRot, type GxRotIn, type GxRotOut } from './grouprot'
import { holdOf } from '../geom/rotbox'

interface Vector {
  name: string
  in: GxRotIn
  out: GxRotOut
}

const near = (a: number, b: number, eps = 1e-9) => expect(Math.abs(a - b)).toBeLessThan(eps)

describe('общие векторы с Go (grouprot.json)', () => {
  const list = vectors as Vector[]
  it('векторы прочитаны', () => expect(list.length).toBeGreaterThan(0))
  list.forEach((v) => {
    it(v.name, () => {
      const o = groupRot(v.in)
      near(o.deg, v.out.deg)
      near(o.dx, v.out.dx)
      near(o.dy, v.out.dy)
      expect(o.places.length).toBe(v.out.places.length)
      o.places.forEach((p, i) => {
        const w = v.out.places[i]
        near(p.x, w.x)
        near(p.y, w.y)
        near(p.w, w.w)
        near(p.h, w.h)
        near(p.rot ?? 0, w.rot ?? 0)
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

describe('поворот группы', () => {
  const inp = (deg: number): GxRotIn => ({
    sw: 562,
    ph: 284,
    places: [
      { x: 100, y: 100, w: 100, h: 60 },
      { x: 240, y: 110, w: 80, h: 50, rot: 30 },
    ],
    texts: [{ x: 150, y: 190, w: 120, h: 20, rot: -15, size: 14 }],
    deg,
    cx: 220,
    cy: 150,
  })

  it('угол не упирается и прибавляется к углам членов', () => {
    const o = groupRot(inp(25))
    expect(o.deg).toBe(25)
    expect(o.places[0].rot).toBe(25)
    expect(o.places[1].rot).toBe(55)
    expect(o.texts[0].rot).toBe(10)
  })

  it('холостой: 0° и 360° ничего не меняют', () => {
    for (const d of [0, 360, 0.04]) {
      const o = groupRot(inp(d))
      expect(o.deg).toBe(0)
      expect(o.places).toEqual(inp(d).places)
    }
  })

  it('у правого края группа сдвигается внутрь целиком, всё внутри', () => {
    const i = inp(30)
    i.places = i.places.map((p) => ({ ...p, x: p.x + 300 }))
    i.texts = i.texts.map((t) => ({ ...t, x: t.x + 300 }))
    i.cx += 300
    const o = groupRot(i)
    expect(o.dx).toBeLessThan(0)
    for (const p of o.places) {
      const h = holdOf(p)
      expect(h.x).toBeGreaterThanOrEqual(-1e-9)
      expect(h.x + h.w).toBeLessThanOrEqual(562 + 1e-9)
    }
    const a = aabb(o.texts[0])
    expect(a.x + a.w).toBeLessThanOrEqual(562 + 1e-9)
  })
})
