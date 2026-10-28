import { describe, expect, it } from 'vitest'
import { ownLabel } from './ownlabel'

describe('ownLabel', () => {
  it.each([
    [16 / 9, '16 : 9'],
    [1.7778, '16 : 9'],
    [1.5, '3 : 2'],
    [0.75, '3 : 4'],
    [1, '1 : 1'],
    [2, '2 : 1'],
    [1.3, '13 : 10'],
  ])('%f → %s', (v, want) => expect(ownLabel(v)).toBe(want))

  it('не рациональное с малым знаменателем — десятичная', () => {
    expect(ownLabel(1.2345)).toBe('1,23 : 1')
    expect(ownLabel(0.137)).toBe('0,14 : 1')
  })

  it('нет кадра или мусор — пусто', () => {
    for (const v of [0, -1, NaN, Infinity]) expect(ownLabel(v)).toBe('')
  })
})
