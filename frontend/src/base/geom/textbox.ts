// Геометрия повёрнутого блока текста (спека 008, design §3, §6.4). Зеркало Go (internal/geom/textbox.go):
// считает Go, это предпросмотр жестов, пока не отпущена кнопка. Всё в мм разворота.
// Угол θ — по часовой на экране (ось y вниз), поворот вокруг левого верхнего угла (x, y).
import { moveBox, MIN_BOX_W, type MoveResult } from '../../editor/text/textedit'
import type { SnapLines } from './snap'
import type { Rect } from '../core/types'

export type Align = 'left' | 'center' | 'right'

// Рамка блока до поворота; подходит и TextView.
export interface TextBox {
  x: number
  y: number
  w: number
  h: number
  rot: number
  align?: Align
}

export interface Pt {
  x: number
  y: number
}

// Угол в (−180, 180], шаг 0.1; |d| < 0.05 → 0. Округление как math.Round в Go (от нуля).
export function normDeg(d: number): number {
  if (!Number.isFinite(d)) return 0
  let m = d % 360
  if (m > 180) m -= 360
  else if (m <= -180) m += 360
  const sign = m < 0 ? -1 : 1
  let r = (sign * Math.round(Math.abs(m) * 10)) / 10
  if (r <= -180) r = 180
  return Math.abs(r) < 0.05 ? 0 : r
}

// sin и cos; кратные 90° — ровно, чтобы при 0° и 90° не копились ошибки.
function sincos(deg: number): [number, number] {
  const m = ((deg % 360) + 360) % 360
  if (m === 0) return [0, 1]
  if (m === 90) return [1, 0]
  if (m === 180) return [0, -1]
  if (m === 270) return [-1, 0]
  const r = (deg * Math.PI) / 180
  return [Math.sin(r), Math.cos(r)]
}

// R(θ)·(u, v)
export function rotate(deg: number, u: number, v: number): Pt {
  const [s, c] = sincos(deg)
  return { x: u * c - v * s, y: u * s + v * c }
}

// Локальная точка рамки (u, v) → разворот.
export function toSpread(t: TextBox, u: number, v: number): Pt {
  const r = rotate(t.rot, u, v)
  return { x: t.x + r.x, y: t.y + r.y }
}

export function corners(t: TextBox): [Pt, Pt, Pt, Pt] {
  return [toSpread(t, 0, 0), toSpread(t, t.w, 0), toSpread(t, 0, t.h), toSpread(t, t.w, t.h)]
}

// Описанный прямоугольник; при θ = 0 — сама рамка.
export function aabb(t: TextBox): Rect {
  const cs = corners(t)
  const xs = cs.map((p) => p.x)
  const ys = cs.map((p) => p.y)
  const x = Math.min(...xs)
  const y = Math.min(...ys)
  return { x, y, w: Math.max(...xs) - x, h: Math.max(...ys) - y }
}

export const center = (t: TextBox): Pt => toSpread(t, t.w / 2, t.h / 2)

// Тот же блок, повёрнутый на угол deg вокруг своего центра: центр стоит, меняются x, y и rot.
export function rotateAbout(t: TextBox, deg: number): TextBox {
  const c = center(t)
  const r = rotate(deg, t.w / 2, t.h / 2)
  return { ...t, x: c.x - r.x, y: c.y - r.y, rot: deg }
}

export const ALIGN_X: Record<Align, number> = { left: 0, center: 0.5, right: 1 }

// Точка, которая стоит при росте ширины «по тексту»: левый, средний или правый край верхней стороны.
export const anchor = (t: TextBox): Pt => toSpread(t, ALIGN_X[t.align ?? 'left'] * t.w, 0)

// Угол направления c → p без округления: «вверх» = 0°, по часовой растёт; (−90, 270].
export const cursorDeg = (c: Pt, p: Pt): number => (Math.atan2(p.y - c.y, p.x - c.x) * 180) / Math.PI + 90

// Угол ручки поворота: c — центр блока на экране, p — курсор (px). Ручка над серединой верхней стороны,
// поэтому направление «вверх» = 0°. Shift — шаг 15°; без него — прилипание к 0/±90/180 в пределах 2°.
export function angleAt(c: Pt, p: Pt, shift: boolean): number {
  const deg = cursorDeg(c, p)
  if (shift) return normDeg(Math.round(deg / 15) * 15)
  const n = normDeg(deg)
  for (const s of [0, 90, -90, 180]) {
    // расстояние до цели по кругу, чтобы −179.5 прилипал к 180
    const d = Math.abs(((n - s + 540) % 360) - 180)
    if (d <= 2) return normDeg(s)
  }
  return n
}

export interface LocalResize {
  x: number
  y: number
  w: number
}

// Боковая ручка повёрнутого блока: дельта мыши (dx, dy) проецируется на ось строки, без прилипания.
// Правая двигает правый край (P стоит), левая — левый (P уходит вдоль оси). Ширина 10..sheetW (ширина листа, мм).
export function resizeLocal(t: TextBox, side: 'l' | 'r', dx: number, dy: number, sheetW: number): LocalResize {
  const [s, c] = sincos(t.rot)
  const du = dx * c + dy * s
  const max = sheetW
  if (side === 'r') {
    return { x: t.x, y: t.y, w: Math.min(max, Math.max(MIN_BOX_W, t.w + du)) }
  }
  const w = Math.min(max, Math.max(MIN_BOX_W, t.w - du))
  const p = rotate(t.rot, t.w - w, 0)
  return { x: t.x + p.x, y: t.y + p.y, w }
}

// Перенос повёрнутого блока: описанный прямоугольник двигается, как рамка прямого блока
// (в пределах разворота, с прилипанием), и тот же сдвиг получает P.
export function moveRotated(t: TextBox, dx: number, dy: number, lines: SnapLines, thr: number, spreadW: number, pageH: number): MoveResult {
  const a = aabb(t)
  const r = moveBox(a, dx, dy, lines, thr, spreadW, pageH)
  return { x: t.x + (r.x - a.x), y: t.y + (r.y - a.y), guideX: r.guideX, guideY: r.guideY }
}

export type Corner = 'tl' | 'tr' | 'bl' | 'br'

export interface ScaleResult {
  k: number // во сколько раз меняется кегль
  x: number // опора — (x, y) блока: Go в SetTextScale держит левый верхний угол до поворота
  y: number
  w: number
  h: number
}

// Угловая ручка: кегль и рамка растут вместе. Вектор v — от противоположного угла к ручке (в осях блока),
// сдвиг мыши (dx, dy) переводится в эти оси; k — проекция нового вектора на старый. Опора для всех углов
// одна, (x, y): иначе правка стала бы двумя шагами undo (SetTextScale + SetTextBox). Предел k задаёт вызывающий
// (кегль 6..200 пт); ширина не меньше MIN_BOX_W.
export function scaleCorner(t: TextBox, corner: Corner, dx: number, dy: number, kMin = 0, kMax = Infinity): ScaleResult {
  const v = { x: corner === 'tl' || corner === 'bl' ? -t.w : t.w, y: corner === 'tl' || corner === 'tr' ? -t.h : t.h }
  const d = rotate(-t.rot, dx, dy)
  const len2 = v.x * v.x + v.y * v.y
  let k = len2 > 0 ? (v.x * (v.x + d.x) + v.y * (v.y + d.y)) / len2 : 1
  if (t.w > 0) kMin = Math.max(kMin, MIN_BOX_W / t.w)
  k = Math.min(kMax, Math.max(kMin, k))
  return { k, ...scaleAnchor(t, corner, k), w: t.w * k, h: t.h * k }
}

// Угол рамки (x, y) после роста в k раз, при котором противоположный тянутому угол стоит:
// в осях блока он в O, угол рамки уезжает на (1 − k)·R·O (Go принимает новые x, y в SetTextScale).
export function scaleAnchor(t: TextBox, corner: Corner, k: number): Pt {
  const o = rotate(t.rot, corner === 'tl' || corner === 'bl' ? t.w : 0, corner === 'tl' || corner === 'tr' ? t.h : 0)
  return { x: t.x + (1 - k) * o.x, y: t.y + (1 - k) * o.y }
}

// Выравнивание как в Go (alignX): неизвестное — по центру.
export const alignOf = (a: string | undefined): Align => (a === 'left' || a === 'right' ? a : 'center')

// Рамка блока из View (TextView и CaptionView подходят) с выравниванием стиля.
export const boxOf = (t: { x: number; y: number; w: number; h: number; rot: number; style: { align: string } }): TextBox => ({
  x: t.x,
  y: t.y,
  w: t.w,
  h: t.h,
  rot: t.rot,
  align: alignOf(t.style.align),
})

export interface EditorFrame {
  x: number // опорная точка, мм разворота
  y: number
  w: number | null // закреплённая ширина; null — по содержимому (max-content)
  maxW: number // предел при «по тексту», мм
  shift: number // доля ширины, на которую рамка сдвигается влево от опорной точки (0, 0.5, 1)
  rot: number
}

// Рамка редактора (design §6.4). Закреплённая ширина — от угла P. «По тексту» — от якоря: браузер растит
// рамку от той же точки и в ту же сторону, что Go (anchor), поэтому после ответа Go блок не прыгает.
export function editorFrame(t: TextBox, auto: boolean, maxW: number): EditorFrame {
  if (!auto) return { x: t.x, y: t.y, w: t.w, maxW, shift: 0, rot: t.rot }
  const a = anchor(t)
  return { x: a.x, y: a.y, w: null, maxW, shift: ALIGN_X[t.align ?? 'left'], rot: t.rot }
}

// Угол блока при повороте ручкой. grab — направление курсора в момент захвата (cursorDeg), rot0 — угол блока тогда:
// их разность запоминается, чтобы блок не прыгал под курсор, если ручку взяли не ровно по оси.
export function dragAngle(c: Pt, p: Pt, grab: number, rot0: number, shift: boolean): number {
  const v = rotate(-(grab - rot0), p.x - c.x, p.y - c.y)
  return angleAt(c, { x: c.x + v.x, y: c.y + v.y }, shift)
}
