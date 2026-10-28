// Место столбца мини-панели (спека 108, owner-ux §1, владелец 2026-10-23): справа от фото, иначе слева, иначе
// внутри у правого края; не помещается — запасной ряд (minipanel.ts, miniPlace). Чистая логика; рисует MiniPanel.svelte.
import type { Rect } from '../../base/core/types'
import { arcWhere, COL_IN_GAP, COL_W, miniPlace, ROT_KEEP, type ColWhere, type MiniView, type MiniWhere } from './minipanel'

export { COL_W } from './minipanel'
export type { ColWhere } from './minipanel'

// Размеры значков столбца (px): кнопка, линия-разделитель с отступами, зазор между потомками, рамка и отступы столбца.
const BTN_H = 22
const SEP_H = 7
const GAP = 2
const FRAME = 10
const MARGIN = 4 // воздух до края видимой области
const SIDE_GAP = 8 // от рамки фото до столбца сбоку
// Внутри: видимая часть рамки не уже трёх столбцов и не ниже столбца + 28 (по 14 сверху и снизу)
const IN_MIN_W = 3 * COL_W
const IN_MIN_PAD = 28
const CAP_GAP = 8 // от подписи до низа столбца внутри

// Высота столбца по составу, а не замером: выбор «столбец или ряд» не должен зависеть от того, какой вид нарисован
// сейчас (иначе панель скакала бы между видами). Константы совпадают с CSS MiniPanel.svelte.
// Г одно фото — 245, с «Подогнать» — 269, группа — 188, «Простой» — 98.
export const colHeight = (buttons: number, seps: number): number => buttons * BTN_H + seps * SEP_H + (buttons + seps - 1) * GAP + FRAME

// То же по списку значков (линия перед значком — sep); пусто — 0.
export const colHeightOf = (items: { sep?: boolean }[]): number =>
  items.length === 0 ? 0 : colHeight(items.length, items.filter((i) => i.sep).length)

export interface ColSpot {
  left: number // левый край столбца, px от угла разворота (без translateX)
  top: number
  where: ColWhere
}

const hits = (av: { x: number; y: number } | undefined, left: number, top: number, h: number): boolean =>
  !!av && av.x > left - ROT_KEEP && av.x < left + COL_W + ROT_KEEP && av.y > top - ROT_KEEP && av.y < top + h + ROT_KEEP

// Внутри у правого края видимой части рамки: по середине по высоте, подпись у низа поднимает столбец над собой.
function inRight(x0: number, y0: number, x1: number, y1: number, ppm: number, h: number, view: MiniView, caps: Rect[]): ColSpot | null {
  const vl = Math.max(x0, view.left)
  const vr = Math.min(x1, view.right)
  const vt = Math.max(y0, view.top)
  const vb = Math.min(y1, view.bottom)
  if (vr - vl < IN_MIN_W || vb - vt < h + IN_MIN_PAD) return null
  const left = vr - COL_IN_GAP - COL_W
  let top = (vt + vb) / 2 - h / 2
  const px = caps
    .map((c) => ({ l: c.x * ppm, r: (c.x + c.w) * ppm, t: c.y * ppm, b: (c.y + c.h) * ppm }))
    .filter((c) => c.r > left && c.l < left + COL_W) // только под столбцом по горизонтали
    .sort((a, b) => b.t - a.t) // снизу вверх: сдвинули над нижней — проверяем следующую
  for (const c of px) if (c.t < top + h + CAP_GAP && c.b > top) top = c.t - CAP_GAP - h
  if (top < vt + COL_IN_GAP) return null
  return { left, top, where: 'inright' }
}

// Место столбца высотой h (px) у рамки box (мм); все величины — px от угла разворота, как у miniPlace. null — запасной ряд.
// Соседние места не проверяются (108-2): сторона зависит только от рамки и видимой области. avoid — центр ⟳ повёрнутого
// места: сторона, на которую столбец лёг бы на него, пропускается. caps — подписи и заглушки выделенного, мм.
export function colPlace(
  box: Rect,
  ppm: number,
  h: number,
  view: MiniView,
  avoid?: { x: number; y: number },
  caps: Rect[] = [],
): ColSpot | null {
  if (!(h > 0)) return null
  const x0 = box.x * ppm
  const y0 = box.y * ppm
  const x1 = (box.x + box.w) * ppm
  const y1 = (box.y + box.h) * ppm
  // Фото целиком выше или ниже видимой области: столбцу не к чему прижаться (иначе он повис бы на чужом содержимом).
  // null → запасной ряд, а его панель прячет panelOutside — как в 0.23.
  if (y1 <= view.top || y0 >= view.bottom) return null
  // Низкий холст: низ фото виден — столбец заходит на ленту разворотов (owner-ux §1.8, как §1.9).
  let floor = view.bottom
  if (view.bottom - view.top < h + 2 * MARGIN && view.spill !== undefined && y1 <= view.bottom) floor = Math.max(view.bottom, view.spill)
  if (floor - view.top < h + 2 * MARGIN) return null
  // Верх вровень с верхом фото; у низа видимой области поднимается.
  const top = Math.max(view.top + MARGIN, Math.min(y0, floor - h - MARGIN))
  if (x1 + SIDE_GAP + COL_W <= view.right - MARGIN && !hits(avoid, x1 + SIDE_GAP, top, h))
    return { left: x1 + SIDE_GAP, top, where: 'right' }
  const left = x0 - SIDE_GAP - COL_W
  if (left >= view.left + MARGIN && !hits(avoid, left, top, h)) return { left, top, where: 'left' }
  const inner = inRight(x0, y0, x1, y1, ppm, h, view, caps)
  return inner && !hits(avoid, inner.left, inner.top, h) ? inner : null
}

// Место мини-панели: столбец, а если он не помещается — прежний ряд (под → над → внутри у низа → у верха).
// w — ширина ряда (замер), h — высота столбца (colHeightOf).
export function placeMini(
  box: Rect,
  ppm: number,
  width: number,
  height: number,
  w: number,
  h: number,
  avoid: { x: number; y: number } | undefined,
  capBottom: number,
  view: MiniView,
  caps: Rect[],
): { top: number; left: number; where: MiniWhere } {
  return colPlace(box, ppm, h, view, avoid, caps) ?? miniPlace(box, ppm, width, height, w, avoid, capBottom, view, caps)
}

// Где встанет панель в режиме наклона (⟳ спрятан): тем же путём, что placeMini, — от этого места считается место дуге.
export function arcWhereOf(
  box: Rect,
  ppm: number,
  width: number,
  height: number,
  w: number,
  h: number,
  capBottom: number,
  view: MiniView,
  caps: Rect[],
): MiniWhere {
  return colPlace(box, ppm, h, view, undefined, caps)?.where ?? arcWhere(box, ppm, width, height, w, capBottom, view, caps)
}
