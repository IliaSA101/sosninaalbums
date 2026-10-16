import { describe, expect, it } from 'vitest'
import { backdropCss, backdropLost, backdropOf, zoneScopes } from './backdrop'
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
