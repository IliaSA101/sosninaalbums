// Чистые расчёты ленты альбома (Timeline): размеры миниатюр, групп, подсказки.
import type { PlaceView, Rect, SpreadView } from '../../base/core/types'
import { unitOf, type Unit } from '../../base/geom/sheet'
import { RESET_WARN } from '../gallery/gridgallery'

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
  ask?: string // ручной в паре: «У разворота 2 ручная раскладка» — действие через вопрос (103)
}

// Виньетку не склеить и не разрезать: сначала «Обычный разворот» (103 §4.6).
export const VIG_WHY = 'Виньетку не склеить — сначала «Обычный разворот» (правая кнопка)'
export const VIG_CUT_WHY = 'Виньетку не разрезать — сначала «Обычный разворот» (правая кнопка)'

const SPREAD_UNIT = unitOf(undefined)

// «У разворота 2 ручная раскладка» / «У разворотов 2 и 3 ручная раскладка»; nums — номера (с 1), u — слова.
export function manualText(nums: readonly number[], u: Unit = SPREAD_UNIT): string {
  return nums.length === 1 ? `У ${u.of} ${nums[0]} ручная раскладка` : `У ${u.plural[2]} ${nums.join(' и ')} ручная раскладка`
}

// Склейка группы i со следующей. Ручной в паре — не отказ, а действие через вопрос (ask, 103).
export function joinState(spreads: SpreadView[], i: number, max: number, u: Unit = SPREAD_UNIT): CanDo {
  const pair = [spreads[i], spreads[i + 1]]
  if (pair.some((s) => s?.kind === 'vignette')) return { ok: false, why: VIG_WHY }
  const cnt = mergeCount(spreads, i)
  if (cnt > max) return { ok: false, why: `Нельзя склеить: вместе ${cnt} фото, максимум ${max}` }
  const manual = pair.flatMap((s, k) => (s?.grid ? [i + 1 + k] : []))
  return manual.length > 0 ? { ok: true, why: '', ask: manualText(manual, u) } : { ok: true, why: '' }
}

// Разрез группы перед фото j: show — показывать ли пункт меню (перед первым фото резать нечего).
export function cutState(sp: SpreadView, j: number, u: Unit = SPREAD_UNIT): CanDo & { show: boolean } {
  const show = j > 0 && j < sp.photos.length
  if (sp.kind === 'vignette') return { show, ok: false, why: VIG_CUT_WHY }
  return sp.grid ? { show, ok: true, why: '', ask: `У ${u.of} ручная раскладка` } : { show, ok: true, why: '' }
}

// Способ мышью: Ctrl+щелчок, на Mac ⌘+щелчок (Ctrl+щелчок там — правая кнопка, 073).
export const modClick = (mac: boolean) => (mac ? '⌘+щелчок' : 'Ctrl+щелчок')

// Подсказка: можно — как сделать (у ручного — что будет с местами), нельзя — причина.
const askTip = (mac: boolean, verb: string, ask?: string) =>
  `${modClick(mac)} — ${verb}${ask ? `. ${ask}: места вернутся к «Авто», сначала спросим` : ''}`
export const joinTip = (st: CanDo, mac: boolean) => (st.ok ? askTip(mac, 'склеить', st.ask) : st.why)
export const cutTip = (mac: boolean, ask?: string) => askTip(mac, 'разрезать', ask)

// Вопрос о склейке и разрезе ручного разворота (103 §4.3): заголовок, текст и кнопка подтверждения.
// manual — номера ручных (с 1), n — номер группы (при склейке — левой), photo — номер фото (с 1) при разрезе.
export function glueText(kind: 'join' | 'cut', manual: readonly number[], n: number, u: Unit = SPREAD_UNIT, photo = 0) {
  const head = `${manualText(manual, u)}. ${RESET_WARN}`
  if (kind === 'join') return { title: `Склеить ${u.many}?`, ask: `${head} Склеить ${u.many} ${n} и ${n + 1}?`, ok: 'Склеить' }
  return { title: `Разрезать ${u.one}?`, ask: `${head} Разрезать ${u.one} ${n} перед фото ${photo}?`, ok: 'Разрезать' }
}

// Левый край подсказки: по центру якоря, но не за пределы окна.
export const tipLeft = (x: number, w: number, win: number) => Math.max(8, Math.min(x - w / 2, win - w - 8))

// Прокрутка, при которой группа окажется по центру ленты.
export const centerScroll = (offsetLeft: number, boxW: number, elW: number) => offsetLeft - (boxW - elW) / 2
