// Перенос и масштаб группы (спека 009, design §4.1): каждая точка p → a + (p−a)·k + d.
// Зеркало Go (internal/geom/groupxf.go, паритет testdata/parity/groupxf.json): считает Go, это предпросмотр
// жеста, пока не отпущена кнопка. Всё в мм разворота.
import { aabb } from '../geom/textbox'
import { MIN_SIZE, MAX_SIZE } from '../../editor/text/textedit'
import { MIN_PLACE } from '../geom/grid'
import { aabbOf, holdOf, round2, roundAway } from '../geom/rotbox'
import type { Rect, RotRect } from '../core/types'

// Блок текста для расчёта: угол рамки до поворота, ширина, высота по тексту, угол и кегль (пт); al — доля
// выравнивания (0 / 0,5 / 1), нужна только groupAxes.
export interface GxText extends Rect {
  rot: number
  size: number
  al?: number
}

export interface GxIn {
  sw: number
  ph: number
  places: RotRect[]
  texts: GxText[]
  k: number
  ax: number
  ay: number
  dx: number
  dy: number
}

// Итог: действующие k и d после зажима, новые места и тексты (шаг 0.01, кегль 0.1).
export interface GxOut {
  k: number
  dx: number
  dy: number
  places: RotRect[]
  texts: GxText[]
}

const clamp = (v: number, lo: number, hi: number) => Math.max(lo, Math.min(hi, v))

// Описанный прямоугольник набора рамок; пусто — null.
export function bounds(rs: Rect[]): Rect | null {
  if (rs.length === 0) return null
  const x0 = Math.min(...rs.map((r) => r.x))
  const y0 = Math.min(...rs.map((r) => r.y))
  const x1 = Math.max(...rs.map((r) => r.x + r.w))
  const y1 = Math.max(...rs.map((r) => r.y + r.h))
  return { x: x0, y: y0, w: x1 - x0, h: y1 - y0 }
}

// Рамка текста в осях разворота (у повёрнутого — описанный прямоугольник).
export const textRect = (t: GxText): Rect => aabb(t)

// Рамка группы (пунктир GroupFrame, линейки, поля карточки): описанный прямоугольник того, что видно —
// повёрнутое место и текст входят своим aabb; пусто — null.
export const groupBox = (places: RotRect[], texts: GxText[]): Rect | null => bounds([...places.map(aabbOf), ...texts.map(textRect)])

// Общий угол членов группы (места и тексты, °): все равны — он, иначе или пусто — null.
export function commonAngle(rots: number[]): number | null {
  if (rots.length === 0) return null
  return rots.every((r) => Math.abs(r - rots[0]) < 1e-6) ? rots[0] : null
}

// Как groupXf в Go: k не больше, чем группа помещается в разворот и позволяет верхний предел
// кегля; не меньше, чем места остаются ≥ 5 мм и кегль ≥ нижнего предела (нижний сильнее). d сдвигается
// так, чтобы описанный прямоугольник группы после масштаба лежал внутри разворота; повёрнутое место
// входит в него своим hold (тогда зажим места после группы его не сдвигает), угол стоит.
export function groupXf(inp: GxIn): GxOut {
  const rects = [...inp.places.map(holdOf), ...inp.texts.map(textRect)]
  const bb = bounds(rects)
  if (!bb) return { k: 1, dx: 0, dy: 0, places: [], texts: [] }
  let k = inp.k
  if (bb.w > 0) k = Math.min(k, inp.sw / bb.w)
  if (bb.h > 0) k = Math.min(k, inp.ph / bb.h)
  for (const t of inp.texts) k = Math.min(k, MAX_SIZE / t.size)
  for (const r of inp.places) k = Math.max(k, MIN_PLACE / Math.min(r.w, r.h))
  for (const t of inp.texts) k = Math.max(k, MIN_SIZE / t.size)
  const nx = inp.ax + (bb.x - inp.ax) * k
  const ny = inp.ay + (bb.y - inp.ay) * k
  const dx = clamp(inp.dx, -nx, inp.sw - bb.w * k - nx)
  const dy = clamp(inp.dy, -ny, inp.ph - bb.h * k - ny)
  const mx = (v: number) => round2(inp.ax + (v - inp.ax) * k + dx)
  const my = (v: number) => round2(inp.ay + (v - inp.ay) * k + dy)
  return {
    k,
    dx,
    dy,
    places: inp.places.map((r) => ({ x: mx(r.x), y: my(r.y), w: round2(r.w * k), h: round2(r.h * k), rot: r.rot ?? 0 })),
    texts: inp.texts.map((t) => ({
      x: mx(t.x),
      y: my(t.y),
      w: round2(t.w * k),
      h: t.h * k,
      rot: t.rot,
      size: clamp(roundAway(t.size * k * 10) / 10, MIN_SIZE, MAX_SIZE),
    })),
  }
}

export type GroupCorner = 'nw' | 'ne' | 'se' | 'sw'

// Неподвижный угол при масштабе за угол corner — противоположный.
export function anchorOf(bb: Rect, corner: GroupCorner): { x: number; y: number } {
  return { x: corner.includes('w') ? bb.x + bb.w : bb.x, y: corner.includes('n') ? bb.y + bb.h : bb.y }
}

// Угол, который тянут: сам corner.
export function cornerPt(bb: Rect, corner: GroupCorner): { x: number; y: number } {
  return { x: corner.includes('w') ? bb.x : bb.x + bb.w, y: corner.includes('n') ? bb.y : bb.y + bb.h }
}

// Пропорциональный k по сдвигу угла (dx, dy): проекция на диагональ от неподвижного угла.
// Вырожденная рамка — 1 (масштабировать нечего).
export function cornerK(bb: Rect, corner: GroupCorner, dx: number, dy: number): number {
  const a = anchorOf(bb, corner)
  const c = cornerPt(bb, corner)
  const vx = c.x - a.x
  const vy = c.y - a.y
  const len2 = vx * vx + vy * vy
  if (len2 === 0) return 1
  return (vx * (vx + dx) + vy * (vy + dy)) / len2
}
