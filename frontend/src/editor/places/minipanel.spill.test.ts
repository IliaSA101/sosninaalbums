import { describe, expect, it } from 'vitest'
import { arcWhere, MINI_GAP, MINI_H, miniPlace, sameView } from './minipanel'
import { FIT_BOTTOM, FIT_TOP, fit, fitH, placement } from '../zoom/zoom'

// owner-ux §1.9 (владелец 2026-10-21): полосы под листом нет — мини-панель у нижнего фото встаёт под ним поверх ленты
// разворотов (view.spill — низ ленты в осях листа), если низ фото виден на холсте.
describe('мини-панель поверх ленты разворотов (095)', () => {
  const BAR = 32
  const STRIP = 80 // лента разворотов под холстом, px
  // Как Stage + MiniPanel: «вписать» листа 600×300 мм в окно W×H; видимая область — окно холста в осях листа.
  const fitted = (W: number, H: number) => {
    const h = H - BAR
    const k0 = Math.min((W - 32) / 600, fitH(h) / 300)
    const p = placement(fit(600, 300), { w: W, h, k0, sw: 600, ph: 300, padB: FIT_BOTTOM - FIT_TOP })
    return { ppm: k0, view: { left: -p.left, top: -p.top, right: W - p.left, bottom: h - p.top, spill: h + STRIP - p.top } }
  }
  const rot = (b: { x: number; y: number; w: number }, ppm: number) => ({ x: (b.x + b.w / 2) * ppm, y: b.y * ppm - 24 })

  it('два вертикальных 182,7×274 при Y = 13 — панель под фото поверх ленты (владелец 2026-10-21)', () => {
    const sizes: [number, number][] = [[1280, 760]]
    for (let a = 400; a <= 1000; a += 100) for (let b = 900; b <= 2000; b += 100) sizes.push([a, b], [b, a])
    let over = 0
    for (const [W, H] of sizes) {
      const { ppm, view } = fitted(W, H)
      for (const x of [100, 317.3]) {
        const b = { x, y: 13, w: 182.7, h: 274 }
        const p = miniPlace(b, ppm, 600 * ppm, 300 * ppm, 200, rot(b, ppm), 0, view)
        expect(p.where, `окно ${W}×${H}`).toBe('below')
        expect(p.top).toBeCloseTo((13 + 274) * ppm + MINI_GAP, 6)
        expect(p.top + MINI_H, `окно ${W}×${H}`).toBeLessThanOrEqual(view.spill)
        if (p.top + MINI_H > view.bottom) over++
        // дуга наклона считается от того же места
        expect(arcWhere(b, ppm, 600 * ppm, 300 * ppm, 200, 0, view, [])).toBe('below')
      }
    }
    expect(over).toBeGreaterThan(0) // в малых окнах панель действительно заходит на ленту
  })

  it('низ фото за краем холста — без захода на ленту', () => {
    // зум: фото 1000×600 px, окно кончается на 450, лента — до 530
    const view = { left: -100, top: -50, right: 700, bottom: 450, spill: 530 }
    const p = miniPlace({ x: 20, y: 0, w: 200, h: 240 }, 2, 600, 400, 100, undefined, 0, view)
    expect(p.where).not.toBe('below')
    expect(p.top + MINI_H).toBeLessThanOrEqual(450)
  })

  it('ленты нет — над фото', () => {
    const b = { x: 20, y: 150, w: 100, h: 50 } // низ 400 px, окно кончается на 420
    const view = { left: -100, top: -50, right: 700, bottom: 420 }
    expect(miniPlace(b, 2, 600, 400, 100, undefined, 0, view).where).toBe('above')
    // с лентой под холстом — под фото поверх неё
    expect(miniPlace(b, 2, 600, 400, 100, undefined, 0, { ...view, spill: 500 })).toMatchObject({ where: 'below', top: 408 })
  })

  it('sameView различает низ ленты', () => {
    const v = { left: 0, top: 0, right: 10, bottom: 10 }
    expect(sameView(v, { ...v })).toBe(true)
    expect(sameView(v, { ...v, spill: 20 })).toBe(false)
  })
})
