// Список окна справочника F1 (спека 136, design §1): разделы, открытые разделы, шаг стрелкой, счётчик. Чистая логика.
import { plural } from '../../base/words/plural'
import { ARTICLES, GUIDE_GROUPS, type Article, type GroupId } from '../guidetext/articles'
import { inOrder } from '../guidetext/order'
import { stepRow, type NavKey } from '../hotkeys/keylist'
import { functionsText } from './inventory'

export interface GuideSection {
  id: GroupId
  title: string
  items: Article[]
}

// Разделы в порядке GUIDE_GROUPS, статьи внутри — по GUIDE_ORDER; пустые разделы не показываем.
export function sectionsOf(articles: readonly Article[] = ARTICLES): GuideSection[] {
  const out: GuideSection[] = []
  for (const g of GUIDE_GROUPS) {
    const items = inOrder(
      articles.filter((a) => a.group === g.id),
      g.id,
    )
    if (items.length) out.push({ id: g.id, title: g.title, items })
  }
  return out
}

// Открытые разделы: открытые руками плюс раздел выбранной статьи (он открыт всегда).
export function openGroups(user: ReadonlySet<GroupId>, selGroup: GroupId | undefined): Set<GroupId> {
  const out = new Set(user)
  if (selGroup) out.add(selGroup)
  return out
}

// Шаг ↑/↓/Home/End по всем статьям подряд, свёрнутость разделов не учитывается: раздел цели станет разделом выбора
// и откроется сам. Те же правила краёв, что у списка «Горячих клавиш».
export const stepSel = (order: readonly string[], id: string | null, key: NavKey): string | null => stepRow(order, id, key)

// Статьи в порядке показа (для шагов стрелкой).
export const orderOf = (sections: readonly { items: readonly Article[] }[]): string[] => sections.flatMap((s) => s.items.map((a) => a.id))

export interface CountOpts {
  shown: number // сколько статей в списке
  total: number // сколько всего
  searching: boolean // есть запрос
  filtered: boolean // список ограничен строкой таблицы уровней
  functions?: number // число функций программы (опись); нет — в шапке только статьи
}

// Счётчик в шапке окна: «115 статей · 170 функций» / «Найдено 7 из 115» / «9 из 115».
export function countLine(o: CountOpts): string {
  if (o.searching) return `Найдено ${o.shown} из ${o.total}`
  if (o.filtered) return `${o.shown} из ${o.total}`
  const articles = `${o.total} ${plural(o.total, ['статья', 'статьи', 'статей'])}`
  return o.functions === undefined ? articles : `${articles} · ${functionsText(o.functions)}`
}

// Отметки «Изучил» (спека 140, design §1). Чистые помощники; хранит их learned.svelte.ts.

// Список из config.json → набор id текущих статей: прежний id (RENAMED) засчитывается новому, неизвестные
// (новая версия, убранная статья) при показе отбрасываются, в файле остаются.
export function learnedOf(raw: readonly string[], renamed: Readonly<Record<string, string>>, known: ReadonlySet<string>): Set<string> {
  const out = new Set<string>()
  for (const id of raw) {
    const to = known.has(id) ? id : renamed[id]
    if (to && known.has(to)) out.add(to)
  }
  return out
}

// «Изучено n из m»: m — статьи, доступные на текущем уровне; изученные, но скрытые уровнем, в n не входят.
export function progress(
  articles: readonly Article[],
  learned: ReadonlySet<string>,
  can: (feature: Article['feature']) => boolean,
): { n: number; m: number } {
  const open = articles.filter((a) => can(a.feature))
  return { n: open.filter((a) => learned.has(a.id)).length, m: open.length }
}

// Число в заголовке раздела: «5 / 18», если в разделе есть изученные, иначе «18».
export function sectionMark(items: readonly { id: string }[], learned: ReadonlySet<string>): string {
  const k = items.filter((a) => learned.has(a.id)).length
  return k > 0 ? `${k} / ${items.length}` : String(items.length)
}
