// Выделение рамкой (спека 009, gestures.md «Выделение рамкой»). Чистая логика под тесты: рамка в px
// экрана → мм разворота, какие объекты она касается, как они сливаются с выделением. Сама протяжка —
// Marquee.svelte. Фон рамкой не выделяется (его правят в режиме фона).
import { aabb } from '../../base/geom/textbox'
import { normalize, type GroupSel } from '../places/groupsel'
import { aabbOf } from '../../base/geom/rotbox'
import type { Rect, SpreadView } from '../../base/core/types'

export interface Pt {
  x: number
  y: number
}

// Меньше этого (px) протяжка — щелчок.
export const MARQUEE_MIN = 3

// Рамка по двум углам, любой порядок.
export function rectOf(a: Pt, b: Pt): Rect {
  return { x: Math.min(a.x, b.x), y: Math.min(a.y, b.y), w: Math.abs(a.x - b.x), h: Math.abs(a.y - b.y) }
}

// Рамка в px экрана → мм разворота: box — экранная рамка разворота (ширина = sw мм), точка вне
// разворота (серое поле) даёт отрицательные или большие мм — пересечение это учитывает.
export function toSpreadMM(r: Rect, box: { left: number; top: number; width: number }, sw: number): Rect {
  const k = box.width > 0 ? sw / box.width : 0
  return { x: (r.x - box.left) * k, y: (r.y - box.top) * k, w: r.w * k, h: r.h * k }
}

// Рамки пересекаются (касание краем не считается).
export const touches = (a: Rect, b: Rect) => a.x < b.x + b.w && b.x < a.x + a.w && a.y < b.y + b.h && b.y < a.y + a.h

export interface Hit {
  refs: string[] // фото и места (ref, как в placesel)
  texts: string[]
}

// Что коснулась рамка r (мм). «Ручная» — места (занятое по фото, пустое по ID, порядок мест), «Авто» —
// ячейки фото; тексты — по описанному прямоугольнику (у повёрнутого). Подписи не входят.
export function marqueeHits(s: SpreadView | undefined, r: Rect): Hit {
  if (!s) return { refs: [], texts: [] }
  const refs = s.grid
    ? (s.places ?? []).filter((p) => touches(r, aabbOf(p))).map((p) => p.photo || p.id)
    : s.photos.filter((_, i) => s.rects[i] && touches(r, s.rects[i]))
  const texts = s.texts.filter((t) => touches(r, aabb(t))).map((t) => t.id)
  return { refs, texts }
}

// Итог выделения: Shift/Ctrl добавляют найденное к текущему, иначе заменяют его.
export function marqueePick(cur: GroupSel, hit: Hit, additive: boolean): GroupSel {
  if (!additive) return normalize({ refs: hit.refs, texts: hit.texts, text: null })
  const texts = cur.text ? [cur.text, ...cur.texts] : cur.texts
  return normalize({
    refs: [...cur.refs, ...hit.refs.filter((x) => !cur.refs.includes(x))],
    texts: [...texts, ...hit.texts.filter((x) => !texts.includes(x))],
    text: null,
  })
}
