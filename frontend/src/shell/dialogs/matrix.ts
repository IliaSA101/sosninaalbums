// Подписи матрицы функций (спека 011, design §5.3): подсказки галочек и строка статуса «Включено вместе».
import type { Base, RowId } from '../../base/prefs/levels'
import { dependentsOf, locked, needsOf, shortOf } from '../../base/prefs/levelcustom'

const names = (ids: readonly RowId[]) => ids.map(shortOf).join(', ')

// Подсказка на галочке колонки «Свой».
export function rowTip(id: RowId): string {
  if (locked(id)) return 'Без этого программа не работает'
  const parts: string[] = []
  const needs = needsOf(id)
  const deps = dependentsOf(id)
  if (needs.length) parts.push(`Нужна: ${names(needs)}`)
  if (deps.length) parts.push(`От неё зависят: ${names(deps)}`)
  return parts.join('. ')
}

// Основа в дательном падеже: «Набор сброшен к «Полному»».
const DATIVE: Record<Base, string> = { simple: 'Простому', standard: 'Стандартному', full: 'Полному' }

export const resetText = (b: Base): string => `Набор сброшен к «${DATIVE[b]}»`

// Строка статуса после галочки: что изменилось вместе с ней (сама строка не в счёт). Пусто — строки нет.
export function togetherText(row: RowId, added: readonly RowId[], removed: readonly RowId[]): string {
  const more = (ids: readonly RowId[]) => ids.filter((id) => id !== row)
  const a = more(added)
  if (a.length) return `Включено вместе: ${names(a)}`
  const r = more(removed)
  if (r.length) return `Выключено вместе: ${names(r)}`
  return ''
}
