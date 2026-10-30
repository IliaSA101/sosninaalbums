import { describe, expect, it } from 'vitest'
import { sameWatch } from './watch'

describe('sameWatch', () => {
  it('новый массив с теми же значениями — то же', () => {
    expect(sameWatch(['spread', 3, undefined], ['spread', 3, undefined])).toBe(true)
  })
  it('другое значение, длина или не массив — иное', () => {
    expect(sameWatch(['spread', 3], ['spread', 4])).toBe(false)
    expect(sameWatch(['a'], ['a', 'b'])).toBe(false)
    expect(sameWatch('a', ['a'])).toBe(false)
    expect(sameWatch(undefined, [])).toBe(false)
  })
  it('скаляры сравниваются по значению', () => {
    expect(sameWatch(1, 1)).toBe(true)
    expect(sameWatch(1, 2)).toBe(false)
    expect(sameWatch(NaN, NaN)).toBe(true)
  })
})
