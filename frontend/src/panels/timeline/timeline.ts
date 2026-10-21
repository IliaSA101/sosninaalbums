// Чистые расчёты ленты альбома (Timeline): размеры миниатюр, групп, подсказки.
import type { PlaceView, Rect, SpreadView } from '../../base/core/types'

export const H = 44 // высота миниатюры вместе с рамкой
export const B = 2 // белая рамка
export const GAP = 2 // между миниатюрами группы
export const PAD = 4 // отступ группы

// Ширина по аспекту: ячейки фото на развороте.
export const chipW = (a: number) => Math.round((H - 2 * B) * (a > 0 && Number.isFinite(a) ? a : 1)) + 2 * B
export const cellW = (r: Rect | undefined) => chipW(r ? r.w / r.h : 1)

// Точная ширина группы для content-visibility: прокрутка к далёкой группе попадает в неё.
// Пустые места сетки (спека 007) идут в группе после фото.
export const emptyPlaces = (sp: SpreadView): PlaceView[] => (sp.places ?? []).filter((p) => !p.photo)

// Разворот без фото и без мест (спека 013): в ленте вместо пустой группы — одна квадратная заглушка.
export const isBlank = (sp: SpreadView) => sp.photos.length === 0 && emptyPlaces(sp).length === 0

export const groupW = (sp: SpreadView) => {
  const n = sp.photos.length + emptyPlaces(sp).length
  return (
    sp.photos.reduce((w, _, j) => w + cellW(sp.rects[j]), 0) +
    emptyPlaces(sp).reduce((w, p) => w + cellW(p), 0) +
    (isBlank(sp) ? chipW(1) : 0) +
    GAP * Math.max(0, n - 1) +
    2 * PAD
  )
}

// Сколько фото будет в развороте, если склеить i-й со следующим.
export const mergeCount = (spreads: SpreadView[], i: number) => (spreads[i]?.photos.length ?? 0) + (spreads[i + 1]?.photos.length ?? 0)

// Можно ли действие и почему нет: одна причина у черты, у зоны и у пункта меню (094 §3).
export interface CanDo {
  ok: boolean
  why: string // пусто, когда можно
}

// Склейка и разрез у сетки закрыты: сначала «Вернуть авто» (спека 007).
export const GRID_WHY = 'Сначала «Вернуть авто»'

// Склейка группы i со следующей.
export function joinState(spreads: SpreadView[], i: number, max: number): CanDo {
  if (spreads[i]?.grid || spreads[i + 1]?.grid) return { ok: false, why: GRID_WHY }
  const cnt = mergeCount(spreads, i)
  if (cnt > max) return { ok: false, why: `Нельзя склеить: вместе ${cnt} фото, максимум ${max}` }
  return { ok: true, why: '' }
}

// Разрез группы перед фото j: show — показывать ли пункт меню (перед первым фото резать нечего).
export function cutState(sp: SpreadView, j: number): CanDo & { show: boolean } {
  const show = j > 0 && j < sp.photos.length
  return sp.grid ? { show, ok: false, why: GRID_WHY } : { show, ok: true, why: '' }
}

// Способ мышью: Ctrl+щелчок, на Mac ⌘+щелчок (Ctrl+щелчок там — правая кнопка, 073).
export const modClick = (mac: boolean) => (mac ? '⌘+щелчок' : 'Ctrl+щелчок')

// Подсказка черты: можно — как склеить, нельзя — причина.
export const joinTip = (st: CanDo, mac: boolean) => (st.ok ? `${modClick(mac)} — склеить` : st.why)
export const cutTip = (mac: boolean) => `${modClick(mac)} — разрезать`

// Левый край подсказки: по центру якоря, но не за пределы окна.
export const tipLeft = (x: number, w: number, win: number) => Math.max(8, Math.min(x - w / 2, win - w - 8))

// Прокрутка, при которой группа окажется по центру ленты.
export const centerScroll = (offsetLeft: number, boxW: number, elW: number) => offsetLeft - (boxW - elW) / 2
