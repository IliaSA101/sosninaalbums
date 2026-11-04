import { describe, expect, it } from 'vitest'
import { MINI_H, MINI_IN_GAP, miniPlace, zoomLift } from './minipanel'

describe('zoomLift: полоса внутри у низа', () => {
  const box = { x: 0, y: 0, w: 300, h: 150 } // ppm 2: 600×300 px, лист 600×300
  it('zoomLift: ползунок масштаба над полосой (владелец 2026-10-17)', () => {
    const p = miniPlace(box, 2, 600, 300, 100)
    const zb = { x: 0, y: 0, w: 600, h: 300 } // низ 300 px
    expect(zoomLift(p, zb)).toBe(300 - p.top + 8)
    // вне 'inside' и с целиком видимым низом — прежние 16 px
    expect(zoomLift({ top: 10, where: 'below' }, zb)).toBe(16)
    expect(zoomLift({ top: 10, where: 'above' }, zb)).toBe(16)
    expect(zoomLift({ top: 10, where: 'top' }, zb)).toBe(16)
    // у повёрнутого места оси чужие: вне 'inside' — 16, на 'inside' — подъём над полосой (ревью 0.14.1)
    expect(zoomLift({ top: 10, where: 'below' }, { ...zb, rot: 15 })).toBe(16)
    expect(zoomLift(p, { ...zb, rot: 15 })).toBe(MINI_IN_GAP + MINI_H + 8)
  })
  it('зум: повёрнутое место с полосой у низа — ползунок выше полосы', () => {
    const v = { left: 0, top: 0, right: 1000, bottom: 400 }
    const r = (3 * Math.PI) / 180
    const [W, H] = [200, 220] // мм, ppm 2 → 400×440 px; низ рамки в 20 px под низом окна: полоса стоит у низа видимой части
    const aw = W * Math.cos(r) + H * Math.sin(r)
    const ah = W * Math.sin(r) + H * Math.cos(r)
    const b = { x: 250 - aw / 2, y: 210 - ah, w: aw, h: ah }
    const p = miniPlace(b, 2, 1000, 1000, 290, undefined, 0, v)
    expect(p.where).toBe('inside')
    const zb = { x: 0, y: 0, w: W * 2, h: H * 2, rot: 3 }
    zb.x = 500 - zb.w / 2
    zb.y = 2 * (210 - ah / 2) - zb.h / 2 // тот же центр, что у описанной рамки (мм → px)
    const lift = zoomLift(p, zb, v)
    // прежнее место (над полосой «в чужих осях») лежит на полосе: ветка maxY в zoomFits поднимает выше
    expect(lift).toBeGreaterThan(MINI_IN_GAP + MINI_H + 8)
    // сам результат: углы ползунка (высота 16) выше верха полосы с зазором 8
    const a = (3 * Math.PI) / 180
    const [cx, cy] = [zb.x + zb.w / 2, zb.y + zb.h / 2]
    const hw = Math.min(zb.w * 0.6, 170) / 2
    for (const dx of [-hw, hw])
      for (const dy of [zb.h / 2 - lift - 16, zb.h / 2 - lift]) {
        expect(cy + dx * Math.sin(a) + dy * Math.cos(a)).toBeLessThanOrEqual(p.top - 8)
        expect(cx + dx * Math.cos(a) - dy * Math.sin(a)).toBeGreaterThanOrEqual(v.left)
      }
  })
})

// Часть Б 070: ползунок масштаба кадра не уходит за край экрана (owner-ux §1.6 «не прячется»).
describe('zoomLift: столбец сбоку (108)', () => {
  const view = { left: 0, top: 0, right: 1000, bottom: 400 }
  const b = { x: 100, y: -200, w: 400, h: 1000 } // низ 800 px, окно кончается на 400
  it('zoomLift: столбец сбоку — ползунок у низа видимой части, не над панелью (владелец 2026-10-23)', () => {
    for (const where of ['right', 'left', 'inright'] as const) expect(zoomLift({ top: 150, where }, b, view), where).toBe(800 - 400 + 16)
  })
  it('низ рамки виден — прежние 16 px', () => {
    expect(zoomLift({ top: 10, where: 'right' }, { x: 100, y: 50, w: 300, h: 200 }, view)).toBe(16)
  })
})

describe('zoomLift: видимая часть рамки (070 Б)', () => {
  const view = { left: 0, top: 0, right: 1000, bottom: 400 }
  const below = { top: 0, where: 'above' as const } // панель не внутри: ползунок считается от края видимой области
  it('рамка целиком видна — прежние 16 px', () => {
    expect(zoomLift(below, { x: 100, y: 50, w: 300, h: 200 }, view)).toBe(16)
  })
  it('низ рамки за экраном — ползунок у нижнего края видимой части', () => {
    const b = { x: 100, y: -200, w: 400, h: 1000 } // низ 800 px, окно кончается на 400
    expect(zoomLift(below, b, view)).toBe(800 - 400 + 16)
  })
  it('панель внутри у низа — ползунок над ней (как прежде)', () => {
    const b = { x: 100, y: -200, w: 400, h: 1000 }
    const p = { top: 400 - MINI_IN_GAP - MINI_H, where: 'inside' as const }
    expect(zoomLift(p, b, view)).toBe(800 - p.top + 8)
  })
  it('нет видимой части (рамка целиком ниже или выше окна) — прежнее место', () => {
    expect(zoomLift(below, { x: 0, y: 500, w: 300, h: 300 }, view)).toBe(16)
    expect(zoomLift(below, { x: 0, y: -600, w: 300, h: 300 }, view)).toBe(16)
    // без видимой области (ещё не измерена) — тоже
    expect(zoomLift(below, { x: 0, y: -200, w: 400, h: 1000 })).toBe(16)
  })
  it('видна лишь узкая полоска у верха — подъём не выше самой видимой части', () => {
    // видимые 10 px (390..400 при y=390): ползунку (16) не поместиться — остаётся прежнее
    expect(zoomLift(below, { x: 0, y: 390, w: 300, h: 600 }, view)).toBe(16)
  })
  it('поворот 30°: ползунок поднимается вдоль оси места, пока не окажется целиком в видимой области', () => {
    const b = { x: 300, y: -300, w: 400, h: 900, rot: 30 } // центр (500, 150), низ места — за нижним краем окна
    const lift = zoomLift(below, b, view)
    expect(lift).toBeGreaterThan(16)
    // проверяем сам результат: углы ползунка внутри окна
    const a = (30 * Math.PI) / 180
    const hw = Math.min(b.w * 0.6, 170) / 2
    for (const dx of [-hw, hw])
      for (const dy of [b.h / 2 - lift - 16, b.h / 2 - lift]) {
        const x = 500 + dx * Math.cos(a) - dy * Math.sin(a)
        const y = 150 + dx * Math.sin(a) + dy * Math.cos(a)
        expect(x).toBeGreaterThanOrEqual(view.left)
        expect(x).toBeLessThanOrEqual(view.right)
        expect(y).toBeGreaterThanOrEqual(view.top)
        expect(y).toBeLessThanOrEqual(view.bottom)
      }
  })
  it('поворот 90°: «низ» места смотрит влево — ползунок поднимается, пока не войдёт в окно', () => {
    const b = { x: 100, y: 200, w: 300, h: 900, rot: 90 } // центр (250, 650); локальный низ → влево, в мир x = 250 − (h/2 − up)
    const lift = zoomLift(below, b, view)
    // в мире ползунок: x = 250 − (450 − up) ... центр по y — 650 за экраном: подходящей точки нет → прежнее
    expect(lift).toBe(16)
    const c = { x: 100, y: -250, w: 300, h: 900, rot: 90 } // центр (250, 200): виден; «низ» слева за краем x = −200
    const l2 = zoomLift(below, c, view)
    expect(l2).toBeGreaterThan(16)
    // локальный низ (dy>0) → мир x = 250 − dy; ползунок целиком правее левого края окна
    expect(250 - (450 - l2)).toBeGreaterThanOrEqual(view.left)
  })
  it('поворот, подходящей точки нет — прежнее место (16, внутри — над полосой)', () => {
    const tiny = { x: 0, y: 0, w: 40, h: 30, rot: 20 }
    expect(zoomLift(below, tiny, { left: 500, top: 500, right: 510, bottom: 510 })).toBe(16)
    expect(zoomLift({ top: 100, where: 'inside' }, tiny, { left: 500, top: 500, right: 510, bottom: 510 })).toBe(MINI_IN_GAP + MINI_H + 8)
  })
})
