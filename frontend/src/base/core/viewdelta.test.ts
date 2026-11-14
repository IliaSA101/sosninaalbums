import { describe, expect, it } from 'vitest'
import vectors from '../../../../testdata/parity/viewdelta.json'
import { admit, mergeView } from './viewdelta'
import type { SpreadView, View } from './types'

type Case = { name: string; prev: unknown; delta: unknown; want: unknown }

describe('mergeView: паритет с Go (testdata/parity/viewdelta.json)', () => {
  for (const c of vectors as Case[]) {
    it(c.name, () => {
      const got = mergeView(c.prev as View, c.delta as View)
      if (c.want === null) expect(got).toBeNull()
      else expect(got).toEqual(c.want)
    })
  }
})

const sp = (id: string, name = ''): SpreadView => ({ id, name }) as unknown as SpreadView
const full = (ids: string[], seq: number): View => ({ seq, rev: 1, spreads: ids.map((i) => sp(i)) }) as unknown as View
const delta = (order: string[], fresh: SpreadView[], base: number, seq = base + 1): View =>
  ({ seq, base, order, rev: 2, spreads: fresh }) as unknown as View

describe('mergeView', () => {
  it('неизменённые развороты — те же объекты', () => {
    const prev = full(['a', 'b', 'c'], 5)
    const nb = sp('b', 'новое')
    const got = mergeView(prev, delta(['a', 'b', 'c'], [nb], 5))!
    expect(got.spreads[0]).toBe(prev.spreads[0])
    expect(got.spreads[1]).toBe(nb)
    expect(got.spreads[2]).toBe(prev.spreads[2])
  })

  it('в результате нет base и order, seq — дельты', () => {
    const got = mergeView(full(['a'], 5), delta(['a'], [], 5))!
    expect('base' in got || 'order' in got).toBe(false)
    expect(got.seq).toBe(6)
  })

  it('дельта без order — null', () => {
    expect(mergeView(full(['a'], 5), { seq: 6, base: 5, spreads: [] } as unknown as View)).toBeNull()
  })
})

describe('admit: цепочка видов', () => {
  const cur = full(['a', 'b'], 5)
  it('полный без seq (вне цепочки) применяется', () => {
    expect(admit(cur, 5, full(['a'], 0)).kind).toBe('set')
  })
  it('полный с seq больше — применяется, не больше — пропуск', () => {
    expect(admit(cur, 5, full(['a'], 6)).kind).toBe('set')
    expect(admit(cur, 5, full(['a'], 5)).kind).toBe('skip')
    expect(admit(cur, 5, full(['a'], 3)).kind).toBe('skip')
  })
  it('дельта к нашему виду — set', () => {
    expect(admit(cur, 5, delta(['a', 'b'], [], 5)).kind).toBe('set')
  })
  it('дубль и пустая дельта (seq == base == наш) — пропуск', () => {
    expect(admit(cur, 6, delta(['a', 'b'], [], 5)).kind).toBe('skip')
    expect(admit(cur, 5, delta(['a', 'b'], [], 5, 5)).kind).toBe('skip')
  })
  it('дельта к чужому виду — resync с её seq', () => {
    expect(admit(cur, 5, delta(['a'], [], 8))).toEqual({ kind: 'resync', seq: 9 })
  })
  it('разворота нет нигде — resync', () => {
    expect(admit(cur, 5, delta(['a', 'zz'], [], 5))).toEqual({ kind: 'resync', seq: 6 })
  })
})
