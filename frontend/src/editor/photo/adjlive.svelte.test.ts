import { beforeEach, describe, expect, it } from 'vitest'
import { adjlive } from './adjlive.svelte'

describe('adjlive: живой тон', () => {
  beforeEach(() => adjlive.clear())

  it('нет живого значения — строка из View; пустая строка — без атрибута', () => {
    expect(adjlive.filterOf('a', { filter: 'grayscale(1)' })).toBe('grayscale(1)')
    expect(adjlive.filterOf('a', { filter: '' })).toBeUndefined()
    expect(adjlive.filterOf('a')).toBeUndefined()
  })

  it('живое значение главнее View и видно только своему фото', () => {
    adjlive.set('a', { gray: true, bright: 30, contrast: 0 })
    expect(adjlive.filterOf('a', { filter: '' })).toBe('grayscale(1) brightness(130%)')
    expect(adjlive.filterOf('b', { filter: 'contrast(80%)' })).toBe('contrast(80%)')
  })

  it('живой ноль скрывает фильтр из View (тянут к нулю)', () => {
    adjlive.set('a', { gray: false, bright: 0, contrast: 0 })
    expect(adjlive.filterOf('a', { filter: 'brightness(120%)' })).toBeUndefined()
  })

  it('clear возвращает View', () => {
    adjlive.set('a', { bright: 10 })
    adjlive.clear()
    expect(adjlive.of('a')).toBeNull()
    expect(adjlive.filterOf('a', { filter: 'contrast(90%)' })).toBe('contrast(90%)')
  })
})
