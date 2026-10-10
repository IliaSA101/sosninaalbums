import { describe, expect, it } from 'vitest'
import {
  clampZoom,
  fit,
  HOLD_MS,
  MAX_ZOOM,
  MIN_ZOOM,
  panBy,
  percent,
  placement,
  posZoom,
  spaceTap,
  wheelFactor,
  isZoomWheel,
  wheelPan,
  zoomAt,
  zoomPos,
  type Viewport,
} from './zoom'

// Окно 1000×600 px, разворот 600×300 мм: k0 = 1 px/мм → при z = 1 он занимает 600×300 px по центру.
const v: Viewport = { w: 1000, h: 600, k0: 1, sw: 600, ph: 300 }

describe('fit / placement', () => {
  it('fit — центр разворота, масштаб 1', () => expect(fit(600, 300)).toEqual({ z: 1, cx: 300, cy: 150 }))
  it('при z = 1 разворот стоит по центру окна', () => {
    expect(placement(fit(600, 300), v)).toEqual({ left: 200, top: 150, width: 600, height: 300 })
  })
  it('результат — целые px', () => {
    const p = placement({ z: 1.37, cx: 300, cy: 150 }, { ...v, k0: 0.913 })
    for (const n of Object.values(p)) expect(Number.isInteger(n)).toBe(true)
  })
})

describe('clampZoom', () => {
  it('масштаб в 1..8, NaN — 1', () => {
    expect(clampZoom({ z: 0.2, cx: 300, cy: 150 }, v).z).toBe(MIN_ZOOM)
    expect(clampZoom({ z: 99, cx: 300, cy: 150 }, v).z).toBe(MAX_ZOOM)
    expect(clampZoom({ z: NaN, cx: NaN, cy: NaN }, v)).toEqual({ z: 1, cx: 300, cy: 150 })
  })
  it('разворот уже окна — по центру; шире — край окна упирается в край разворота', () => {
    expect(clampZoom({ z: 1, cx: 10, cy: 10 }, v)).toEqual({ z: 1, cx: 300, cy: 150 })
    // z = 4: k = 4, полуокно 125×75 мм
    expect(clampZoom({ z: 4, cx: 0, cy: 0 }, v)).toEqual({ z: 4, cx: 125, cy: 75 })
    expect(clampZoom({ z: 4, cx: 999, cy: 999 }, v)).toEqual({ z: 4, cx: 475, cy: 225 })
  })
  it('по осям независимо', () => {
    // z = 1.5: ширина 900 < 1000 и высота 450 < 600 — внутри окна: cx ≥ 600 − 1000/3, cy ≥ 300 − 200
    const n = clampZoom({ z: 1.5, cx: 0, cy: 0 }, v)
    expect(n.cx).toBeCloseTo(600 - 1000 / 3)
    expect(n.cy).toBeCloseTo(100)
    // z = 2.5: ширина 1500 > 1000 и высота 750 > 600 — клемп
    const c = clampZoom({ z: 2.5, cx: 0, cy: 0 }, v)
    expect(c.cx).toBeCloseTo(200)
    expect(c.cy).toBeCloseTo(120)
  })
  it('мусор в окне не ломает', () => {
    expect(clampZoom({ z: 2, cx: 5, cy: 6 }, { ...v, k0: 0 })).toEqual({ z: 2, cx: 5, cy: 6 })
  })
})

// Боковые панели (U14): pad 150 px — свободная зона 150..850, разворот при z = 1 — 200..800.
describe('clampZoom с pad', () => {
  const vp: Viewport = { ...v, pad: 150 }
  it('при z = 1 — по центру, как без pad', () => {
    expect(clampZoom({ z: 1, cx: 0, cy: 999 }, vp)).toEqual({ z: 1, cx: 300, cy: 150 })
  })
  it('уже свободной зоны — внутри неё', () => {
    for (const cx of [-1e6, 1e6]) {
      const p = placement(clampZoom({ z: 1.1, cx, cy: 150 }, vp), vp)
      expect(p.left).toBeGreaterThanOrEqual(150)
      expect(p.left + p.width).toBeLessThanOrEqual(850)
    }
  })
  it('шире — край разворота доходит до pad от края окна, не дальше', () => {
    const l = placement(clampZoom({ z: 4, cx: -1e6, cy: 150 }, vp), vp)
    expect(l.left).toBe(150)
    const r = placement(clampZoom({ z: 4, cx: 1e6, cy: 150 }, vp), vp)
    expect(r.left + r.width).toBe(850)
  })
  it('на границе «уже/шире» cx непрерывен', () => {
    const z0 = 700 / 600 // ширина разворота = свободной зоне
    for (const cx of [-1e6, 1e6]) {
      const a = clampZoom({ z: z0 - 1e-6, cx, cy: 150 }, vp).cx
      const b = clampZoom({ z: z0 + 1e-6, cx, cy: 150 }, vp).cx
      expect(Math.abs(a - b)).toBeLessThan(1e-3)
      expect(a).toBeCloseTo(300, 2)
    }
  })
  it('мусорный pad — как 0', () => {
    expect(clampZoom({ z: 4, cx: 0, cy: 0 }, { ...v, pad: NaN })).toEqual(clampZoom({ z: 4, cx: 0, cy: 0 }, v))
  })
})

describe('zoomAt', () => {
  const mmAt = (s: { z: number; cx: number; cy: number }, x: number, y: number) => ({
    x: s.cx + (x - v.w / 2) / (v.k0 * s.z),
    y: s.cy + (y - v.h / 2) / (v.k0 * s.z),
  })
  it('мм под курсором остаётся под ним', () => {
    const s0 = fit(600, 300)
    const before = mmAt(s0, 600, 250)
    const s1 = zoomAt(s0, v, 3, 600, 250)
    expect(s1.z).toBe(3)
    const after = mmAt(s1, 600, 250)
    expect(after.x).toBeCloseTo(before.x, 9)
    expect(after.y).toBeCloseTo(before.y, 9)
  })
  it('от 100 % первые шаги держат точку и по горизонтали (pad 0 и pad 150)', () => {
    for (const vp of [v, { ...v, pad: 150 }]) {
      let s = fit(600, 300)
      const before = mmAt(s, 750, 400)
      for (let i = 0; i < 2; i++) s = zoomAt(s, vp, 1.04, 750, 400)
      expect(s.z).toBeGreaterThan(1)
      const after = mmAt(s, 750, 400)
      expect(after.x).toBeCloseTo(before.x, 9)
      expect(after.y).toBeCloseTo(before.y, 9)
    }
  })
  it('курсор в центре окна — центр не двигается', () => {
    expect(zoomAt(fit(600, 300), v, 2, 500, 300)).toEqual({ z: 2, cx: 300, cy: 150 })
  })
  it('предел масштаба и возврат к 1 в центр разворота', () => {
    let s = fit(600, 300)
    for (let i = 0; i < 20; i++) s = zoomAt(s, v, 2, 700, 100)
    expect(s.z).toBe(MAX_ZOOM)
    for (let i = 0; i < 20; i++) s = zoomAt(s, v, 0.5, 700, 100)
    expect(s).toEqual({ z: 1, cx: 300, cy: 150 })
  })
  it('у края окна разворот не оставляет пустых полей', () => {
    const s = zoomAt(fit(600, 300), v, 4, 1000, 600)
    const p = placement(s, v)
    expect(p.left + p.width).toBeGreaterThanOrEqual(v.w)
    expect(p.top + p.height).toBeGreaterThanOrEqual(v.h)
    expect(p.left).toBeLessThanOrEqual(0)
  })
  it('мусорный factor и курсор — без изменений', () => {
    const s = { z: 2, cx: 300, cy: 150 }
    expect(zoomAt(s, v, NaN, 1, 1)).toEqual(s)
    expect(zoomAt(s, v, 0, 1, 1)).toEqual(s)
    expect(zoomAt(s, v, 2, NaN, 1)).toEqual(s)
  })
})

describe('panBy', () => {
  it('содержимое едет за курсором: сдвиг вправо — центр влево', () => {
    const s = panBy({ z: 4, cx: 300, cy: 150 }, v, 40, 0)
    expect(s.cx).toBeCloseTo(290)
    expect(s.cy).toBe(150)
  })
  it('упирается в край разворота', () => {
    expect(panBy({ z: 4, cx: 300, cy: 150 }, v, 1e6, 1e6)).toEqual({ z: 4, cx: 125, cy: 75 })
  })
  it('при z = 1 ничего не двигает', () => {
    expect(panBy(fit(600, 300), v, 50, 50)).toEqual(fit(600, 300))
  })
})

describe('wheelFactor', () => {
  it('вверх — крупнее, вниз — мельче, 0 — единица', () => {
    expect(wheelFactor(-100, 0)).toBeGreaterThan(1)
    expect(wheelFactor(100, 0)).toBeLessThan(1)
    expect(wheelFactor(0, 0)).toBe(1)
    expect(wheelFactor(-100, 0)).toBeCloseTo(Math.exp(0.4), 12)
  })
  it('шаг ограничен 100, строки — по 40', () => {
    expect(wheelFactor(-5000, 0)).toBeCloseTo(Math.exp(0.4), 12)
    expect(wheelFactor(-1, 1)).toBeCloseTo(Math.exp(40 * 0.004), 12)
    expect(wheelFactor(NaN, 0)).toBe(1)
  })
  it('щипок тачпада (мелкая дробная дельта) — плавно, а щелчок мыши — заметно', () => {
    expect(wheelFactor(-3.5, 0)).toBeLessThan(1.02)
    expect(wheelFactor(-100, 0)).toBeGreaterThan(1.4)
  })
  it('обратные дельты взаимно отменяются', () => expect(wheelFactor(80, 0) * wheelFactor(-80, 0)).toBeCloseTo(1, 12))
})

describe('spaceTap / percent', () => {
  it('короткий Пробел без панорамы — нажатие', () => {
    expect(spaceTap(120, false)).toBe(true)
    expect(spaceTap(HOLD_MS - 1, false)).toBe(true)
  })
  it('долгий или с панорамой — нет', () => {
    expect(spaceTap(HOLD_MS, false)).toBe(false)
    expect(spaceTap(100, true)).toBe(false)
    expect(spaceTap(-1, false)).toBe(false)
  })
  it('percent', () => {
    expect(percent(1)).toBe(100)
    expect(percent(1.7)).toBe(170)
    expect(percent(1.004)).toBe(100)
  })
})

const w = (deltaX: number, deltaY: number, ctrlKey = false, deltaMode = 0) => ({ deltaX, deltaY, deltaMode, ctrlKey })

describe('isZoomWheel', () => {
  it('Ctrl или Cmd — зум (и щипок тачпада, и колесо мыши), без них — нет', () => {
    for (const d of [-2.5, 3.1416, 100, -120]) {
      expect(isZoomWheel(w(0, d, true)), String(d)).toBe(true)
      expect(isZoomWheel(w(0, d)), String(d)).toBe(false)
    }
    expect(isZoomWheel(w(100, 0))).toBe(false) // Shift+колесо — не зум
    expect(isZoomWheel({ ...w(0, -3, false, 1), metaKey: true })).toBe(true)
  })
})

describe('wheelPan', () => {
  it('сдвиг: содержимое против прокрутки, строки — по 40 px; Shift+колесо — по X, колесо — по Y', () => {
    expect(wheelPan(w(2, -3))).toEqual({ dx: -2, dy: 3 })
    expect(wheelPan(w(0, 1, false, 1))).toEqual({ dx: 0, dy: -40 })
    expect(wheelPan(w(100, 0))).toEqual({ dx: -100, dy: 0 })
    expect(wheelPan(w(0, 100))).toEqual({ dx: 0, dy: -100 })
  })
})

describe('zoomPos / posZoom', () => {
  it('200 и 400 % на третях шкалы, края — 0 и 1', () => {
    expect(zoomPos(1)).toBe(0)
    expect(zoomPos(2)).toBeCloseTo(1 / 3, 10)
    expect(zoomPos(4)).toBeCloseTo(2 / 3, 10)
    expect(zoomPos(8)).toBe(1)
    expect(posZoom(0)).toBe(1)
    expect(posZoom(1)).toBe(MAX_ZOOM)
    expect(posZoom(1 / 3)).toBeCloseTo(2, 10)
  })
  it('функции взаимно обратны', () => {
    for (const z of [1, 1.25, 1.7, 2, 3.3, 5, 7.9, 8]) expect(posZoom(zoomPos(z))).toBeCloseTo(z, 10)
    for (const t of [0, 0.1, 0.5, 0.77, 1]) expect(zoomPos(posZoom(t))).toBeCloseTo(t, 10)
  })
  it('вне диапазона и не число — к краю', () => {
    expect(zoomPos(0.2)).toBe(0)
    expect(zoomPos(100)).toBe(1)
    expect(zoomPos(NaN)).toBe(0)
    expect(posZoom(-1)).toBe(1)
    expect(posZoom(7)).toBe(MAX_ZOOM)
    expect(posZoom(NaN)).toBe(1)
  })
})
