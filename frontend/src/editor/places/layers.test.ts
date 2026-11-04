import { describe, expect, it } from 'vitest'
import { LAYER0, layerIndex, layerTargets } from './layers'

describe('layerIndex', () => {
  it('z из Go сдвигается на служебные слои', () => {
    expect(layerIndex(1)).toBe(LAYER0 + 1)
    expect(layerIndex(7)).toBe(17)
  })
  it('нет z — низ контента', () => expect(layerIndex(undefined)).toBe(LAYER0))
  it('порядок z сохраняется: текст между фото остаётся между', () => {
    expect(layerIndex(1)).toBeLessThan(layerIndex(2))
    expect(layerIndex(2)).toBeLessThan(layerIndex(3))
  })
})

describe('layerTargets', () => {
  it('места, тексты группы и одиночный текст — вместе', () => {
    expect(layerTargets(['p1', 'p2'], ['t1'], 't2')).toEqual(['p1', 'p2', 't1', 't2'])
  })
  it('повторы выбрасываются, пустое выделение — пусто', () => {
    expect(layerTargets(['p1'], ['t1'], 't1')).toEqual(['p1', 't1'])
    expect(layerTargets([], [], null)).toEqual([])
  })
})
