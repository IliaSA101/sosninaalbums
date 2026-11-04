// Строки «где стоит фото» (106): общие для карточки «О снимке», окна «Фото уже в альбоме» и меню ленты.
// Подпись и вид вхождения готовит Go (PhotoUse.kind/page/what); подпись по « — » не разбираем.
import type { PhotoUse, UseKind } from '../core/types'

// Вхождение, как его надо показать и открыть. sheet — индекс в ряду холста (−1 — фон альбома).
export interface UseLine {
  kind: UseKind
  spread: string
  sheet: number
  page: string // текст ссылки: «Разворот 3»
  what: string // после ссылки обычным текстом: «место»; '' — нечего добавлять
  pick: string // что выделить при переходе; '' — ничего
}

// Что берём из ответа Go: label не нужен; kind приходит строкой (типы wailsjs).
export type UseSrc = Pick<PhotoUse, 'sheet' | 'spread' | 'page' | 'what' | 'pick'> & { kind: string }

export const lineOf = (u: UseSrc): UseLine => ({
  kind: u.kind as UseKind,
  spread: u.spread,
  sheet: u.sheet,
  page: u.page,
  what: u.what,
  pick: u.pick ?? '',
})

// Целиком одной строкой: «Разворот 3 — место», «Фон альбома».
export const lineText = (l: Pick<UseLine, 'page' | 'what'>): string => (l.what ? `${l.page} — ${l.what}` : l.page)

// Первые max вхождений и сколько не показано. Порядок — как отдал Go, не сортируем.
export function useLines(uses: readonly UseSrc[], max: number): { shown: UseLine[]; more: number } {
  return { shown: uses.slice(0, max).map(lineOf), more: Math.max(0, uses.length - max) }
}
