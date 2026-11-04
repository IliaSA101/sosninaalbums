import { describe, expect, it } from 'vitest'
import vectors from '../../../../testdata/parity/tone.json'
import { MAX_TONE, toneCss } from './tone'

// Строка CSS у Go (model.Tone.CSS) и у живого показа одна: общие векторы testdata/parity/tone.json.
describe('toneCss: общие векторы с Go (tone.json)', () => {
  it('векторы не пусты', () => expect(vectors.length).toBeGreaterThan(5))
  for (const v of vectors) {
    it(JSON.stringify(v.tone), () => {
      const t = v.tone as { gray?: boolean; b?: number; c?: number }
      expect(toneCss({ gray: t.gray, bright: t.b, contrast: t.c })).toBe(v.css)
    })
  }
})

describe('toneCss: крайнее', () => {
  it('нет тона — пустая строка', () => {
    expect(toneCss(null)).toBe('')
    expect(toneCss(undefined)).toBe('')
    expect(toneCss({})).toBe('')
    expect(toneCss({ gray: false, bright: 0, contrast: 0 })).toBe('')
  })
  it('зажим, как в Go: мусор не уходит в фильтр', () => {
    expect(toneCss({ bright: 900, contrast: -1e9 })).toBe(`brightness(${100 + MAX_TONE}%) contrast(${100 - MAX_TONE}%)`)
    expect(toneCss({ bright: NaN })).toBe('')
  })
})
