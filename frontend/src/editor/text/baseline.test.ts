// @vitest-environment happy-dom
import { describe, expect, it } from 'vitest'
import { editorShift, measureBase } from './baseline'

// Расчёт поправки редактора (156-3). Метрики — доли кегля из design §2.1, кегль 80 pt, k = 1,52 px/мм.
const K = 1.52
const SIZE_MM = (80 * 25.4) / 72
const SIZE_PX = SIZE_MM * K
const LH = 1.2 * SIZE_PX

interface Face {
  asc: number
  desc: number
}
const BADSCRIPT: Face = { asc: 1.324, desc: 0.648 }
const OLDSTANDARD: Face = { asc: 0.762, desc: 0.238 }

// Блок Go: asc, шаг 1,2 кегля, H = asc + desc + (n−1)·шаг; y блока = 10 мм.
function block(f: Face, n: number) {
  const y = 10
  const lines = Array.from({ length: n }, (_, i) => ({ y: y + f.asc * SIZE_MM + i * 1.2 * SIZE_MM }))
  return { lines, box: { y, h: (f.asc + f.desc) * SIZE_MM + (n - 1) * 1.2 * SIZE_MM } }
}

// База первой строки в браузере: (lh − (A + D))/2 + A, округлённая до целого px.
const browserBase = (f: Face) => Math.round((LH - (f.asc + f.desc) * SIZE_PX) / 2 + f.asc * SIZE_PX)

describe('editorshift', () => {
  it('editorshift: badscript — сдвиг вниз 11 мм', () => {
    const { lines, box } = block(BADSCRIPT, 3)
    const s = editorShift(lines, box, browserBase(BADSCRIPT), SIZE_PX, K)
    expect(Math.abs(s.padTop - 11.1 * K)).toBeLessThan(0.5)
    expect(s.shiftY).toBe(0)
  })

  it('editorshift: oldstandard — вверх через shifty', () => {
    const { lines, box } = block(OLDSTANDARD, 3)
    const s = editorShift(lines, box, browserBase(OLDSTANDARD), SIZE_PX, K)
    expect(s.padTop).toBe(0)
    expect(s.padBottom).toBe(0)
    expect(s.shiftY).toBeLessThan(0)
    expect(Math.abs(s.shiftY + 2.8 * K)).toBeLessThan(0.5)
  })

  it('editorshift: рамка редактора = рамка go при top, bottom ≥ 0', () => {
    for (const n of [1, 3]) {
      const { lines, box } = block(BADSCRIPT, n)
      const s = editorShift(lines, box, browserBase(BADSCRIPT), SIZE_PX, K)
      expect(s.padTop + n * LH + s.padBottom).toBeCloseTo(box.h * K, 6)
      expect(s.minH).toBeCloseTo(LH + s.padTop + s.padBottom, 9)
    }
  })

  it('editorshift: низкий шрифт — обводка шире рамки go', () => {
    for (const n of [1, 3]) {
      const { lines, box } = block(OLDSTANDARD, n)
      const base = browserBase(OLDSTANDARD)
      const s = editorShift(lines, box, base, SIZE_PX, K)
      const top = (lines[0].y - box.y) * K - base
      const bottom = (box.y + box.h - lines[n - 1].y) * K - (LH - base)
      expect(top).toBeLessThan(0)
      expect(bottom).toBeLessThan(0)
      // Верх обводки = y·k + top, низ = (y+h)·k − bottom: шире рамки Go на |top| сверху и |bottom| снизу.
      expect(box.y * K + s.shiftY).toBeCloseTo(box.y * K + top, 6)
      expect(box.y * K + s.shiftY + n * LH + s.padBottom).toBeCloseTo((box.y + box.h) * K - bottom, 6)
    }
  })

  it('editorshift: minh — рамка go одной строки', () => {
    const { lines, box } = block(BADSCRIPT, 1)
    const s = editorShift(lines, box, browserBase(BADSCRIPT), SIZE_PX, K)
    expect(s.minH).toBeCloseTo(box.h * K, 6)
  })

  it('editorshift: пусто и nan — нули', () => {
    const zero = { padTop: 0, padBottom: 0, shiftY: 0, minH: 0 }
    const { lines, box } = block(BADSCRIPT, 1)
    expect(editorShift([], box, 40, SIZE_PX, K)).toEqual(zero)
    expect(editorShift(lines, box, null, SIZE_PX, K)).toEqual(zero)
    expect(editorShift(lines, box, NaN, SIZE_PX, K)).toEqual(zero)
    expect(editorShift([{ y: NaN }], box, 40, SIZE_PX, K)).toEqual(zero)
    expect(editorShift(lines, { y: 10, h: NaN }, 40, SIZE_PX, K)).toEqual(zero)
    expect(editorShift(lines, box, 40, 0, K)).toEqual(zero)
    expect(editorShift(lines, box, 40, SIZE_PX, 0)).toEqual(zero)
  })

  it('editorshift: предел ±2 кегля', () => {
    const { lines, box } = block(BADSCRIPT, 1)
    const big = editorShift([{ y: lines[0].y + 1e6 }], { y: box.y, h: box.h + 2e6 }, 40, SIZE_PX, K)
    expect(big.padTop).toBe(2 * SIZE_PX)
    expect(big.padBottom).toBe(2 * SIZE_PX)
    const neg = editorShift(lines, box, 1e6, SIZE_PX, K)
    expect(neg.shiftY).toBe(-2 * SIZE_PX)
    expect(neg.padTop).toBe(0)
  })
})

describe('measurebase', () => {
  it('measurebase: нет span — null', () => {
    expect(measureBase(document.createElement('div'))).toBeNull()
  })
})
