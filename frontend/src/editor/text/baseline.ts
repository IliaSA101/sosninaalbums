// Редактор надписи встаёт на строки Go (спека 156). Go ставит первую базу на asc шрифта и шаг 1,2 кегля,
// CSS — на (lh − (A + D))/2 + A; разницу гасят отступы и сдвиг. Геометрию даёт Go (lines, h), здесь только вычитание.

export interface Shift {
  padTop: number // px, ≥ 0
  padBottom: number // px, ≥ 0
  shiftY: number // px, ≤ 0: translateY после rotate
  minH: number // px: lh + padTop + padBottom; 0 — поправки нет, min-height остаётся прежним
}

const ZERO: Shift = { padTop: 0, padBottom: 0, shiftY: 0, minH: 0 }

// Предел поправки в кеглях: чужой .salbum вёрстку не раздует.
const LIMIT = 2

// Поправка редактора. lines и box — из View (мм), base — база первой строки в браузере (px, measureBase),
// sizePx — кегль в px, k — px на мм. asc = lines[0].y − y, desc = y + h − lines[n−1].y.
// top < 0 (низкий шрифт: asc + desc < 1,2 кегля) CSS-отступом не выразить — уходит в shiftY, обводка шире рамки Go.
export function editorShift(lines: { y: number }[], box: { y: number; h: number }, base: number | null, sizePx: number, k: number): Shift {
  const n = lines.length
  if (n === 0 || base === null || !(sizePx > 0) || !(k > 0)) return ZERO
  const lh = 1.2 * sizePx
  const asc = lines[0].y - box.y
  const desc = box.y + box.h - lines[n - 1].y
  const lim = LIMIT * sizePx
  const clamp = (v: number) => Math.max(-lim, Math.min(lim, v))
  const top = clamp(asc * k - base)
  const bottom = clamp(desc * k - (lh - base))
  if (!Number.isFinite(top) || !Number.isFinite(bottom)) return ZERO
  const padTop = Math.max(top, 0)
  const padBottom = Math.max(bottom, 0)
  return { padTop, padBottom, shiftY: Math.min(top, 0), minH: lh + padTop + padBottom }
}

// База первой строки зонда: низ пустого inline-block `.pb` лежит на базовой линии. Без деления на масштаб:
// зум меняет ширину, а не CSS scale. null — нет span или нулевая раскладка (happy-dom, скрытый предок).
export function measureBase(probe: HTMLElement): number | null {
  const pb = probe.querySelector('.pb')
  if (!pb) return null
  const p = probe.getBoundingClientRect()
  if (!(p.height > 0)) return null
  const v = pb.getBoundingClientRect().top - p.top
  return Number.isFinite(v) ? v : null
}
