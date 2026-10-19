// Форма места (спека 044, design §8): круг/овал и скруглённые углы. Радиус и кадр считает Go (мм из View);
// здесь только перевод в CSS, попадание точки в форму (запасной хит-тест, решение №42) и предпросмотр ползунка.
import { rotate, type Pt } from './textbox'

export const SHAPE_OVAL = 'oval'
export const MAX_ROUND = 50 // % меньшей стороны

// Форма для расчётов кадра: зеркало geom.Outline в Go (теги JSON те же, паритет frames.json).
export interface Outline {
  oval?: boolean
  radius?: number // мм; у овала не используется
}

export const isOval = (shape?: string): boolean => shape === SHAPE_OVAL

// Вид формы места (076): прямоугольное · скруглённое · круглое. Один ключ на меню «Место ▾», карточку, меню правой
// кнопки и виньетку; процент скругления решает Go (10 % у «Скруглённого»), здесь его нет.
export type ShapeKey = 'rect' | 'round' | 'oval'

// Форма по виду листа или места: круг, любое скругление (в том числе из своего шаблона) или прямоугольник.
export function shapeKey(f: { shape?: string; round?: number }): ShapeKey {
  if (isOval(f.shape)) return 'oval'
  return (f.round ?? 0) > 0 ? 'round' : 'rect'
}

// Аргумент `shape` у AddPlace и SetPlaceShape: '' — прямые углы, 'rounded' — скруглённое, 'oval' — круг.
export const shapeArg = (k: ShapeKey): '' | 'rounded' | 'oval' => (k === 'rect' ? '' : k === 'round' ? 'rounded' : 'oval')

// Форма места из полей View: нет формы и радиуса — undefined (прямоугольник, прежние формулы).
export function outlineOf(shape?: string, radius?: number): Outline | undefined {
  if (isOval(shape)) return { oval: true }
  return radius && radius > 0 ? { radius } : undefined
}

// Радиус в пределах рамки w×h: не больше половины меньшей стороны (как Outline.radius в Go).
const radiusIn = (radius: number, w: number, h: number): number => (radius > 0 ? Math.min(radius, Math.min(w, h) / 2) : 0)

// border-radius места: овал — 50 %; скругление ρ мм — проценты от своих сторон (дают круглые углы).
// Прямоугольник — undefined (стиль не ставится).
export function shapeCss(shape: string | undefined, radius: number | undefined, w: number, h: number): string | undefined {
  if (isOval(shape)) return '50%'
  const r = radiusIn(radius ?? 0, w, h)
  if (!(r > 0) || !(w > 0) || !(h > 0)) return undefined
  return `${(r / w) * 100}% / ${(r / h) * 100}%`
}

// Радиус (мм) для предпросмотра ползунка «Скругление, %»: round % меньшей стороны. В проект не уходит.
export function liveRadius(round: number, w: number, h: number): number {
  if (!Number.isFinite(round)) return 0
  return (Math.min(MAX_ROUND, Math.max(0, round)) / 100) * Math.min(w, h)
}

// Отступ (мм) от угла рамки до угла вписанного в форму прямоугольника: овал — (1 − 1/√2)/2 стороны (14,6 %),
// скругление — ρ(1 − 1/√2). Значки на ячейке (цветовой профиль, «!» dpi) стоят там, чтобы форма их не обрезала.
export function cornerInset(shape: string | undefined, radius: number | undefined, w: number, h: number): Pt {
  const k = 1 - Math.SQRT1_2
  if (isOval(shape)) return { x: (w * k) / 2, y: (h * k) / 2 }
  const r = radiusIn(radius ?? 0, w, h) * k
  return { x: r, y: r }
}

// «!» dpi у места с формой: в левом верхнем углу вписанного прямоугольника, всегда прямо (обратный поворот
// на rot вокруг самого значка). off — отступ на экране, px. Без формы — undefined (прежнее положение).
export function markAt(
  r: { w: number; h: number },
  rot: number,
  shape: string | undefined,
  radius: number | undefined,
  off: number,
): { left: string; top: string; rot: number } | undefined {
  if (!isOval(shape) && !(radiusIn(radius ?? 0, r.w, r.h) > 0)) return undefined
  const k = cornerInset(shape, radius, r.w, r.h)
  const o = rotate(-rot, off, off) // отступ off задан в осях экрана, значок стоит в осях места
  const pc = (v: number, of: number) => (of > 0 ? (v / of) * 100 : 0)
  return { left: `calc(${pc(k.x, r.w)}% + ${o.x}px)`, top: `calc(${pc(k.y, r.h)}% + ${o.y}px)`, rot: -rot }
}

// Точка (от левого верхнего угла рамки w×h, мм) внутри формы. Прямоугольник — внутри рамки.
export function inShape(p: Pt, w: number, h: number, shape: string | undefined, radius?: number): boolean {
  const eps = 1e-6
  if (p.x < -eps || p.y < -eps || p.x > w + eps || p.y > h + eps) return false
  const x = Math.abs(p.x - w / 2)
  const y = Math.abs(p.y - h / 2)
  if (isOval(shape)) {
    const a = w / 2 + eps
    const b = h / 2 + eps
    return (x * x) / (a * a) + (y * y) / (b * b) <= 1
  }
  const r = radiusIn(radius ?? 0, w, h)
  const dx = x - (w / 2 - r)
  const dy = y - (h / 2 - r)
  return dx <= 0 || dy <= 0 || dx * dx + dy * dy <= (r + eps) * (r + eps)
}
