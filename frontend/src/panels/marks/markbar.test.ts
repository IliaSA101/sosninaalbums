import { describe, expect, it } from 'vitest'
import type { Photo, XmpMarks } from '../../base/core/types'
import { filterLabel, flagClick, hasOwn, ratingClick, sharedMark, starsLit } from './markbar'

const ph = (id: string, xmp?: XmpMarks): Photo => ({ id, path: id, name: id, w: 1, h: 1, xmp })

describe('sharedMark', () => {
  it('никого — всё не определено', () => {
    expect(sharedMark([], {})).toEqual({ rating: null, label: null, flag: null })
  })
  it('одинаковое у всех — значение, разное — null', () => {
    const a = ph('a', { rating: 3, label: 'red' })
    const b = ph('b', { rating: 3, label: 'green', flag: 1 })
    expect(sharedMark([a, b], {})).toEqual({ rating: 3, label: null, flag: null })
  })
  it('своя отметка главнее файла', () => {
    const a = ph('a', { rating: 5 })
    const b = ph('b', { rating: 1 })
    expect(sharedMark([a, b], { a: { rating: 2 }, b: { rating: 2 } }).rating).toBe(2)
  })
})

describe('starsLit', () => {
  it('наведение главнее общей оценки', () => {
    expect(starsLit(3, 5)).toBe(3)
    expect(starsLit(1, null)).toBe(1)
  })
  it('без наведения — общая оценка; разные оценки и ноль — ни одной', () => {
    expect(starsLit(null, 4)).toBe(4)
    expect(starsLit(null, null)).toBe(0)
    expect(starsLit(null, 0)).toBe(0)
  })
})

describe('filterLabel', () => {
  it('без фильтра отметок — область и число', () => {
    expect(filterLabel('all', 666, 666, false)).toBe('Все 666')
    expect(filterLabel('unused', 120, 120, false)).toBe('Не в альбоме 120')
  })
  it('с фильтром отметок — «показано из»', () => {
    expect(filterLabel('all', 666, 12, true)).toBe('Показано 12 из 666')
    expect(filterLabel('unused', 120, 5, true)).toBe('Не в альбоме: 5 из 120')
  })
})

describe('hasOwn', () => {
  const own = { a: { rating: 2 } }
  it('есть у одного из выделенных', () => {
    expect(hasOwn(['b', 'a'], own)).toBe(true)
  })
  it('ни у кого, пусто и служебные ключи объекта', () => {
    expect(hasOwn(['b'], own)).toBe(false)
    expect(hasOwn([], own)).toBe(false)
    expect(hasOwn(['toString', 'constructor'], own)).toBe(false)
  })
})

describe('клики', () => {
  it('звезда: по текущей снимает', () => {
    expect(ratingClick(3, 3)).toBe(0)
    expect(ratingClick(3, 4)).toBe(3)
    expect(ratingClick(3, null)).toBe(3)
  })
  it('флажок: повтор снимает', () => {
    expect(flagClick(1, 1)).toBe(0)
    expect(flagClick(-1, 1)).toBe(-1)
    expect(flagClick(1, null)).toBe(1)
  })
})
