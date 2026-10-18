// Сворачивание длинной группы ленты (спека 027 §7): больше LIMIT плиток — видны SHOWN и плитка «+N».
// Плитки — фото и пустые места сетки; без Svelte, под тесты.
import type { SpreadView } from '../../base/core/types'
import { B, GAP, H, PAD, cellW, chipW, emptyPlaces, isBlank } from './timeline'

// Лист виньетки (031) в ленте — одна миниатюра листа, а не плитки мест: 25–60 мест разворачивались
// в полосу квадратиков с «+N» шириной в полэкрана, лист в ней не узнавался.
export const asSheet = (sp: Pick<SpreadView, 'kind'>): boolean => sp.kind === 'vignette'

// Ширина миниатюры листа без рамки: высота H вместе с рамкой, пропорции листа из Go (w/h), как у плитки обложки.
export const sheetThumbW = (sp: Pick<SpreadView, 'w' | 'h'>): number =>
  sp.w > 0 && sp.h > 0 ? Math.round(((H - 2 * B) * sp.w) / sp.h) : H - 2 * B

export const LIMIT = 12 // группа до стольких плиток показывается целиком
export const SHOWN = 11 // дальше — столько плиток и «+N» (вместе тоже LIMIT)
export const HOLD_MS = 600 // сколько держать перетаскиваемое фото над «+N», чтобы группа раскрылась

export type Chips = {
  photos: number // сколько фото показано (с начала)
  places: number // сколько пустых мест показано (после фото)
  more: number // сколько плиток скрыто: «+N»
  fold: boolean // раскрытая длинная группа: в конце плитка «‹»
}

// expanded — id раскрытой группы (одна на ленту) или null.
export function visibleChips(sp: SpreadView, expanded: string | null): Chips {
  const ph = sp.photos.length
  const pl = emptyPlaces(sp).length
  if (ph + pl <= LIMIT) return { photos: ph, places: pl, more: 0, fold: false }
  if (expanded === sp.id) return { photos: ph, places: pl, more: 0, fold: true }
  const photos = Math.min(ph, SHOWN)
  return { photos, places: SHOWN - photos, more: ph + pl - SHOWN, fold: false }
}

// Ширина группы по видимым плиткам (для contain-intrinsic-size): плитки «+N» и «‹» — квадраты.
export function shownW(sp: SpreadView, expanded: string | null): number {
  if (asSheet(sp)) return sheetThumbW(sp) + 2 * B + 2 * PAD
  const c = visibleChips(sp, expanded)
  const places = emptyPlaces(sp)
  let w = 0
  for (let j = 0; j < c.photos; j++) w += cellW(sp.rects[j])
  for (let k = 0; k < c.places; k++) w += cellW(places[k])
  let n = c.photos + c.places
  if (c.more > 0 || c.fold || isBlank(sp)) {
    w += chipW(1)
    n++
  }
  return w + GAP * Math.max(0, n - 1) + 2 * PAD
}

// Раскрытая группа сворачивается, когда текущий разворот сменился (prev — прежний, cur — новый).
export const afterNav = (expanded: string | null, prev: string, cur: string): string | null => (cur === prev ? expanded : null)

// Ширина свёрнутой группы по объекту разворота: дельта вида (075) сохраняет неизменённые объекты, пересчёт — по сменившимся.
const widths = new WeakMap<SpreadView, number>()

// Подпись ширин всех групп ленты: меняется, только если сдвинулись центры групп (Timeline перемеряет их по ней,
// а не на каждую правку: чтение offsetLeft после правки DOM заставляет браузер раскладывать ленту немедленно).
export function widthsKey(spreads: readonly SpreadView[], expanded: string | null): string {
  let out = ''
  for (const sp of spreads) {
    let w = sp.id === expanded ? undefined : widths.get(sp)
    if (w === undefined) {
      w = shownW(sp, expanded)
      if (sp.id !== expanded) widths.set(sp, w)
    }
    out += w + ','
  }
  return out
}

// Фото спрятано под «+N» в свёрнутой группе — его выбор на холсте раскрывает группу.
export function isHidden(sp: SpreadView, photo: string, expanded: string | null): boolean {
  if (asSheet(sp)) return false // лист виньетки не сворачивается — раскрывать нечего
  const j = sp.photos.indexOf(photo)
  return j >= 0 && j >= visibleChips(sp, expanded).photos
}
