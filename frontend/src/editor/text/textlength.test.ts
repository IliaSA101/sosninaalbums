import { describe, expect, it } from 'vitest'
import { lineDx, lineLen } from './fonts'

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

// Кернинг своих шрифтов (161): dx из Go — мм по кодовым точкам; UI только склеивает в атрибут.
describe('lineDx: атрибут dx строки SVG', () => {
  it('значения через пробел', () => {
    expect(lineDx({ text: 'То', dx: [0, -0.49] })).toBe('0 -0.49')
  })
  it('нет dx (встроенный шрифт) или пустой — без атрибута, строка как в 0.35', () => {
    expect(lineDx({ text: 'То' })).toBeUndefined()
    expect(lineDx({ text: 'То', dx: [] })).toBeUndefined()
    expect(lineDx({ text: '', dx: [] })).toBeUndefined()
  })
  it('on = false (запасной флаг KERN_DX) — undefined даже при dx', () => {
    expect(lineDx({ text: 'То', dx: [0, -0.49] }, false)).toBeUndefined()
  })
  it('длина dx считается по кодовым точкам: суррогатная пара — один знак', () => {
    expect(lineDx({ text: 'А😀Т', dx: [0, 0, -0.3] })).toBe('0 0 -0.3') // 3 знака, хотя UTF-16 даёт 4
    expect(lineDx({ text: 'А😀Т', dx: [0, 0, 0, -0.3] })).toBeUndefined() // посчитано по UTF-16 — не наш контракт
  })
  it('комбинирующий знак занимает место в списке', () => {
    expect(lineDx({ text: 'и\u0306', dx: [0, 0] })).toBe('0 0')
    expect(lineDx({ text: 'и\u0306', dx: [0] })).toBeUndefined()
  })
  it('длина не сходится с текстом или не число — без dx (сдвиг не на тот знак хуже, чем без сдвига)', () => {
    expect(lineDx({ text: 'Тон', dx: [0, -0.5] })).toBeUndefined()
    expect(lineDx({ text: 'То', dx: [0, NaN] })).toBeUndefined()
    expect(lineDx({ text: 'То', dx: [0, Infinity] })).toBeUndefined()
  })
})

describe('TextLayer: все три места строк', () => {
  // Компонент без DOM не смонтировать; страж на разметку: подписи, заглушки и тексты ставят textLength одинаково.
  const src = Object.values(
    import.meta.glob('./TextLayer.svelte', { query: '?raw', import: 'default', eager: true }) as Record<string, string>,
  )[0]
  it('каждый <text> строки несёт textLength из Go и lengthAdjust="spacing"', () => {
    const texts = src.match(/<text\s[^>]*>/g) ?? []
    expect(texts).toHaveLength(3)
    for (const t of texts) {
      expect(t).toContain('textLength={lineLen(l.w)}')
      expect(t).toContain('lengthAdjust="spacing"')
    }
  })
  it('каждый <text> ставит dx из lineDx и выключает кернинг браузера, когда dx есть', () => {
    // dx берётся один раз на строку: {@const dx = lineDx(l)}, в атрибут идёт {dx}, а в стиль — признак noKern.
    expect(src.match(/\{@const dx = lineDx\(l\)\}/g) ?? []).toHaveLength(3)
    const texts = src.match(/<text\s[^>]*>/g) ?? []
    for (const t of texts) {
      expect(t).toMatch(/\{dx\}/)
      expect(t).toMatch(/style=\{(liveCss|svgTextCss)\([\s\S]*dx !== undefined\)\}/)
    }
  })
})
