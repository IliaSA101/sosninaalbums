import { describe, expect, it } from 'vitest'
import { backdropLost, backdropOf, zoneScopes } from './backdrop'
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
