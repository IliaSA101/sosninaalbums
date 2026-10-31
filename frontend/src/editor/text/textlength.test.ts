import { describe, expect, it } from 'vitest'
import { lineLen } from './fonts'

// Длина строки из Go (спека 082, design §7.1): экран подгоняется под ширину Go и JPEG.
describe('lineLen: textLength строки SVG', () => {
  it('ширина из Go передаётся как есть', () => {
    expect(lineLen(42.5)).toBe(42.5)
  })
  it('нет ширины, ноль, минус, NaN — без textLength (как раньше)', () => {
    expect(lineLen(undefined)).toBeUndefined()
    expect(lineLen(0)).toBeUndefined()
    expect(lineLen(-3)).toBeUndefined()
    expect(lineLen(NaN)).toBeUndefined()
    expect(lineLen(Infinity)).toBeUndefined()
  })
})

describe('TextLayer: все три места строк', () => {
  // Компонент без DOM не смонтировать; страж на разметку: подписи, заглушки и тексты ставят textLength одинаково.
  const src = Object.values(
    import.meta.glob('./TextLayer.svelte', { query: '?raw', import: 'default', eager: true }) as Record<string, string>,
  )[0]
  it('каждый <text> строки несёт textLength из Go и lengthAdjust="spacing"', () => {
    const texts = src.match(/<text [^>]*>/g) ?? []
    expect(texts).toHaveLength(3)
    for (const t of texts) {
      expect(t).toContain('textLength={lineLen(l.w)}')
      expect(t).toContain('lengthAdjust="spacing"')
    }
  })
})
