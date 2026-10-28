// Отбор плиток окон оформления (133, ступень 2): источник (тема или выборка) → «Набор» → «Вид» → «Стиль».
// Вид и стиль размечает Go (DecorItem.form / style), здесь только отбор и числа слева. Чистое, под тест.
import type { SideItem } from '../../base/tpl/SideList.svelte'
import { sideItems, themeItems, type DecorCatalog, type DecorItem } from '../decor/decor'

export type FormSel = '' | 'sticker' | 'corner' | 'divider'
// Выбор в ряду отбора; '' — «Все». Помнится до конца сеанса (DecorUi.filters), варианта в списке может уже не быть — тогда в показе «Все».
export interface BrowseSel {
  pack: string // packId
  form: FormSel
  style: string
}
export const NO_SEL: Readonly<BrowseSel> = { pack: '', form: '', style: '' }

export interface Opt {
  id: string
  label: string
}

export type BrowseItem = Pick<DecorItem, 'id' | 'name' | 'pack' | 'packId' | 'form' | 'style'>

export const BUILTIN_PACK = 'sa:builtin'
export const BUILTIN_LABEL = 'Встроенные'
const FORM_LABELS: readonly Opt[] = [
  { id: 'sticker', label: 'Стикеры' },
  { id: 'corner', label: 'Уголки' },
  { id: 'divider', label: 'Разделители' },
]
const STYLE_MIN = 3 // стиль — от трёх предметов: чужие приставки «X: Y» ряд не раздувают
const STYLE_MAX = 6 // стилей больше — шесть самых крупных, остальные видны в «Все»

const formOf = (it: Pick<BrowseItem, 'form'>): string => it.form ?? 'sticker'

// Наборы списка в порядке каталога (order — весь каталог: в «Недавних» и «Новых» порядок не появления, а каталога),
// «Встроенные» последними. Меньше двух — выбирать не из чего. Одинаковые названия различаются id в скобках.
export function packOpts(list: readonly BrowseItem[], order: readonly BrowseItem[] = list): Opt[] {
  const rank = new Map<string, number>()
  order.forEach((it, i) => {
    if (it.packId && !rank.has(it.packId)) rank.set(it.packId, i)
  })
  const names = new Map<string, string>()
  for (const it of list) {
    if (it.packId && !names.has(it.packId)) names.set(it.packId, it.packId === BUILTIN_PACK ? BUILTIN_LABEL : it.pack || it.packId)
  }
  if (names.size < 2) return []
  const ids = [...names.keys()].sort(
    (a, b) => Number(a === BUILTIN_PACK) - Number(b === BUILTIN_PACK) || (rank.get(a) ?? Infinity) - (rank.get(b) ?? Infinity),
  )
  const dup = new Map<string, number>()
  names.forEach((n) => dup.set(n, (dup.get(n) ?? 0) + 1))
  return ids.map((id) => {
    const n = names.get(id) ?? id
    return { id, label: (dup.get(n) ?? 0) > 1 ? `${n} (${id})` : n }
  })
}

// Виды, что встретились: «Стикеры · Уголки · Разделители». Один — выбирать не из чего.
export function formOpts(list: readonly BrowseItem[]): Opt[] {
  const have = new Set(list.map(formOf))
  const found = FORM_LABELS.filter((f) => have.has(f.id))
  return found.length > 1 ? [...found] : []
}

// Стили от трёх предметов; больше шести — шесть самых многочисленных (ничья — порядок каталога), показ — в порядке каталога
// (order — весь каталог, как у packOpts: в «Недавних» кнопки не прыгают).
export function styleOpts(list: readonly BrowseItem[], order: readonly BrowseItem[] = list): Opt[] {
  const rank = new Map<string, number>()
  order.forEach((it, i) => {
    if (it.style && !it.form && !rank.has(it.style)) rank.set(it.style, i)
  })
  const n = new Map<string, number>()
  for (const it of list) if (it.style && !it.form) n.set(it.style, (n.get(it.style) ?? 0) + 1)
  const at = (s: string) => rank.get(s) ?? Infinity
  const big = [...n.entries()].filter(([, c]) => c >= STYLE_MIN)
  const top = new Set(
    [...big]
      .sort((a, b) => b[1] - a[1] || at(a[0]) - at(b[0]))
      .slice(0, STYLE_MAX)
      .map((e) => e[0]),
  )
  const out = big
    .filter(([s]) => top.has(s))
    .sort((a, b) => at(a[0]) - at(b[0]))
    .map(([s]) => ({ id: s, label: s }))
  return out.length > 1 ? out : []
}

// Выбранного нет среди вариантов — «Все» в показе; сам выбор в памяти не стирается (как activeFamily, 056).
const active = (sel: string, opts: readonly Opt[]): string => (sel && opts.some((o) => o.id === sel) ? sel : '')

export interface Browsed<T> {
  items: T[] // что видно
  packs: Opt[] // варианты рядов; пусто — ряда нет
  forms: Opt[]
  styles: Opt[]
  sel: BrowseSel // действующий выбор (с «Все» там, где выбранного нет)
}

// Конвейер. Варианты каждого шага — из того, что осталось после предыдущего. Свои (без packId, form, style) скрыты при выбранном
// наборе, стиле, «Уголках» и «Разделителях»; «Стикеры» их показывают. keep (стоящая плитка: «Заменить…», фон области) видна всегда.
export function browse<T extends BrowseItem>(list: readonly T[], order: readonly BrowseItem[], f: BrowseSel, keep = ''): Browsed<T> {
  const packs = packOpts(list, order)
  const pack = active(f.pack, packs)
  const a = pack ? list.filter((i) => i.packId === pack) : list
  const forms = formOpts(a)
  const form = active(f.form, forms) as FormSel
  const b = form ? a.filter((i) => formOf(i) === form) : a
  const styles = styleOpts(b, order)
  const style = active(f.style, styles)
  const c = style ? b.filter((i) => i.style === style) : b
  const shown = new Set<BrowseItem>(c)
  const items = c === list || !keep ? [...c] : list.filter((i) => shown.has(i) || i.id === keep)
  return { items, packs, forms, styles, sel: { pack, form, style } }
}

// Числа слева: сколько плиток будет видно после щелчка по строке — тем же конвейером (и «Куда» у «Разворотов» — extra).
// Go считает по всем, поэтому «Недавние 3» при двух видимых плитках без этого.
export function sideCounts(
  cat: DecorCatalog | null,
  f: BrowseSel,
  o: { extra?: (list: DecorItem[]) => DecorItem[]; keep?: string } = {},
): SideItem<string>[] {
  const order = cat?.items ?? []
  return sideItems(cat).map((t) => {
    const list = themeItems(cat, t.id)
    return { ...t, count: browse(o.extra ? o.extra(list) : list, order, f, o.keep).items.length }
  })
}

// Подпись плитки: при выбранном стиле — без приставки «Значки: » («Колокольчик»); при «Все» и скрытом ряде — полное имя
// (иначе «Значки: Колокольчик» и встроенный «Колокольчик» стали бы одинаковыми). Полное имя — в подсказке и просмотре.
export function tileLabel(it: Pick<BrowseItem, 'name' | 'style'>, style: string): string {
  const pre = style && it.style === style ? `${style}: ` : ''
  return pre && it.name.startsWith(pre) && it.name.length > pre.length ? it.name.slice(pre.length) : it.name
}
