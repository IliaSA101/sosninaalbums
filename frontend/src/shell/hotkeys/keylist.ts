// Список команд окна «Горячие клавиши» (074, design §8): поиск, фильтр «Только свои», группы. Чистая логика.
import { codeOfChord } from '../../base/keys/chord'
import { chordText, codeText } from '../../base/keys/chordtext'
import type { KeymapEntry, KeymapView, KeyScope } from '../../base/types/keys'
import { RU } from './board'
import { SCOPE_TAG } from './keywords'

// Без регистра, ё = е (как в справочнике F1).
export const norm = (s: string): string => s.toLowerCase().replaceAll('ё', 'е')

// Запрос из одной буквы — это клавиша, а не слово: «в» находит команды на клавише с русской «В» (латинская D),
// «d» — на клавише D, «0» — на нуле. Иначе одна буква нашла бы почти любую команду по названию.
function byKeyName(e: KeymapEntry, q: string): boolean {
  return e.keys.some((k) => {
    const code = codeOfChord(k)
    return /[а-я]/.test(q) ? norm(RU[code] ?? '') === q : codeText(code).toLowerCase() === q
  })
}

// Название, пояснение или текст клавиши: «ctrl+d», «ctrl d», «del».
export function matches(e: KeymapEntry, query: string, mac = false): boolean {
  const q = norm(query.trim())
  if (!q) return true
  if ([...q].length === 1) return byKeyName(e, q)
  if (norm(e.label).includes(q) || norm(e.hint ?? '').includes(q)) return true
  return e.keys.some((k) =>
    [chordText(k, false), chordText(k, mac)].some((t) => {
      const s = t.toLowerCase()
      return s.includes(q) || s.replaceAll('+', ' ').includes(q)
    }),
  )
}

export interface ListOpts {
  query: string
  onlyOwn: boolean
  visible: (e: KeymapEntry) => boolean // команда есть на уровне интерфейса
  mac?: boolean
}

// Команды для списка по порядку каталога: уровень, поиск, «Только свои».
export const listed = (view: KeymapView, o: ListOpts): KeymapEntry[] =>
  view.commands.filter((e) => o.visible(e) && matches(e, o.query, o.mac) && (!o.onlyOwn || e.custom))

export interface ListGroup {
  id: string
  title: string
  scopeTag: string // одна область на всю группу — подпись в заголовке, а не в каждой строке; '' — разная или «везде»
  rows: KeymapEntry[]
}

export function groupRows(view: KeymapView, rows: KeymapEntry[]): ListGroup[] {
  const out: ListGroup[] = []
  for (const g of view.groups) {
    const own = rows.filter((e) => e.group === g.id)
    if (own.length === 0) continue
    const scopes = new Set<KeyScope>(own.map((e) => e.scope))
    const only = scopes.size === 1 ? [...scopes][0] : null
    out.push({ id: g.id, title: g.title, scopeTag: only ? SCOPE_TAG[only] : '', rows: own })
  }
  return out
}

export const emptyText = (o: Pick<ListOpts, 'query' | 'onlyOwn'>): string =>
  o.onlyOwn && !o.query.trim()
    ? 'Своих сочетаний пока нет.'
    : 'Ничего не нашлось. Попробуйте другое слово, название клавиши или русскую букву.'

// Метка строки: подпись области нужна, если область не «везде» и группа не держит её в заголовке.
export const rowTag = (e: KeymapEntry, g: ListGroup): string => (g.scopeTag ? '' : SCOPE_TAG[e.scope])
