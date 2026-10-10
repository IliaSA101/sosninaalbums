// Правка мест сетки (спека 007): предпросмотр переноса и размера до отпускания кнопки.
// Всё в мм разворота. Итог всё равно считает и зажимает Go (SetPlaces) — здесь только то, что нужно
// для плавного предпросмотра: места не вылетают за разворот и не меньше минимума.
import { clampBox } from './snap'
import type { Handle, Rect } from '../core/types'

// Размер разворота: w = 2·pageW, h = pageH.
export interface Bounds {
  w: number
  h: number
}

// Минимальная сторона места, мм (как minPlace в Go).
export const MIN_PLACE = 5

// Шаг стрелок: 1 мм, с Shift — 10 мм.
export const NUDGE_MM = 1
export const NUDGE_BIG_MM = 10

// Округление до 0.01 мм, как в Go: предпросмотр совпадает с тем, что вернётся.
export const round2 = (n: number) => Math.round(n * 100) / 100
export const roundBox = (b: Rect): Rect => ({ x: round2(b.x), y: round2(b.y), w: round2(b.w), h: round2(b.h) })

const hasW = (h: Handle) => h.includes('w')
const hasE = (h: Handle) => h.includes('e')
const hasN = (h: Handle) => h.includes('n')
const hasS = (h: Handle) => h.includes('s')

// Свободное растяжение: каждый край, за который держим, двигается отдельно.
function resizeFree(box: Rect, handle: Handle, dx: number, dy: number, min: number, maxW: number, maxH: number): Rect {
  let x0 = box.x
  let x1 = box.x + box.w
  let y0 = box.y
  let y1 = box.y + box.h
  if (hasW(handle)) x0 = Math.min(Math.max(x0 + dx, 0), x1 - min)
  if (hasE(handle)) x1 = Math.max(Math.min(x1 + dx, maxW), x0 + min)
  if (hasN(handle)) y0 = Math.min(Math.max(y0 + dy, 0), y1 - min)
  if (hasS(handle)) y1 = Math.max(Math.min(y1 + dy, maxH), y0 + min)
  return { x: x0, y: y0, w: x1 - x0, h: y1 - y0 }
}

// Масштаб при сохранении пропорций: у угла — по большему относительному сдвигу, у боковой ручки — по своей оси.
function lockScale(box: Rect, handle: Handle, dx: number, dy: number): number {
  const sw = (box.w + (hasW(handle) ? -dx : dx)) / box.w
  const sh = (box.h + (hasN(handle) ? -dy : dy)) / box.h
  const horiz = hasW(handle) || hasE(handle)
  const vert = hasN(handle) || hasS(handle)
  if (horiz && vert) return Math.abs(sw - 1) >= Math.abs(sh - 1) ? sw : sh
  return horiz ? sw : sh
}

// Растяжение с сохранением пропорций: противоположный край (угол) неподвижен,
// у боковой ручки второй размер меняется вокруг центра.
function resizeLocked(box: Rect, handle: Handle, dx: number, dy: number, min: number, maxW: number, maxH: number): Rect {
  const { x, y, w, h } = box
  if (!(w > 0) || !(h > 0)) return { ...box }
  const x1 = x + w
  const y1 = y + h
  let s = lockScale(box, handle, dx, dy)

  // Предел роста: сколько места до края разворота в сторону роста (у боковой ручки — симметрично от центра).
  const cx = x + w / 2
  const cy = y + h / 2
  const roomW = hasW(handle) ? x1 : hasE(handle) ? maxW - x : 2 * Math.min(cx, maxW - cx)
  const roomH = hasN(handle) ? y1 : hasS(handle) ? maxH - y : 2 * Math.min(cy, maxH - cy)
  const upper = Math.min(roomW / w, roomH / h)
  const lower = Math.max(min / w, min / h)
  // Минимум важнее предела: если место и так не лезет, оно не должно схлопнуться.
  s = Math.max(lower, Math.min(s, upper))

  const w2 = w * s
  const h2 = h * s
  return {
    x: hasW(handle) ? x1 - w2 : hasE(handle) ? x : cx - w2 / 2,
    y: hasN(handle) ? y1 - h2 : hasS(handle) ? y : cy - h2 / 2,
    w: w2,
    h: h2,
  }
}

// Тянем ручку handle на (dx, dy). lock — сохранить пропорции. min — минимальная сторона;
// bounds (если задан) не даёт выйти за разворот.
export function resizeBox(box: Rect, handle: Handle, dx: number, dy: number, lock: boolean, min = MIN_PLACE, bounds?: Bounds): Rect {
  const maxW = bounds?.w ?? Infinity
  const maxH = bounds?.h ?? Infinity
  return lock ? resizeLocked(box, handle, dx, dy, min, maxW, maxH) : resizeFree(box, handle, dx, dy, min, maxW, maxH)
}

// Перенос одного места: размер сохраняется, место остаётся внутри разворота.
export function moveBox(box: Rect, dx: number, dy: number, bounds: Bounds): Rect {
  return clampBox({ ...box, x: box.x + dx, y: box.y + dy }, bounds.w, bounds.h)
}

// Перенос выделенных мест вместе: сдвиг ограничен так, чтобы ни одно не вышло за разворот
// (взаимное положение сохраняется). Порядок и число мест те же.
export function moveBoxes(boxes: Rect[], dx: number, dy: number, bounds: Bounds): Rect[] {
  if (boxes.length === 0) return []
  const left = Math.min(...boxes.map((b) => b.x))
  const top = Math.min(...boxes.map((b) => b.y))
  const right = Math.max(...boxes.map((b) => b.x + b.w))
  const bottom = Math.max(...boxes.map((b) => b.y + b.h))
  const ddx = Math.min(Math.max(dx, -left), bounds.w - right)
  const ddy = Math.min(Math.max(dy, -top), bounds.h - bottom)
  return boxes.map((b) => ({ ...b, x: b.x + ddx, y: b.y + ddy }))
}

// Сдвиг стрелкой в мм: Shift — крупный шаг. Не стрелка — нулевой сдвиг.
export function arrowDelta(key: string, shift: boolean): { dx: number; dy: number } {
  const step = shift ? NUDGE_BIG_MM : NUDGE_MM
  switch (key) {
    case 'ArrowLeft':
      return { dx: -step, dy: 0 }
    case 'ArrowRight':
      return { dx: step, dy: 0 }
    case 'ArrowUp':
      return { dx: 0, dy: -step }
    case 'ArrowDown':
      return { dx: 0, dy: step }
  }
  return { dx: 0, dy: 0 }
}

// Сдвиг выделенных мест стрелкой.
export function nudge(boxes: Rect[], key: string, shift: boolean, bounds: Bounds): Rect[] {
  const { dx, dy } = arrowDelta(key, shift)
  return moveBoxes(boxes, dx, dy, bounds)
}
