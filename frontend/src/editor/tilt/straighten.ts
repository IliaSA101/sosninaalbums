// Режим «Выпрямить по линии» (спека 045, design §6): чистые решения без Svelte. Угол считает Go —
// здесь только перевод пикселей экрана в мм листа, порог линии и «жив ли режим».
import { backdropAt, inBox } from '../backdrop/bgmode'
import { backdropOf } from '../backdrop/backdrop'
import { rotate, type Pt, type TextBox } from '../../base/geom/textbox'
import { stackAt } from '../canvas/pickunder'
import type { BackdropScope, Rect, SpreadView } from '../../base/core/types'

// Линия короче — случайный щелчок или дрожь руки: ничего не шлём, режим остаётся (спека 045 А.6).
export const MIN_LINE_PX = 12

export type StraightenTarget = { kind: 'photo'; spread: string; photo: string } | { kind: 'backdrop'; spread: string; scope: BackdropScope }

export const isLine = (a: Pt, b: Pt): boolean => Math.hypot(b.x - a.x, b.y - a.y) >= MIN_LINE_PX

// Линия в мм листа: k — мм на пиксель экрана. Знак и порядок точек Go не важны.
export const lineMM = (a: Pt, b: Pt, k: number): { dx: number; dy: number } => ({ dx: (b.x - a.x) * k, dy: (b.y - a.y) * k })

// Режим жив, пока цель на месте: тот же разворот и то же выделенное фото (фон — тот же режим фона и область).
export function keepStraighten(
  cur: StraightenTarget | null,
  spread: SpreadView | null | undefined,
  selected: string | null,
  bg: { spread: string; scope: BackdropScope } | null,
): boolean {
  if (!cur || !spread || spread.id !== cur.spread) return false
  if (cur.kind === 'photo') return selected === cur.photo && spread.photos.includes(cur.photo)
  return bg?.spread === cur.spread && bg.scope === cur.scope && !!backdropOf(spread, cur.scope)
}

// Рамка места как TextBox (угол — левый верхний, поворот вокруг него): место повёрнуто вокруг своего центра,
// поэтому угол переезжает — иначе inBox не применить.
export function placeBox(spread: SpreadView, photo: string): TextBox | null {
  const i = spread.photos.indexOf(photo)
  const r = spread.rects[i]
  if (i < 0 || !r) return null
  const rot = spread.frames?.[i]?.cellRot ?? 0
  const o = rotate(rot, -r.w / 2, -r.h / 2)
  return { x: r.x + r.w / 2 + o.x, y: r.y + r.h / 2 + o.y, w: r.w, h: r.h, rot }
}

// Точка p (мм листа) внутри видимой рамки цели: для фона — ещё и внутри области, она обрезает фото.
export function hitTarget(spread: SpreadView, t: StraightenTarget, p: Pt): boolean {
  if (t.kind === 'backdrop') return backdropAt(spread.backdrops, p) === t.scope
  const b = placeBox(spread, t.photo)
  return !!b && inBox(b, p)
}

// Верхнее фото под точкой p (мм листа), кроме except: его выделит щелчок мимо цели. Тексты и подписи не берём.
export function photoUnder(spread: SpreadView, p: Pt, except?: string): string | null {
  return stackAt(spread, p, spread.w, spread.h, (k) => k === 'photo').find((it) => it.ref !== except)?.ref ?? null
}

// Рамка цели для подсветки: угол и поворот вокруг угла (как у фона); area — область, что обрезает фото фона.
export function targetFrame(spread: SpreadView, t: StraightenTarget): { box: TextBox; area?: Rect } | null {
  if (t.kind === 'photo') {
    const box = placeBox(spread, t.photo)
    return box && { box }
  }
  const b = backdropOf(spread, t.scope)
  return b ? { box: { ...b.box }, area: b.area } : null
}
