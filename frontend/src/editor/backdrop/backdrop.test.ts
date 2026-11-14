import { describe, expect, it } from 'vitest'
import { backdropCss, backdropLost, backdropOf, turnBackdropBox, zoneScopes } from './backdrop'
import { center, type TextBox } from '../../base/geom/textbox'
import vectors from '../../../../testdata/parity/bdturn.json'
import type { BackdropView, SpreadView } from '../../base/core/types'

const bd = (scope: BackdropView['scope']) => ({ scope, photo: 'p' }) as BackdropView
const sp = (...b: BackdropView[]) => ({ id: 's', backdrops: b }) as unknown as SpreadView

describe('zoneScopes', () => {
  it('нет фона и цвета едины — одна зона на разворот', () => {
    expect(zoneScopes([], false)).toEqual(['spread'])
    expect(zoneScopes(null, false)).toEqual(['spread'])
    expect(zoneScopes([bd('spread')], false)).toEqual(['spread'])
  })
  it('цвета по страницам или фон по страницам — две зоны', () => {
    expect(zoneScopes([], true)).toEqual(['left', 'right'])
    expect(zoneScopes([bd('right')], false)).toEqual(['left', 'right'])
  })
})

describe('backdropOf', () => {
  it('ищет по области', () => {
    expect(backdropOf(sp(bd('left')), 'left')?.scope).toBe('left')
    expect(backdropOf(sp(bd('left')), 'right')).toBeUndefined()
    expect(backdropOf(undefined, 'spread')).toBeUndefined()
  })
})

describe('backdropLost', () => {
  it('фон второго теряется, только если фоны есть у обоих', () => {
    expect(backdropLost(sp(bd('spread')), sp(bd('left')))).toBe(true)
    expect(backdropLost(sp(), sp(bd('left')))).toBe(false)
    expect(backdropLost(sp(bd('spread')), sp())).toBe(false)
    expect(backdropLost(undefined, undefined)).toBe(false)
  })
})

describe('backdropCss (спека 045): transform картинки фона', () => {
  const css = (b: Partial<BackdropView>) => backdropCss({ frame: { s: 1, x: 0, y: 0 }, flipH: false, flipV: false, ...b } as BackdropView)
  it('без наклона и зеркала — transform не задан', () => {
    expect(css({})).toBeUndefined()
  })
  it('наклон и зеркало: поворот идёт до зеркала', () => {
    expect(css({ frame: { s: 1, x: 0, y: 0, r: 4.5 } })).toBe('rotate(4.5deg)')
    expect(css({ frame: { s: 1, x: 0, y: 0, r: 4.5 }, flipH: true })).toBe('rotate(4.5deg) scale(-1, 1)')
    expect(css({ frame: { s: 1, x: 0, y: 0, r: -3 }, flipV: true })).toBe('rotate(-3deg) scale(1, -1)')
  })
  it('только зеркало — как до 0.12', () => {
    expect(css({ flipH: true, flipV: true })).toBe('scale(-1, -1)')
  })
})

describe('turnBackdropBox: поворот рамки фона (QA 061-3, 070 Г)', () => {
  const box = { x: 100, y: 20, w: 12, h: 200, rot: 0 } // корешок: 12 мм в ширину
  const near = (a: number, b: number) => expect(a).toBeCloseTo(b, 6)

  it('корешок 0 → 90: w и h поменялись, центр тот же', () => {
    const r = turnBackdropBox(box, 90)
    expect([r.w, r.h, r.rot]).toEqual([200, 12, 90])
    near(center(r).x, center(box).x)
    near(center(r).y, center(box).y)
  })
  it('корешок 90 → 180 и 90 → 0: обратно', () => {
    const v = turnBackdropBox(box, 90)
    for (const deg of [180, 0]) {
      const r = turnBackdropBox(v, deg)
      expect([r.w, r.h, r.rot]).toEqual([12, 200, deg])
      near(center(r).x, center(box).x)
      near(center(r).y, center(box).y)
    }
  })
  it('корешок 0 → 30, 90 → -90, 0 → 180: без смены w и h', () => {
    expect(turnBackdropBox(box, 30)).toMatchObject({ w: 12, h: 200, rot: 30 })
    const v = turnBackdropBox(box, 90)
    expect(turnBackdropBox(v, -90)).toMatchObject({ w: 200, h: 12, rot: -90 })
    expect(turnBackdropBox(box, 180)).toMatchObject({ w: 12, h: 200, rot: 180 })
  })
  it('любая область (070 Г): 0 → 90 меняет стороны вокруг центра', () => {
    const r = turnBackdropBox({ x: 100, y: 20, w: 200, h: 80, rot: 0 }, 90)
    expect([r.w, r.h, r.rot]).toEqual([80, 200, 90])
    near(center(r).x, 200)
    near(center(r).y, 60)
  })
  it('80 → 100 без смены ориентации — стороны те же', () => {
    expect(turnBackdropBox({ x: 10, y: 20, w: 200, h: 80, rot: 80 }, 100)).toMatchObject({ w: 200, h: 80, rot: 100 })
  })
})

describe('turnBackdropBox: пути поворота (решение 33)', () => {
  const box = { x: 100, y: 20, w: 12, h: 200, rot: 0 }
  const side = (r: { w: number; h: number }) => [r.w, r.h]

  it('двойной щелчок ⟳ (→ 0°) после 90°: стороны вернулись вдоль корешка', () => {
    const v = turnBackdropBox(box, 90)
    expect(side(v)).toEqual([200, 12])
    expect(side(turnBackdropBox(v, 0))).toEqual([12, 200])
  })
  it('двойной щелчок ⟳ на уже прямой рамке: ничего не меняется', () => {
    expect(turnBackdropBox(box, 0)).toEqual(box)
  })
  it('протяжка ⟳ считается от рамки начала жеста: до 90° стороны поменялись, назад до 10° — нет', () => {
    expect(side(turnBackdropBox(box, 90))).toEqual([200, 12])
    expect(side(turnBackdropBox(box, 10))).toEqual([12, 200])
  })
  it('граница 45°: ещё вдоль корешка, после неё — поперёк', () => {
    expect(side(turnBackdropBox(box, 44.9))).toEqual([12, 200])
    expect(side(turnBackdropBox(box, 45))).toEqual([12, 200])
    expect(side(turnBackdropBox(box, 45.1))).toEqual([200, 12])
  })
  it('граница 135°: до неё поперёк, на ней и дальше — вдоль', () => {
    expect(side(turnBackdropBox(box, 134.9))).toEqual([200, 12])
    expect(side(turnBackdropBox(box, 135))).toEqual([12, 200])
    expect(side(turnBackdropBox(box, 135.1))).toEqual([12, 200])
  })
  it('из 90° к 135° и 45°: рамка снова вдоль корешка', () => {
    const v = turnBackdropBox(box, 90)
    expect(side(turnBackdropBox(v, 135))).toEqual([12, 200])
    expect(side(turnBackdropBox(v, 45))).toEqual([12, 200])
  })
})

// Паритет с Go (geom.TurnBox): vectors пишет TestTurnBoxParity -update.
describe('общие векторы с Go (bdturn.json)', () => {
  const cases = vectors as { box: TextBox; rot: number; want: TextBox }[]
  it('набор не пуст', () => expect(cases.length).toBeGreaterThan(0))
  it.each(cases.map((c, i) => [i, c] as const))('вектор %i', (_i, c) => {
    const r = turnBackdropBox(c.box, c.rot)
    for (const k of ['x', 'y', 'w', 'h'] as const) expect(r[k]).toBeCloseTo(c.want[k], 6)
    expect(r.rot).toBe(c.want.rot)
  })
})
