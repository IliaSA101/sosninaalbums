import { describe, expect, it } from 'vitest'
import { plural } from './plural'

const f: [string, string, string] = ['раскладка', 'раскладки', 'раскладок']

describe('plural', () => {
  it('склоняет по числу', () => {
    expect([1, 2, 5, 0].map((n) => plural(n, f))).toEqual([f[0], f[1], f[2], f[2]])
  })
  it('11-14 — всегда третья форма', () => {
    for (const n of [11, 12, 13, 14, 111, 112]) expect(plural(n, f)).toBe(f[2])
  })
  it('21, 22, 25, 101', () => {
    expect([21, 22, 25, 101].map((n) => plural(n, f))).toEqual([f[0], f[1], f[2], f[0]])
  })
})
