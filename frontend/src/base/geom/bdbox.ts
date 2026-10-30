// Рамка фото на фоне в режиме фона (спека 009, design-backdrop §6.3): предпросмотр жестов, пока кнопка
// не отпущена. Окончательно зажимает Go (clampBackdrop); здесь те же пределы, чтобы рамка не прыгала
// после ответа. Всё в мм разворота; поворот — как у текста (geom/textbox.ts).
import { panFrame } from './frame'
import { snapBox, type SnapLines } from './snap'
import { aabb, center, cursorDeg, dragAngle, normDeg, rotate, scaleCorner, type Corner, type TextBox } from './textbox'
import type { Frame, Rect } from '../core/types'

// Как в Go (limits_design.go): рамка 10 мм .. 3 области по каждой оси.
export const BD_MIN = 10
export const BD_MAX_K = 3

export type Side = 't' | 'r' | 'b' | 'l'

const clamp = (v: number, lo: number, hi: number) => Math.min(hi, Math.max(lo, v))
const uniq = (a: number[]) => [...new Set(a.map((n) => Math.round(n * 1000) / 1000))].sort((x, y) => x - y)

// Линии прилипания рамки: края и центр области, шов (mid > 0). У обложки (032) шва нет: вместо него — края
// и центры сторонок (sides, см. coverSides): рамка прилипает к корешку с обеих сторон, а не к его середине.
export function bdLines(area: Rect, mid: number, sides?: readonly (readonly [number, number])[]): SnapLines {
  const seam = sides?.length ? sides.flatMap(([a, b]) => [a, (a + b) / 2, b]) : mid > 0 ? [mid] : []
  return {
    v: uniq([area.x, area.x + area.w / 2, area.x + area.w, ...seam]),
    h: uniq([area.y, area.y + area.h / 2, area.y + area.h]),
  }
}

export interface BdMove {
  x: number
  y: number
  guideX: number | null
  guideY: number | null
}

// Перенос: описанный прямоугольник прилипает краями и центром (thr = 0 — без прилипания, Alt), затем
// центр рамки зажимается в область, как в Go: фон нельзя потерять за краем.
export function moveFrame(t: TextBox, dx: number, dy: number, area: Rect, lines: SnapLines, thr: number): BdMove {
  const a = aabb(t)
  const m = { x: a.x + dx, y: a.y + dy, w: a.w, h: a.h }
  const s = thr > 0 ? snapBox(m, lines, thr) : { x: m.x, y: m.y, guideX: null, guideY: null }
  const x = t.x + (s.x - a.x)
  const y = t.y + (s.y - a.y)
  const c = center({ ...t, x, y })
  const cx = clamp(c.x, area.x, area.x + area.w) - c.x
  const cy = clamp(c.y, area.y, area.y + area.h) - c.y
  return { x: x + cx, y: y + cy, guideX: cx === 0 ? s.guideX : null, guideY: cy === 0 ? s.guideY : null }
}

// Середина стороны: размер по одной оси рамки, противоположная сторона стоит. Сдвиг мыши (dx, dy)
// переводится в оси рамки; левая и верхняя ручки уводят угол (x, y) вдоль своей оси.
export function resizeSide(t: TextBox, side: Side, dx: number, dy: number, area: Rect): TextBox {
  const d = rotate(-t.rot, dx, dy)
  const w = (v: number) => clamp(v, BD_MIN, BD_MAX_K * area.w)
  const h = (v: number) => clamp(v, BD_MIN, BD_MAX_K * area.h)
  switch (side) {
    case 'r':
      return { ...t, w: w(t.w + d.x) }
    case 'b':
      return { ...t, h: h(t.h + d.y) }
    case 'l': {
      const nw = w(t.w - d.x)
      const o = rotate(t.rot, t.w - nw, 0)
      return { ...t, x: t.x + o.x, y: t.y + o.y, w: nw }
    }
    case 't': {
      const nh = h(t.h - d.y)
      const o = rotate(t.rot, 0, t.h - nh)
      return { ...t, x: t.x + o.x, y: t.y + o.y, h: nh }
    }
  }
}

// Угол: пропорциональный масштаб от противоположного угла (он стоит), в пределах 10 мм .. 3 области.
export function scaleFrame(t: TextBox, corner: Corner, dx: number, dy: number, area: Rect): TextBox {
  const kMin = Math.max(BD_MIN / t.w, BD_MIN / t.h)
  const kMax = Math.max(kMin, Math.min((BD_MAX_K * area.w) / t.w, (BD_MAX_K * area.h) / t.h))
  const r = scaleCorner(t, corner, dx, dy, kMin, kMax)
  return { ...t, x: r.x, y: r.y, w: r.w, h: r.h }
}

// Угол ⟳: Shift — шаг 15°; без него прилипание к 0, ±90, 180 в пределах 2°; Alt — без прилипания.
export function bdAngle(
  c: { x: number; y: number },
  p: { x: number; y: number },
  grab: number,
  rot0: number,
  shift: boolean,
  alt: boolean,
): number {
  if (shift || !alt) return dragAngle(c, p, grab, rot0, shift)
  const v = rotate(-(grab - rot0), p.x - c.x, p.y - c.y)
  return normDeg(cursorDeg(c, { x: c.x + v.x, y: c.y + v.y }))
}

// Стрелки: рамка на 1 мм, с Shift — на 10 мм; не стрелка — null.
export function nudgeBox(t: TextBox, key: string, shift: boolean): TextBox | null {
  const s = shift ? 10 : 1
  switch (key) {
    case 'ArrowLeft':
      return { ...t, x: t.x - s }
    case 'ArrowRight':
      return { ...t, x: t.x + s }
    case 'ArrowUp':
      return { ...t, y: t.y - s }
    case 'ArrowDown':
      return { ...t, y: t.y + s }
  }
  return null
}

// ✥: сдвиг мыши (мм разворота) переводится в оси рамки, дальше — как кадр ячейки.
export function panBox(f: Frame, t: TextBox, img: Rect, dx: number, dy: number): Frame {
  const d = rotate(-t.rot, dx, dy)
  return panFrame(f, { x: 0, y: 0, w: t.w, h: t.h }, img, d.x, d.y)
}
