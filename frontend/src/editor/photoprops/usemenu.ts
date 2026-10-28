// Подменю вхождений фото (спека 106, 108): общее для меню ленты («Где стоит ▸») и меню холста («Где ещё стоит ▸»).
// Чистая логика: строки подменю по ответу Api.WhereUsed и вхождение по щелчку; загрузка — uselist.svelte.ts.
import type { MenuNode } from '../../base/ui/menulist'
import { lineText, useLines, type UseLine, type UseSrc } from '../../base/words/whereuse'

export const USES_MENU_MAX = 10 // строк в подменю; дальше серое «и ещё N»

// Id строки вхождения: arg — её номер среди показанных (UseLine берётся по нему, подпись не разбирается).
export const USE_ROW = 'use'

// Какое вхождение не показывать: то, по которому щёлкнули (на холсте это «ещё»), — лист и запись места.
export interface UseHere {
  spread: string
  pick: string
}

// Ответ без текущего места; пока ответа нет (null) — null. Порядок Go не трогаем.
export function elsewhere(lines: UseSrc[] | null, here?: UseHere): UseSrc[] | null {
  if (!lines || !here) return lines
  return lines.filter((l) => !(l.kind === 'place' && l.spread === here.spread && (l.pick ?? '') === here.pick))
}

// Строки подменю: пока ответа нет — «…», пустой ответ — пояснение empty, лишнее — «и ещё N».
export function useRows(lines: UseSrc[] | null, empty: string): MenuNode[] {
  if (lines === null) return [{ id: 'wait', label: '…', kind: 'note' }]
  if (lines.length === 0) return [{ id: 'none', label: empty, kind: 'note' }]
  const { shown, more } = useLines(lines, USES_MENU_MAX)
  const rows: MenuNode[] = shown.map((l, i) => ({ id: USE_ROW, arg: String(i), label: lineText(l) }))
  return more > 0 ? [...rows, { id: 'more', label: `и ещё ${more}`, kind: 'note' }] : rows
}

// Вхождение по пункту подменю (arg — номер); нет такого — null.
export function lineAt(lines: UseSrc[] | null, arg: string | undefined): UseLine | null {
  const i = Number(arg)
  const { shown } = useLines(lines ?? [], USES_MENU_MAX)
  return Number.isInteger(i) ? (shown[i] ?? null) : null
}
