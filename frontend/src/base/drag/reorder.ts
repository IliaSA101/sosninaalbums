// Живая перестановка разворотов (096): чистая часть. Слоты замерены один раз в начале жеста, в координатах
// содержимого и без смещений — поэтому промежуток не дрожит: смещения на выбор места не влияют.

export interface Slot {
  x: number
  y: number
  w: number
  h: number
}
// line — лента (один ряд, ширины разные), grid — «Все развороты» (ячейки равны, порядок чтения).
export type Mode = 'line' | 'grid'
export interface Off {
  dx: number
  dy: number
}
export interface Area {
  left: number
  top: number
  right: number
  bottom: number
}

export const REORDER_OUT = 60 // px за видимой частью — отмена
export const REORDER_MS = 150

const ZERO: Off = { dx: 0, dy: 0 }

// Слот стоит «до» точки: в ленте — его середина левее; в сетке — его ряд выше точки целиком или это ряд точки
// и середина левее. Точка между рядами относится к нижнему ряду.
function before(s: Slot, px: number, py: number, mode: Mode): boolean {
  const left = s.x + s.w / 2 < px
  if (mode === 'line') return left
  return py >= s.y + s.h || (py >= s.y && left)
}

// Итоговый индекс (to для MoveSpread): сколько слотов, кроме from, стоят до точки.
export function gapAt(slots: Slot[], from: number, px: number, py: number, mode: Mode): number {
  let n = 0
  for (let k = 0; k < slots.length; k++) if (k !== from && before(slots[k], px, py, mode)) n++
  return n
}

// На сколько позиций сдвигается слот k, когда from встаёт на to.
export function shiftOf(k: number, from: number, to: number): -1 | 0 | 1 {
  if (from < to && k > from && k <= to) return -1
  if (to < from && k >= to && k < from) return 1
  return 0
}

// Зазор между соседними слотами ленты (ширина черты склейки); один слот — без зазора.
const spacing = (slots: Slot[]): number => (slots.length > 1 ? slots[1].x - slots[0].x - slots[0].w : 0)

// Смещение слота k на время перестановки, px.
export function offsetOf(slots: Slot[], k: number, from: number, to: number, mode: Mode): Off {
  const d = shiftOf(k, from, to)
  if (d === 0) return ZERO
  if (mode === 'line') return { dx: d * (slots[from].w + spacing(slots)), dy: 0 }
  const a = slots[k]
  const b = slots[k + d]
  return { dx: b.x - a.x, dy: b.y - a.y }
}

// Где окажется поднятый после броска, относительно своего слота.
export function landOf(slots: Slot[], from: number, to: number, mode: Mode): Off {
  if (to === from) return ZERO
  const f = slots[from]
  const t = slots[to]
  if (mode === 'grid') return { dx: t.x - f.x, dy: t.y - f.y }
  if (to < from) return { dx: t.x - f.x, dy: 0 }
  return { dx: t.x + t.w - f.w - f.x, dy: 0 }
}

// Точка не дальше pad от видимой части области.
export function inside(area: Area, x: number, y: number, pad = REORDER_OUT): boolean {
  return x >= area.left - pad && x <= area.right + pad && y >= area.top - pad && y <= area.bottom + pad
}
