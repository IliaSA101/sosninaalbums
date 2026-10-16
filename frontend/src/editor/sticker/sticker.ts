// Стикеры на листе (048 Ф2): чистые решения для слоя, жестов и карточки. Рамку, пропорцию и зажим считает Go
// (SetStickerBox → ClampSticker/FitStickers), здесь — только то, что видно, пока кнопка не отпущена.
// Рамка стикера — как у места: угол до поворота (x, y, w, h) и поворот вокруг центра; мм листа.
import { arrowDelta } from '../../base/geom/grid'
import { clampF, round2 } from '../../base/geom/rotbox'
import { snapPlaceMove } from '../../base/geom/snap'
import { rotate } from '../../base/geom/textbox'
import type { Axis, SnapCtx } from '../../base/geom/gridedit'
import type { Rect, SpreadView, StickerView } from '../../base/core/types'
import { patternPx, patternSrc } from '../backdrop/pattern'

export interface StBox extends Rect {
  rot: number
}

// Нижний предел стороны, мм (Go: ClampSticker). Верхний — вдвое больше большей стороны листа.
export const MIN_SIDE = 3
export const maxSide = (sw: number, ph: number): number => 2 * Math.max(sw, ph)

export const boxOf = (st: Pick<StickerView, 'x' | 'y' | 'w' | 'h' | 'rot'>): StBox => ({ x: st.x, y: st.y, w: st.w, h: st.h, rot: st.rot })

export const sameBox = (a: StBox, b: StBox): boolean =>
  Math.abs(a.x - b.x) < 0.005 &&
  Math.abs(a.y - b.y) < 0.005 &&
  Math.abs(a.w - b.w) < 0.005 &&
  Math.abs(a.h - b.h) < 0.005 &&
  Math.abs(a.rot - b.rot) < 0.05

// Центр рамки.
export const centerOf = (b: StBox): { x: number; y: number } => ({ x: b.x + b.w / 2, y: b.y + b.h / 2 })

// «За край можно, центр — на листе» (спека 048, сц. 12): центр зажат в лист, сама рамка может вылезать.
export function centerIn(b: StBox, sw: number, ph: number): StBox {
  const c = centerOf(b)
  const x = clampF(c.x, 0, sw) - b.w / 2
  const y = clampF(c.y, 0, ph) - b.h / 2
  return { ...b, x, y }
}

// Описанный прямоугольник повёрнутой рамки (для прилипания и рамки выделения).
export function bounds(b: StBox): Rect {
  if (!b.rot) return { x: b.x, y: b.y, w: b.w, h: b.h }
  const c = centerOf(b)
  const hw = b.w / 2
  const hh = b.h / 2
  const pts = [rotate(b.rot, -hw, -hh), rotate(b.rot, hw, -hh), rotate(b.rot, hw, hh), rotate(b.rot, -hw, hh)]
  const xs = pts.map((p) => c.x + p.x)
  const ys = pts.map((p) => c.y + p.y)
  const x = Math.min(...xs)
  const y = Math.min(...ys)
  return { x, y, w: Math.max(...xs) - x, h: Math.max(...ys) - y }
}

// Хоть одна рамка (описанная, с поворотом) выходит за лист sw×ph — только тогда `.content` листа обрезает вылет.
export const stickerOverflow = (boxes: readonly StBox[], sw: number, ph: number): boolean =>
  boxes.some((b) => {
    const r = bounds(b)
    return r.x < -0.005 || r.y < -0.005 || r.x + r.w > sw + 0.005 || r.y + r.h > ph + 0.005
  })

export interface Moved {
  box: StBox
  guideX: number | null
  guideY: number | null
}

// Перенос на (dx, dy): описанный прямоугольник прилипает к краям и местам (snap = null — без привязки),
// axis — удержанная Shift ось (по другой прилипания нет); центр зажат в лист.
export function moveBox(base: StBox, dx: number, dy: number, snap: SnapCtx | null, axis: Axis, sw: number, ph: number): Moved {
  let mx = dx
  let my = dy
  let gx: number | null = null
  let gy: number | null = null
  if (snap) {
    const a = bounds(base)
    const raw = { ...a, x: a.x + dx, y: a.y + dy }
    const s = snapPlaceMove(raw, snap.others, snap.params, snap.thr)
    if (axis !== 'y') {
      mx += s.x - raw.x
      gx = s.guideX
    }
    if (axis !== 'x') {
      my += s.y - raw.y
      gy = s.guideY
    }
  }
  const box = centerIn({ ...base, x: base.x + mx, y: base.y + my }, sw, ph)
  // Зажим центра мог увести рамку от линии: направляющую тогда не рисуем.
  const moved = bounds(box)
  const was = bounds(base)
  return {
    box,
    guideX: gx !== null && Math.abs(moved.x - was.x - mx) < 0.01 ? gx : null,
    guideY: gy !== null && Math.abs(moved.y - was.y - my) < 0.01 ? gy : null,
  }
}

export type StCorner = 'nw' | 'ne' | 'se' | 'sw'

// Размер угловой ручкой в осях стикера: противоположный угол стоит (у повёрнутого — его повёрнутый угол).
// free — без замка (плашка и волна-лента, 049), иначе пропорция держится всегда: масштаб берётся по большему
// относительному сдвигу. Стороны — в пределах Go (MIN_SIDE…maxSide), центр — в листе.
export function resizeBox(base: StBox, corner: StCorner, dx: number, dy: number, free: boolean, sw: number, ph: number): StBox {
  const d = rotate(-base.rot, dx, dy)
  const kx = corner.includes('e') ? 1 : -1
  const ky = corner.includes('s') ? 1 : -1
  const hi = maxSide(sw, ph)
  let w = base.w + kx * d.x
  let h = base.h + ky * d.y
  if (free) {
    w = clampF(w, MIN_SIDE, hi)
    h = clampF(h, MIN_SIDE, hi)
  } else if (base.w > 0 && base.h > 0) {
    const sx = w / base.w
    const sy = h / base.h
    const s0 = Math.abs(sx - 1) >= Math.abs(sy - 1) ? sx : sy
    const s = clampF(s0, Math.max(MIN_SIDE / base.w, MIN_SIDE / base.h), Math.min(hi / base.w, hi / base.h))
    w = base.w * s
    h = base.h * s
  }
  w = round2(w)
  h = round2(h)
  const o = rotate(base.rot, (kx * (w - base.w)) / 2, (ky * (h - base.h)) / 2)
  const c = centerOf(base)
  return centerIn({ x: c.x + o.x - w / 2, y: c.y + o.y - h / 2, w, h, rot: base.rot }, sw, ph)
}

// Сдвиг стрелкой: 1 мм, с Shift — 10; центр зажат в лист. Не стрелка — рамка та же.
export function nudgeBox(base: StBox, key: string, shift: boolean, sw: number, ph: number): StBox {
  const { dx, dy } = arrowDelta(key, shift)
  return centerIn({ ...base, x: base.x + dx, y: base.y + dy }, sw, ph)
}

// ---------- картинка ----------

// Длинная сторона на экране → ширина запроса: шаг 256, не больше 4096 (как у узора).
export const stickerPx = (longSideCss: number, dpr: number): number => patternPx(longSideCss, dpr)

// Адрес процедурного стикера: url от Go всегда без px.
export const artSrc = (url: string, px: number): string => patternSrc(url, px)

// Сторона листа на экране (px) × доля длинной стороны стикера в листе.
export function longSidePx(st: Pick<StickerView, 'w' | 'h'>, sw: number, widthPx: number): number {
  return sw > 0 ? (Math.max(st.w, st.h) / sw) * widthPx : 0
}

// Цвета процедурного стикера для пикеров: столько, сколько пикеров у генератора.
export const inksOf = (st: Pick<StickerView, 'art' | 'inks'>): string[] => (st.art?.inks ?? []).slice(0, st.inks)

// Подпись в карточке: пустое имя — генератор из новой версии программы.
export const STICKER_NEWER = 'Нужна новая версия программы'
export const stickerName = (st: Pick<StickerView, 'name' | 'known'>): string => (st.known ? st.name || 'Стикер' : STICKER_NEWER)

// Нет файла у своего стикера: пустой путь в библиотеке (фото в missing).
export const isLost = (st: Pick<StickerView, 'photo'>, missing: readonly string[]): boolean => !!st.photo && missing.includes(st.photo)

// Свои стикеры текущего листа в порядке слоёв; нет поля — пусто.
export const stickersOf = (s: Pick<SpreadView, 'stickers'> | undefined): StickerView[] => s?.stickers ?? []

export const stickerById = (s: Pick<SpreadView, 'stickers'> | undefined, id: string): StickerView | undefined =>
  stickersOf(s).find((x) => x.id === id)

// Ключ «этот же стикер на том же листе»: новый SpreadView после ответа Go сессию ползунка не рвёт.
export const stKey = (spread: string, id: string): string => `${spread}|${id}`
