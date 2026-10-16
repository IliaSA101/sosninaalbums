// Окно «Автосборка» (027 U2): чистые подписи и зажимы. Состояние запросов — autobuild.svelte.ts.
import { plural } from '../../base/words/plural'
import type { AutoPlan, AutoRange, Variety } from '../../base/core/types'
import type { AutoMem } from '../../base/prefs/view.svelte'
import { unitOf, type Unit } from '../../base/geom/sheet'

export const MAX_PER = 64 // «до» на разворот/лист: предел Go (MaxPhotos)

export const clampInt = (v: number, lo: number, hi: number) => Math.max(lo, Math.min(hi < lo ? lo : hi, Math.round(v)))

const SPREAD = unitOf(undefined)

// «Разнообразие раскладок» (041): три положения ручки и пояснение под ней по выбранному.
export const VARIETY_OPTIONS: readonly { v: Variety; label: string; hint: string }[] = [
  { v: 0, label: 'Меньше', hint: 'На каждом развороте самая плотная раскладка' },
  { v: 1, label: 'Обычное', hint: 'Соседние развороты не повторяются, иногда — воздух и крупное фото' },
  { v: 2, label: 'Больше', hint: 'Чаще ленты, сетки и воздух, соседние развороты разной формы' },
]

// Мусор в хранилище и пустое значение дают «Обычное».
export const normVariety = (v: unknown): Variety => (v === 0 || v === 1 || v === 2 ? v : 1)

// «3 разворота (6 стр.)»; у листов виньетки (031) страниц нет: «3 листа».
export function spreadsLabel(n: number, unit: Unit = SPREAD): string {
  const base = `${n} ${plural(n, unit.plural)}`
  return unit === SPREAD ? `${base} (${2 * n} стр.)` : base
}

// Сколько фото на разворот, если N не запомнен: как прежнее «по 4».
export function defaultSpreads(photos: number): number {
  return Math.max(1, Math.round(photos / 4))
}

// N в пределы ползунка. Пока предела нет (нет фото), N остаётся как был.
export function clampSpreads(n: number, plan: Pick<AutoPlan, 'minSpreads' | 'maxSpreads'> | null): number {
  if (!plan || plan.maxSpreads < 1) return n
  return clampInt(n, plan.minSpreads, plan.maxSpreads)
}

// Поля «от» и «до»: мусор — 64, иначе 1…64.
export function clampMaxPer(v: number): number {
  return Number.isFinite(v) ? clampInt(v, 1, MAX_PER) : MAX_PER
}

// Подсказка под строкой «Фото на разворот» (051).
export const RANGE_HINT = 'Шире — альбом живее, уже — ровнее'

// Диапазон «от … до …» → память окна и обратно; «Авто» поля не хранит.
export const memOf = (r: AutoRange): AutoMem => (r.auto ? 'auto' : [r.minPer, r.maxPer])

// «от» > «до»: подтягивается второе поле, тронутое последним не меняется.
export function settleRange(from: number, to: number, touched: 'min' | 'max'): [number, number] {
  if (from <= to) return [from, to]
  return touched === 'min' ? [from, from] : [to, to]
}

// «серий: 9» рядом с флажком: границ серий cuts → серий на одну больше; нет границ — строки нет.
export const seriesText = (cuts: number): string => (cuts > 0 ? `серий: ${cuts + 1}` : '')

export type Anchor = 'spreads' | 'per'
// Куда ведёт кнопка ⇄: во второе поле — не то, что трогали последним (якорь). Фокус при щелчке уже ушёл с поля,
// поэтому смотрим на якорь, а не на активный элемент.
export const otherField = (anchor: Anchor): 'auto-n' | 'auto-per' => (anchor === 'spreads' ? 'auto-per' : 'auto-n')
// Поле якоря: туда Modal ставит первый фокус, чтобы он не переключал якорь на другое поле.
export const anchorField = (anchor: Anchor): 'auto-n' | 'auto-per' => (anchor === 'spreads' ? 'auto-n' : 'auto-per')

export interface Lim {
  min: number
  max: number
}

// Среднее в поле — целое round(n / N): дробь «9,1» в поле выглядит странно, дробь нигде не показываем.
export const perText = (n: number, spreads: number): string => (spreads > 0 ? String(Math.round(n / spreads)) : '')

// X («фото на разворот») → N = round(n / X) в пределах ползунка; не число или ≤ 0 — null (N не трогаем).
export function spreadsForPer(n: number, x: number, lim: Lim): number | null {
  if (!Number.isFinite(x) || x <= 0) return null
  return clampInt(n / x, lim.min, lim.max)
}

// Шаг ↑↓ в поле среднего: показанное целое round(n / N) меняется ровно на 1 (если N±1 даёт то же — идём дальше),
// N меняется хотя бы на 1, в пределах ползунка. dir = +1 — среднее растёт, N падает.
export function perStep(n: number, spreads: number, dir: 1 | -1, lim: Lim): number {
  const shown = Math.round(n / spreads)
  const end = dir > 0 ? Math.max(1, lim.min) : lim.max
  for (let k = spreads + (dir > 0 ? -1 : 1); dir > 0 ? k >= end : k <= end; k -= dir) {
    if (Math.round(n / k) !== shown) return clampInt(k, lim.min, lim.max)
  }
  return clampInt(end, lim.min, lim.max)
}

// «от 3 до 6»; ровные группы — «по 4».
export const rangeText = (p: Pick<AutoPlan, 'min' | 'max'>): string => (p.min === p.max ? `по ${p.min}` : `от ${p.min} до ${p.max}`)

// Первое N при открытии окна: якорь «среднее» и X задан — из X; иначе запомненное N; нет ничего — round(n / 4).
export function firstSpreads(n: number, saved: { spreads: number | null; per?: number | null; by?: Anchor }, lim: Lim): number {
  if (saved.by === 'per' && saved.per != null) {
    const v = spreadsForPer(n, saved.per, lim)
    if (v !== null) return v
  }
  return clampInt(saved.spreads ?? defaultSpreads(n), lim.min, lim.max)
}

// Откуда фото (034, решение 92). count — сколько фото ушло в запрос, photos — сколько насчитал Go (null — ответа ещё нет).
export interface Source {
  from: 'selected' | 'free'
  count: number
  inAlbum: number
  skipped: number
  filtered: boolean
}
export function sourceLine(d: Source, photos: number | null): string {
  let s: string
  if (d.from === 'selected') {
    s = `Выделенные фото: ${d.count}`
    if (d.inAlbum > 0) s += `, из них ${d.inAlbum} уже в альбоме — переедут в новые развороты`
  } else {
    s = `Все свободные фото ленты: ${d.count}`
    if (d.filtered) s += ' (фильтр ленты)'
    if (d.skipped > 0) s += `, без ${d.skipped} ${plural(d.skipped, ['отклонённого', 'отклонённых', 'отклонённых'])}`
  }
  if (photos !== null && photos > 0 && photos < d.count) s += `; ${d.count - photos} из сеток и с фонов не беру`
  return s
}

// Слово при числе: «1 разворот», «2 разворота», «5 разворотов» — подпись поля N (U3).
export const unitWord = (n: number, unit: Unit = SPREAD): string => plural(n, unit.plural)

// «В альбоме станет 33 разворота (66 стр.)»; пустой альбом — строки нет. reuse (U1) — сколько пустых
// разворотов в конце займёт сборка: они уже в have, новые встанут на их место. Хвост считает Go (AutoPlan.reuse).
export function albumAfter(have: number, spreads: number, unit: Unit = SPREAD, reuse = 0): string {
  const rest = have - reuse
  if (rest <= 0) return ''
  const s = `В альбоме станет ${spreadsLabel(rest + spreads, unit)}`
  return reuse > 0 ? `${s} — пустые в конце (${reuse}) займёт сборка` : s
}

// Подсказка под ползунком: «от 3 до 6 фото на разворот»; ровные группы — «по 4 фото на разворот». Дробного среднего
// нигде не показываем (решение владельца): в поле — целое round(n / N).
export function planHint(p: AutoPlan, unit: Unit = SPREAD): string {
  if (p.photos === 0) return 'Нет фото для сборки'
  return `${rangeText(p)} фото на ${unit.one}`
}
