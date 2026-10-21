import { describe, expect, it } from 'vitest'
import { backdropAccepts, dropPlan } from './drop'
import type { Payload, Target } from '../core/types'

const strip: Payload = { kind: 'photos', ids: ['a', 'b'] }
const inSpread: Payload = { kind: 'photos', ids: ['a'], fromSpread: 's1' }
const group: Payload = { kind: 'spread', id: 's1', index: 1 }

describe('dropPlan: разворот', () => {
  it('на mini — перенос с поправкой индекса', () => {
    const t: Target = { kind: 'mini', spread: 's3', index: 3, side: 'after' }
    expect(dropPlan(group, t)).toEqual({ op: 'moveSpread', id: 's1', at: 3 })
  })
  it('на своё место — ничего', () => {
    expect(dropPlan(group, { kind: 'mini', spread: 's1', index: 1, side: 'before' })).toBeNull()
    expect(dropPlan(group, { kind: 'mini', spread: 's0', index: 0, side: 'after' })).toBeNull()
  })
  it('на другие цели — ничего', () => {
    expect(dropPlan(group, { kind: 'library' })).toBeNull()
    expect(dropPlan(group, { kind: 'new', at: 0 })).toBeNull()
  })
})

describe('dropPlan: фото', () => {
  it('ячейка: после — вставка за ней', () => {
    const t: Target = { kind: 'cell', spread: 's2', index: 1, photo: 'x', side: 'after' }
    expect(dropPlan(strip, t)).toEqual({ op: 'add', spread: 's2', ids: ['a', 'b'], at: 2, fromSpread: false, go: false })
  })
  it('ячейка: центр — обмен, но не с собой и не группой', () => {
    const c = (photo: string): Target => ({ kind: 'cell', spread: 's2', index: 0, photo, side: 'center' })
    expect(dropPlan(inSpread, c('x'))).toEqual({ op: 'swap', a: 'a', b: 'x' })
    expect(dropPlan(inSpread, c('a'))).toBeNull()
    expect(dropPlan({ kind: 'photos', ids: ['a', 'b'], fromSpread: 's1' }, c('x'))).toBeNull()
  })
  it('chip: до — по индексу', () => {
    const t: Target = { kind: 'chip', spread: 's2', index: 2, photo: 'x', side: 'before' }
    expect(dropPlan(inSpread, t)).toMatchObject({ op: 'add', at: 2, fromSpread: true })
  })
  it('spread/mini: в конец; mini переходит на разворот; на свой — ничего', () => {
    expect(dropPlan(strip, { kind: 'spread', spread: 's2' })).toMatchObject({ op: 'add', at: -1, go: false })
    expect(dropPlan(strip, { kind: 'mini', spread: 's2', index: 0, side: 'before' })).toMatchObject({ at: -1, go: true })
    expect(dropPlan(inSpread, { kind: 'spread', spread: 's1' })).toBeNull()
  })
  it('фото на соседа (spread) — в конец, без перехода', () => {
    // сосед в ряду разворотов (спека 012) — цель spread, не mini: перехода нет
    expect(dropPlan(strip, { kind: 'spread', spread: 's3' })).toEqual({
      op: 'add',
      spread: 's3',
      ids: ['a', 'b'],
      at: -1,
      fromSpread: false,
      go: false,
    })
    expect(dropPlan(inSpread, { kind: 'spread', spread: 's3' })).toMatchObject({ at: -1, fromSpread: true, go: false })
  })
  it('spread с точкой (042): доля ширины идёт в план; у mini и соседа её нет', () => {
    expect(dropPlan(strip, { kind: 'spread', spread: 's2', at: 0.75 })).toMatchObject({ op: 'add', at: -1, frac: 0.75 })
    expect(dropPlan(strip, { kind: 'spread', spread: 's2', at: 0 })).toMatchObject({ frac: 0 })
    expect(dropPlan(strip, { kind: 'spread', spread: 's2' })).not.toHaveProperty('frac')
    expect(dropPlan(strip, { kind: 'mini', spread: 's2', index: 0, side: 'after' })).not.toHaveProperty('frac')
  })
  it('new: из ленты со снятием, из разворота без', () => {
    expect(dropPlan(strip, { kind: 'new', at: 4 })).toEqual({ op: 'newSpread', at: 4, ids: ['a', 'b'], strip: true })
    expect(dropPlan(inSpread, { kind: 'new', at: 0 })).toMatchObject({ strip: false })
  })
  it('library: убирает только фото из разворота', () => {
    expect(dropPlan(inSpread, { kind: 'library' })).toEqual({ op: 'remove', ids: ['a'] })
    expect(dropPlan(strip, { kind: 'library' })).toBeNull()
  })
})

describe('dropPlan: место сетки', () => {
  const empty: Target = { kind: 'place', spread: 's2', place: 'p1', photo: null }
  const busy: Target = { kind: 'place', spread: 's2', place: 'p2', photo: 'x' }
  it('из ленты на пустое место', () => {
    expect(dropPlan({ kind: 'photos', ids: ['a'] }, empty)).toEqual({
      op: 'place',
      spread: 's2',
      place: 'p1',
      ids: ['a'],
      fromSpread: false,
    })
  })
  it('из ленты на занятое — замена (тот же план, решает Go)', () => {
    expect(dropPlan({ kind: 'photos', ids: ['a'] }, busy)).toEqual({
      op: 'place',
      spread: 's2',
      place: 'p2',
      ids: ['a'],
      fromSpread: false,
    })
  })
  it('из альбома: обмен или перенос — тот же план с fromSpread', () => {
    expect(dropPlan(inSpread, busy)).toMatchObject({ op: 'place', ids: ['a'], fromSpread: true })
    expect(dropPlan(inSpread, empty)).toMatchObject({ op: 'place', fromSpread: true })
  })
  it('своё место — ничего', () => {
    expect(dropPlan(inSpread, { kind: 'place', spread: 's1', place: 'p1', photo: 'a' })).toBeNull()
  })
  it('несколько фото — все ids уходят в план', () => {
    expect(dropPlan(strip, empty)).toMatchObject({ op: 'place', ids: ['a', 'b'] })
    // группа из двух, одно из них стоит на месте: не «своё» — Go разберёт
    expect(dropPlan({ kind: 'photos', ids: ['x', 'a'], fromSpread: 's2' }, busy)).toMatchObject({ op: 'place' })
  })
  it('место в том же развороте — обычный план place', () => {
    const t: Target = { kind: 'place', spread: 's1', place: 'p9', photo: 'z' }
    expect(dropPlan(inSpread, t)).toEqual({ op: 'place', spread: 's1', place: 'p9', ids: ['a'], fromSpread: true })
  })
  it('пустой набор и разворот-группа — ничего', () => {
    expect(dropPlan({ kind: 'photos', ids: [] }, empty)).toBeNull()
    expect(dropPlan(group, empty)).toBeNull()
  })
})

describe('dropPlan: фото фона', () => {
  const t: Target = { kind: 'backdrop', spread: 's1', scope: 'left' }
  it('одно фото из ленты или из разворота — план backdrop', () => {
    expect(dropPlan({ kind: 'photos', ids: ['a'] }, t)).toEqual({ op: 'backdrop', spread: 's1', scope: 'left', photo: 'a' })
    expect(dropPlan(inSpread, t)).toEqual({ op: 'backdrop', spread: 's1', scope: 'left', photo: 'a' })
  })
  it('несколько фото — отказ с текстом для тоста', () => {
    expect(dropPlan(strip, t)).toEqual({ op: 'reject', text: 'На фон — только одно фото' })
  })
  it('backdropAccepts: только ровно одно фото', () => {
    expect(backdropAccepts({ kind: 'photos', ids: ['a'] })).toBe(true)
    expect(backdropAccepts(strip)).toBe(false)
    expect(backdropAccepts(group)).toBe(false)
    expect(backdropAccepts(null)).toBe(false)
  })
  it('пустой набор и группа-разворот — ничего', () => {
    expect(dropPlan({ kind: 'photos', ids: [] }, t)).toBeNull()
    expect(dropPlan(group, t)).toBeNull()
  })
})

describe('dropPlan: плитка обложки', () => {
  const t: Target = { kind: 'cover' }
  it('одно фото создаёт обложку', () => {
    expect(dropPlan({ kind: 'photos', ids: ['a'] }, t)).toEqual({ op: 'cover', photo: 'a' })
    expect(dropPlan({ kind: 'photos', ids: ['a'], fromSpread: 's1' }, t)).toEqual({ op: 'cover', photo: 'a' })
  })
  it('несколько фото — тост «только одно»', () => {
    expect(dropPlan(strip, t)).toEqual({ op: 'reject', text: 'На обложку — только одно фото' })
  })
  it('пустой набор и группа-разворот — ничего', () => {
    expect(dropPlan({ kind: 'photos', ids: [] }, t)).toBeNull()
    expect(dropPlan(group, t)).toBeNull()
  })
})

describe('dropPlan: зона фона обложки', () => {
  it('фото на сторонку — фон области, как у разворота (id листа «cover»)', () => {
    const t: Target = { kind: 'backdrop', spread: 'cover', scope: 'right' }
    expect(dropPlan({ kind: 'photos', ids: ['a'] }, t)).toEqual({ op: 'backdrop', spread: 'cover', scope: 'right', photo: 'a' })
  })
})

describe('dropPlan: кнопка «Подборки» (083)', () => {
  const t: Target = { kind: 'bin' }
  it('фото из ленты — в подборку, порядок id сохранён', () => {
    expect(dropPlan({ kind: 'photos', ids: ['b', 'a'] }, t)).toEqual({ op: 'bin', ids: ['b', 'a'] })
  })
  it('фото с холста и пустой набор — ничего (с холста бросок в ленту убирает фото, а не добавляет в подборку)', () => {
    expect(dropPlan({ kind: 'photos', ids: ['a'], fromSpread: 's1' }, t)).toBeNull()
    expect(dropPlan({ kind: 'photos', ids: [] }, t)).toBeNull()
  })
  it('разворот на кнопку — ничего', () => {
    expect(dropPlan({ kind: 'spread', id: 's1', index: 0 }, t)).toBeNull()
  })
})
