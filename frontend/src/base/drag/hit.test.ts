import { describe, expect, it } from 'vitest'
import type { Payload } from '../core/types'
import { backdropTarget, cellSide, coverTarget, halfSide, isNoopMove, placeTarget, sheetFrac } from './hit'

describe('backdropTarget', () => {
  const one: Payload = { kind: 'photos', ids: ['a'] }
  it('одно фото и известная область — цель', () => {
    expect(backdropTarget({ spread: 's1', scope: 'right' }, one)).toEqual({ kind: 'backdrop', spread: 's1', scope: 'right' })
  })
  it('несколько фото — цель с many (без подсветки, бросок объяснит тостом)', () => {
    expect(backdropTarget({ spread: 's1', scope: 'left' }, { kind: 'photos', ids: ['a', 'b'] })).toEqual({
      kind: 'backdrop',
      spread: 's1',
      scope: 'left',
      many: true,
    })
  })
  it('пусто, группа, чужая область, нет разворота — не цель', () => {
    expect(backdropTarget({ spread: 's1', scope: 'left' }, { kind: 'photos', ids: [] })).toBeNull()
    expect(backdropTarget({ spread: 's1', scope: 'left' }, { kind: 'spread', id: 's1', index: 0 })).toBeNull()
    expect(backdropTarget({ spread: 's1', scope: 'top' }, one)).toBeNull()
    expect(backdropTarget({ scope: 'left' }, one)).toBeNull()
  })
})

describe('cellSide', () => {
  it('края и центр', () => {
    expect([0.1, 0.5, 0.9].map((x) => cellSide(x, true))).toEqual(['before', 'center', 'after'])
  })
  it('из ленты центр = после', () => {
    expect(cellSide(0.5, false)).toBe('after')
    expect(cellSide(0.1, false)).toBe('before')
  })
})

describe('halfSide', () => {
  it('пополам', () => {
    expect(halfSide(0.49)).toBe('before')
    expect(halfSide(0.5)).toBe('after')
  })
})

describe('isNoopMove', () => {
  it('на себя и на соседнюю позицию', () => {
    expect(isNoopMove(2, 2, 'before')).toBe(true)
    expect(isNoopMove(2, 2, 'after')).toBe(true)
    expect(isNoopMove(2, 1, 'after')).toBe(true)
  })
  it('в другое место — не no-op, из ленты (from < 0) — тоже', () => {
    expect(isNoopMove(2, 0, 'before')).toBe(false)
    expect(isNoopMove(2, 3, 'after')).toBe(false)
    expect(isNoopMove(-1, 0, 'before')).toBe(false)
  })
})

describe('placeTarget', () => {
  const d = { spread: 's1', place: 'p1', photo: 'a' }
  it('фото на занятое и пустое место', () => {
    expect(placeTarget(d, { kind: 'photos', ids: ['b'] })).toEqual({ kind: 'place', spread: 's1', place: 'p1', photo: 'a' })
    expect(placeTarget({ spread: 's1', place: 'p2' }, { kind: 'photos', ids: ['b'] })).toEqual({
      kind: 'place',
      spread: 's1',
      place: 'p2',
      photo: null,
    })
    expect(placeTarget({ spread: 's1', place: 'p2', photo: '' }, { kind: 'photos', ids: ['b'] })).toMatchObject({ photo: null })
  })
  it('своё же фото на своё место — не цель, группа — цель', () => {
    expect(placeTarget(d, { kind: 'photos', ids: ['a'], fromSpread: 's1' })).toBeNull()
    expect(placeTarget(d, { kind: 'photos', ids: ['a', 'b'], fromSpread: 's1' })).not.toBeNull()
  })
  it('разворот-группа и неполная разметка — не цель', () => {
    expect(placeTarget(d, { kind: 'spread', id: 's2', index: 1 })).toBeNull()
    expect(placeTarget({ spread: 's1' }, { kind: 'photos', ids: ['b'] })).toBeNull()
    expect(placeTarget({ place: 'p1' }, { kind: 'photos', ids: ['b'] })).toBeNull()
  })
})

describe('coverTarget', () => {
  it('одно фото — цель «плитка обложки»', () => {
    expect(coverTarget({ kind: 'photos', ids: ['a'] })).toEqual({ kind: 'cover' })
  })
  it('несколько фото — цель с many (без подсветки, бросок объяснит тостом)', () => {
    expect(coverTarget({ kind: 'photos', ids: ['a', 'b'] })).toEqual({ kind: 'cover', many: true })
  })
  it('разворот-группа и пустой набор — не цель', () => {
    expect(coverTarget({ kind: 'spread', id: 's1', index: 0 })).toBeNull()
    expect(coverTarget({ kind: 'photos', ids: [] })).toBeNull()
  })
})

describe('sheetFrac', () => {
  it('доля зажимается в 0..1', () => {
    expect([-0.2, 0, 0.4, 1, 1.3].map(sheetFrac)).toEqual([0, 0, 0.4, 1, 1])
  })
  it('нулевая ширина листа (NaN, ±Infinity) — точки нет, -1', () => {
    expect([NaN, Infinity, -Infinity].map(sheetFrac)).toEqual([-1, -1, -1])
  })
})
