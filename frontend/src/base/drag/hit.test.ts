import { describe, expect, it } from 'vitest'
import type { Payload } from '../core/types'
import {
  backdropTarget,
  binTarget,
  cellSide,
  coverTarget,
  gapIndex,
  gapNoop,
  gapOf,
  halfSide,
  isNoopMove,
  photoNoop,
  placeTarget,
  sheetFrac,
} from './hit'

describe('backdropTarget', () => {
  const one: Payload = { kind: 'photos', ids: ['a'] }
  it('одно фото и известная область — цель', () => {
    expect(backdropTarget({ spread: 's1', scope: 'right' }, one)).toEqual({ kind: 'backdrop', spread: 's1', scope: 'right' })
  })
  it('корешок обложки (061) — тоже область фона', () => {
    expect(backdropTarget({ spread: 'cover', scope: 'spine' }, one)).toEqual({ kind: 'backdrop', spread: 'cover', scope: 'spine' })
    expect(backdropTarget({ spread: 'cover', scope: 'hinge' }, one)).toBeNull()
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

describe('binTarget (083)', () => {
  it('фото из ленты — цель; с холста, пустой набор и разворот — нет', () => {
    expect(binTarget({ kind: 'photos', ids: ['a', 'b'] })).toEqual({ kind: 'bin' })
    expect(binTarget({ kind: 'photos', ids: ['a'], fromSpread: 's1' })).toBeNull()
    expect(binTarget({ kind: 'photos', ids: [] })).toBeNull()
    expect(binTarget({ kind: 'spread', id: 's1', index: 0 })).toBeNull()
  })
})

// 095: одна черта в щели, холостые места не светятся.
describe('щели (095)', () => {
  it('gapOf: после j = до j+1', () => {
    expect(gapOf(3, 'after')).toBe(gapOf(4, 'before'))
    expect(gapOf(0, 'before')).toBe(0)
    expect(gapOf(0, 'after')).toBe(1)
  })
  it('gapIndex: своё место — холостое', () => {
    // разворот 2 в щели 2 и 3 — остаётся вторым
    expect(gapIndex(2, 2)).toBe(2)
    expect(gapIndex(3, 2)).toBe(2)
    expect(gapNoop(2, 2)).toBe(true)
    expect(gapNoop(2, 3)).toBe(true)
    // дальше вправо — индекс на один меньше (сам уходит из-под), влево — как есть
    expect(gapIndex(5, 2)).toBe(4)
    expect(gapIndex(0, 2)).toBe(0)
    expect(gapNoop(2, 4)).toBe(false)
    expect(gapNoop(-1, 0)).toBe(false)
  })
  it('isNoopMove — то же через щель', () => {
    expect(isNoopMove(2, 1, 'after')).toBe(gapNoop(2, 2))
  })
  it('photoNoop: chip, cell, mini, свой лист', () => {
    const one = { kind: 'photos' as const, ids: ['a'], fromSpread: 's1' }
    const two = { kind: 'photos' as const, ids: ['a', 'b'], fromSpread: 's1' }
    const strip = { kind: 'photos' as const, ids: ['a'] }
    // фото 2 своего разворота: щели 2 и 3 — у себя, 1 и 4 — нет
    for (const k of ['chip', 'cell'] as const) {
      expect(photoNoop(k, one, 's1', 2, 2), k).toBe(true)
      expect(photoNoop(k, one, 's1', 2, 3), k).toBe(true)
      expect(photoNoop(k, one, 's1', 2, 1), k).toBe(false)
      expect(photoNoop(k, one, 's1', 2, 4), k).toBe(false)
      // чужой разворот, из ленты, несколько фото — не холостой
      expect(photoNoop(k, one, 's2', 2, 2), k).toBe(false)
      expect(photoNoop(k, strip, 's1', -1, 0), k).toBe(false)
      expect(photoNoop(k, two, 's1', -1, 2), k).toBe(false)
    }
    // своя группа ленты и свой лист — холостой всегда, чужие — нет
    expect(photoNoop('mini', one, 's1')).toBe(true)
    expect(photoNoop('spread', two, 's1')).toBe(true)
    expect(photoNoop('mini', one, 's2')).toBe(false)
    expect(photoNoop('spread', strip, 's1')).toBe(false)
  })
})
