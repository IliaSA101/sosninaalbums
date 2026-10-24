import { describe, expect, it } from 'vitest'
import { EXTRA, GAP, PAD_X, PAD_Y, anchorAfter, anchorTop, colsOf, keepScroll, libWindow, photoAt, revealTop } from './libwindow'

const base = { n: 3200, width: 1000, height: 300, size: 100 }
// Высота содержимого, как её даёт CSS .grid: поля + ряды с зазорами.
const cssHeight = (rows: number, size: number) => 2 * PAD_Y + rows * size + Math.max(0, rows - 1) * GAP
// Высота того, что нарисовано: распорки и ряды окна, между блоками gap.
function drawn(w: ReturnType<typeof libWindow>, size: number): number {
  const blocks = (w.padTop > 0 ? 1 : 0) + (w.padBottom > 0 ? 1 : 0) + (w.r1 - w.r0)
  const rows = (w.r1 - w.r0) * size
  return 2 * PAD_Y + w.padTop + w.padBottom + rows + Math.max(0, blocks - 1) * GAP
}

describe('libWindow', () => {
  it('колонки: узкая лента — одна, обычная — по формуле', () => {
    expect(colsOf(10, 100)).toBe(1)
    expect(colsOf(0, 100)).toBe(1)
    expect(colsOf(1000, 100)).toBe(Math.floor((1000 - 32 + 16) / 116))
  })

  it('начало: r0 = 0, нижняя распорка есть, верхней нет', () => {
    const w = libWindow({ ...base, scrollTop: 0 })
    expect(w.r0).toBe(0)
    expect(w.padTop).toBe(0)
    expect(w.padBottom).toBeGreaterThan(0)
    expect(w.r1).toBeLessThan(w.rows)
  })

  it('середина: обе распорки, запас рядов с обеих сторон', () => {
    const w = libWindow({ ...base, scrollTop: 3000 })
    const first = Math.floor((3000 - PAD_Y) / w.rowH)
    expect(w.r0).toBe(first - EXTRA)
    expect(w.padTop).toBe(w.r0 * w.rowH - GAP)
    expect(w.padBottom).toBe((w.rows - w.r1) * w.rowH - GAP)
    // миниатюр не больше (видимые ряды + 4) · cols
    const visibleRows = Math.ceil(base.height / w.rowH) + 1
    expect((w.r1 - w.r0) * w.cols).toBeLessThanOrEqual((visibleRows + 2 * EXTRA) * w.cols)
  })

  it('конец: r1 = rows, нижней распорки нет', () => {
    const w0 = libWindow({ ...base, scrollTop: 0 })
    const w = libWindow({ ...base, scrollTop: cssHeight(w0.rows, 100) - base.height })
    expect(w.r1).toBe(w.rows)
    expect(w.padBottom).toBe(0)
    expect(w.padTop).toBeGreaterThan(0)
  })

  it('распорки дают прежнюю высоту сетки на любой прокрутке', () => {
    for (const n of [1, 9, 10, 11, 139, 3200]) {
      for (const st of [0, 1, 500, 3000, 40000, 99999]) {
        const w = libWindow({ ...base, n, scrollTop: st })
        expect(drawn(w, 100), `n=${n} st=${st}`).toBe(cssHeight(w.rows, 100))
      }
    }
  })

  it('устаревший scrollTop за концом не оставляет пустое окно', () => {
    const w = libWindow({ ...base, n: 20, scrollTop: 50000 })
    expect(w.r1).toBeGreaterThan(w.r0)
    expect(w.r1).toBe(w.rows)
  })

  it('пусто: r0 = r1 = 0, распорок нет', () => {
    const w = libWindow({ ...base, n: 0, scrollTop: 0 })
    expect([w.rows, w.r0, w.r1, w.padTop, w.padBottom]).toEqual([0, 0, 0, 0, 0])
  })

  it('один элемент и мало фото — один ряд целиком', () => {
    const w = libWindow({ ...base, n: 1, scrollTop: 0 })
    expect([w.rows, w.r0, w.r1, w.padTop, w.padBottom]).toEqual([1, 0, 1, 0, 0])
  })

  it('узкая лента — одна колонка, ряд на фото', () => {
    const w = libWindow({ n: 50, width: 120, height: 300, scrollTop: 0, size: 100 })
    expect(w.cols).toBe(1)
    expect(w.rows).toBe(50)
  })
})

describe('зум: якорь по индексу', () => {
  it('фото под курсором остаётся на месте при смене размера', () => {
    const width = 1000
    const n = 3200
    const st = 12345
    const cy = 140 // курсор от верха сетки
    const a = photoAt(n, width, 100, 400, st + cy)!
    for (const size of [60, 100, 180, 320]) {
      const cols = colsOf(width, size)
      const top = anchorTop(a.index, cols, size, cy, a.fy)
      // верх фото на экране = его верх в содержимом − прокрутка; точка fy остаётся под курсором
      const itemTop = PAD_Y + Math.floor(a.index / cols) * (size + GAP)
      expect(itemTop - top + a.fy * size).toBeCloseTo(cy, 6)
    }
  })

  it('без смены размера прокрутка не меняется', () => {
    const st = 5000
    const cy = 90
    const a = photoAt(3200, 1000, 100, 300, st + cy)!
    expect(anchorTop(a.index, colsOf(1000, 100), 100, cy, a.fy)).toBeCloseTo(st, 6)
  })

  it('photoAt: щель и край — ближайшая ячейка, пусто — null', () => {
    expect(photoAt(0, 1000, 100, 10, 10)).toBeNull()
    const a = photoAt(5, 1000, 100, 9999, 99999)!
    expect(a.index).toBe(4)
    expect(a.fy).toBe(1)
    expect(photoAt(5, 1000, 100, 0, 0)).toEqual({ index: 0, fy: 0 })
  })

  it('anchorTop не уходит в минус', () => {
    expect(anchorTop(0, 5, 100, 400, 0)).toBe(0)
  })
})

describe('anchorAfter: прокрутка при смене списка (088)', () => {
  const ids = (n: number, from = 0) => Array.from({ length: n }, (_, i) => `p${from + i}`)

  it('верхнее фото уцелело — его новый номер', () => {
    const prev = ids(20)
    const next = prev.filter((id) => id !== 'p2' && id !== 'p3') // удалили выше верхнего
    expect(anchorAfter(prev, next, 10)).toBe(8)
  })

  it('верхнее фото удалено — ближайшее следующее уцелевшее', () => {
    const prev = ids(20)
    const next = prev.filter((id) => id !== 'p10' && id !== 'p11')
    expect(next[anchorAfter(prev, next, 10)]).toBe('p12')
  })

  it('следующих уцелевших нет — прежний номер, зажатый в список; пустой список — 0', () => {
    expect(anchorAfter(ids(20), ids(5), 10)).toBe(4)
    expect(anchorAfter(ids(20), [], 10)).toBe(0)
  })

  it('новые файлы выше верхнего сдвигают якорь вниз', () => {
    const prev = ids(10)
    expect(anchorAfter(prev, [...ids(5, 100), ...prev], 4)).toBe(9)
  })
})

describe('keepScroll', () => {
  const size = 100
  const width = 1000
  const cols = colsOf(width, size)
  const ids = (n: number) => Array.from({ length: n }, (_, i) => `p${i}`)
  const rowTop = (index: number) => PAD_Y + Math.floor(index / cols) * (size + GAP)

  it('у самого верха и при прежнем списке — не трогаем', () => {
    expect(keepScroll(ids(100), ids(50), 0, width, size)).toBeNull()
    expect(keepScroll(ids(100), ids(100), 500, width, size)).toBeNull()
    expect(keepScroll([], ids(10), 500, width, size)).toBeNull()
    expect(keepScroll(ids(100), ids(50), 500, 0, size)).toBeNull() // лента свёрнута
  })

  it('удалили фото выше — верхний ряд остаётся на том же месте экрана', () => {
    const prev = ids(300)
    const st = rowTop(cols * 5) + 30 // пятый ряд, 30 px сверху срезано
    const next = prev.slice(cols * 2) // два верхних ряда пропали
    const to = keepScroll(prev, next, st, width, size)!
    expect(to).toBeCloseTo(rowTop(cols * 3) + 30, 6) // тот же ряд, теперь третий
  })

  it('прокрутка не уходит в минус', () => {
    const prev = ids(300)
    const to = keepScroll(prev, prev.slice(1), PAD_Y + 10, width, size)!
    expect(to).toBeGreaterThanOrEqual(0)
  })
})

describe('revealTop: прокрутка ленты к показанному (128)', () => {
  // Плитка 100, ширина на 3 колонки, высота окна 300: ряд i занимает [12 + 116·i, +100].
  const W = 2 * PAD_X + 3 * 100 + 2 * GAP
  const rev = (index: number, st: number) => revealTop(index, st, 300, W, 100)
  it('ряд уже виден — null', () => {
    expect(rev(0, 0)).toBeNull()
    expect(rev(4, 0)).toBeNull() // ряд 1: 128…228
    expect(rev(7, 116)).toBeNull()
  })
  it('ряд выше окна — прокрутка к нему с полем сверху', () => {
    expect(rev(0, 500)).toBe(0)
    expect(rev(6, 700)).toBe(12 + 2 * 116 - PAD_Y) // ряд 2
  })
  it('ряд ниже окна — он оказывается у нижнего края с полем', () => {
    // ряд 5: top = 12 + 580 = 592, bottom = 692; scrollTop = 692 + 12 − 300
    expect(rev(15, 0)).toBe(404)
  })
  it('ряд виден лишь частично — подтягивается', () => {
    expect(rev(9, 116)).toBe(12 + 3 * 116 + 100 + PAD_Y - 300) // ряд 3 торчит снизу: 360…460 при окне 116…416
  })
})
