// Сетка ячеек окна сравнения (138): чистые функции без Svelte. Все ячейки одного размера;
// берётся сетка, где самый мелкий кадр получает наибольшую площадь.
import type { Size } from '../../editor/photo/loupe'

export interface Cell {
  left: number
  top: number
  w: number
  h: number
} // px в окне, без подписи кадра

const EPS = 1e-6
const SQUARE = 1

// Пропорция w/h; неизвестная (0, NaN) считается квадратной: ячейка всё равно рисуется.
const ratio = (a: number): number => (a > 0 && Number.isFinite(a) ? a : SQUARE)

// Площадь кадра пропорции a, вписанного в ячейку w×h.
function inscribed(a: number, w: number, h: number): number {
  if (w <= 0 || h <= 0) return 0
  return a > w / h ? w * (w / a) : h * a * h
}

interface Grid {
  rows: number
  cols: number
  w: number
  h: number
  score: number
}

// Сетка из rows рядов; столбцов ровно столько, чтобы вместить n кадров. Подпись cap вычитается из высоты ячейки.
function grid(aspects: readonly number[], box: Size, gap: number, cap: number, rows: number): Grid {
  const n = aspects.length
  const cols = Math.ceil(n / rows)
  const used = Math.ceil(n / cols) // ряды без кадров не нужны
  const w = Math.max(0, (box.w - gap * (cols - 1)) / cols)
  const h = Math.max(0, (box.h - gap * (used - 1)) / used - cap)
  return { rows: used, cols, w, h, score: Math.min(...aspects.map((a) => inscribed(ratio(a), w, h))) }
}

// Ячейки в порядке кадров. Кандидаты — r рядов × ⌈n/r⌉ столбцов (r = 1…n), неполный последний ряд по центру.
// Лучший — по минимальной площади вписанного кадра; при ничьей меньше рядов. cap — высота подписи под кадром.
export function tiles(aspects: readonly number[], box: Size, gap: number, cap: number): Cell[] {
  const n = aspects.length
  if (n === 0) return []
  let best: Grid | null = null
  for (let r = 1; r <= n; r++) {
    const g = grid(aspects, box, gap, cap, r)
    if (!best || g.score > best.score + EPS) best = g
  }
  const g = best as Grid
  return aspects.map((_, i) => {
    const row = Math.floor(i / g.cols)
    const inRow = Math.min(g.cols, n - row * g.cols)
    const shift = ((g.cols - inRow) * (g.w + gap)) / 2
    return { left: shift + (i % g.cols) * (g.w + gap), top: row * (g.h + cap + gap), w: g.w, h: g.h }
  })
}
