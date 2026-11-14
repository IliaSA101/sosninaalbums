// Полоска отметок в панели ленты (спека 004, п. 7): что у выделенных общее и цвета кружков.
// Чистая логика без Svelte.
import type { Flag, Label, Photo } from '../../base/core/types'
import { markOf, type OwnMarks } from './marks'

// Общее значение действующей отметки выделенных: null — у фото по-разному (или никого нет).
export interface Shared {
  rating: number | null
  label: Label | 'other' | null
  flag: Flag | null
}

export function sharedMark(photos: readonly Photo[], own: OwnMarks): Shared {
  const s: Shared = { rating: null, label: null, flag: null }
  photos.forEach((p, i) => {
    const m = markOf(p, own[p.id])
    if (i === 0) {
      s.rating = m.rating
      s.label = m.label
      s.flag = m.flag
      return
    }
    if (s.rating !== m.rating) s.rating = null
    if (s.label !== m.label) s.label = null
    if (s.flag !== m.flag) s.flag = null
  })
  return s
}

// Метки для кружков и подсказок. Цвета — токены --lbl-* в style.css (там же миниатюра).
export const LABEL_VIEW: readonly { id: Exclude<Label, ''>; name: string }[] = [
  { id: 'red', name: 'красная' },
  { id: 'yellow', name: 'жёлтая' },
  { id: 'green', name: 'зелёная' },
  { id: 'blue', name: 'синяя' },
  { id: 'purple', name: 'фиолетовая' },
]

// Клик по звезде n: по текущей оценке всех выделенных — снять (0), иначе поставить n.
export const ratingClick = (n: number, shared: number | null): number => (shared === n ? 0 : n)

// Клик по флажку: уже стоит у всех — снять.
export const flagClick = (f: 1 | -1, shared: Flag | null): Flag => (shared === f ? 0 : f)

// Сколько звёзд горит: под курсором — до него (как в Lightroom), без курсора — общая оценка выделенных.
export const starsLit = (hover: number | null, shared: number | null): number => hover ?? shared ?? 0

// Подпись кнопки фильтра: область ленты и счёт; при активном фильтре отметок — «Показано 12 из 666»
// (для «Не в альбоме» — «Не в альбоме: 5 из 120»: область важна, «Все: 12 из 666» читается странно).
export function filterLabel(scope: 'all' | 'unused', base: number, shown: number, active: boolean): string {
  if (scope === 'unused') return active ? `Не в альбоме: ${shown} из ${base}` : `Не в альбоме ${base}`
  return active ? `Показано ${shown} из ${base}` : `Все ${base}`
}

// Есть ли у кого-то из выделенных свои отметки (иначе «Как в Lightroom» нечего возвращать).
export const hasOwn = (ids: readonly string[], own: OwnMarks): boolean => ids.some((id) => Object.hasOwn(own, id))
