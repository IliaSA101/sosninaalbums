import { describe, expect, it } from 'vitest'
import { freePhotos } from './freephotos'
import type { Photo } from '../../base/core/types'

const ph = (id: string): Photo => ({ id, path: id, name: id, w: 100, h: 100 })
const photos = ['a', 'b', 'c', 'd'].map(ph)
const used = new Set(['a'])
const bg = new Set(['b'])

describe('freePhotos', () => {
  it('не в местах и не на фоне', () => {
    expect(freePhotos(photos, used, bg).map((p) => p.id)).toEqual(['c', 'd'])
  })
  it('без фонов — только вычет used', () => {
    expect(freePhotos(photos, used, new Set()).map((p) => p.id)).toEqual(['b', 'c', 'd'])
  })
})
