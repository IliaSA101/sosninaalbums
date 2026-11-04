import { describe, expect, it } from 'vitest'
import vectors from '../../../../testdata/parity/groupaxes.json'
import { groupAxes, type GxAxesIn, type GxAxesOut } from './groupaxes'

interface Vector {
  name: string
  in: GxAxesIn
  out: GxAxesOut
}

const near = (a: number, b: number, eps = 1e-9) => expect(Math.abs(a - b)).toBeLessThan(eps)

describe('общие векторы с Go (groupaxes.json)', () => {
  const list = vectors as Vector[]
  it('векторы прочитаны', () => expect(list.length).toBeGreaterThan(0))
  list.forEach((v) => {
    it(v.name, () => {
      const o = groupAxes(v.in)
      near(o.kx, v.out.kx)
      near(o.ky, v.out.ky)
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
        near(t.al ?? 0, w.al ?? 0)
      })
    })
  })
})

describe('масштаб по осям', () => {
  const base = (kx: number, ky: number): GxAxesIn => ({
    sw: 562,
    ph: 284,
    places: [{ x: 50, y: 50, w: 100, h: 60 }],
    texts: [{ x: 50, y: 120, w: 100, h: 20, rot: 0, size: 14, al: 0.5 }],
    kx,
    ky,
    phi: 0,
    ax: 50,
    ay: 50,
  })

  it('растяжка по ширине: кегль стоит, середина текста едет с фото', () => {
    const o = groupAxes(base(1.5, 1))
    expect(o.texts[0].size).toBe(14)
    near(o.texts[0].x + o.texts[0].w / 2, 50 + 1.5 * 50, 0.011)
    expect(o.places[0].w).toBe(150)
    expect(o.places[0].h).toBe(60)
  })

  it('сжатие по ширине вдвое: кегль меньше', () => {
    expect(groupAxes(base(0.5, 1)).texts[0].size).toBe(7)
  })

  it('нижний предел: места не меньше 5 мм, кегль не меньше 6', () => {
    const o = groupAxes(base(0.001, 0.001))
    expect(o.places[0].w).toBeGreaterThanOrEqual(5)
    expect(o.places[0].h).toBeGreaterThanOrEqual(5)
    expect(o.texts[0].size).toBeGreaterThanOrEqual(6)
  })

  it('за край не растёт: множитель уменьшается до того, что влезает', () => {
    const o = groupAxes(base(10, 1))
    expect(o.kx).toBeLessThan(10)
    expect(o.places[0].x + o.places[0].w).toBeLessThanOrEqual(562 + 1e-9)
  })
})
