// Ряд вариантов «Авто» (027): счётчик с «+», когда Go может досчитать ещё, и листание при нём.
// 041: семейства, свёртка похожих и цели «Разнообразить раскладки».
import type { Choice, Family, LayoutScope, SpreadView } from '../../base/core/types'
import type { SideItem } from '../../base/tpl/SideList.svelte'

type Row = Pick<SpreadView, 'variant' | 'variants' | 'more'>

// «1 / 12+»; variant −1 (раскладки нет в ряду) — тире.
export function variantCounter(s: Row): string {
  return `${s.variant >= 0 ? s.variant + 1 : '—'} / ${s.variants}${s.more ? '+' : ''}`
}

// Листать нечего: раскладка в ряду, а ряд из одной и без продолжения.
export function stillSpread(s: Row): boolean {
  return s.variant >= 0 && s.variants <= 1 && !s.more
}

// Ряд «Вариантов» (050): по порядку ленты («Авто» и «Другие варианты») или с перестановкой.
export type RowOrder = 'inOrder' | 'rearranged'

// Ряд, где стоит текущая раскладка (Go: SpreadView.rearranged / PageView.rearranged); иначе — «В порядке ленты».
export function startOrder(s: Pick<SpreadView, 'rearranged' | 'pages'> | undefined, scope: LayoutScope): RowOrder {
  if (!s) return 'inOrder'
  const re = scope === 'spread' ? s.rearranged : s.pages?.[scope === 'left' ? 0 : 1]?.rearranged
  return re ? 'rearranged' : 'inOrder'
}

// Число раскладок в заголовке галереи. У разворота — то, что листает счётчик «N / M»:
// ряд по порядку ленты + «Другие варианты»; «С перестановкой» в листание не входит.
// У страницы «Других» нет — считаем по-прежнему оба ряда.
export function galleryCount(
  spreadScope: boolean,
  rows: { inOrder: readonly unknown[]; rearranged: readonly unknown[]; extra: readonly unknown[] },
): number {
  return rows.inOrder.length + (spreadScope ? rows.extra.length : rows.rearranged.length)
}

// ---------- 041: разнообразие подборки ----------

// Семейства подборки в порядке сегментов (подписи — как в спеке 041).
export const FAMILIES: readonly { id: Family; label: string }[] = [
  { id: 'dense', label: 'Плотно' },
  { id: 'hero', label: 'С главным фото' },
  { id: 'air', label: 'Воздух' },
  { id: 'bleed', label: 'В край' },
  { id: 'strip', label: 'Ленты' },
  { id: 'figure', label: 'Фигуры' },
]

export type FamilyFilter = Family | 'all'

// Семейства, что есть хотя бы в одном ряду: пустые сегменты не показываются.
export function familiesIn(rows: readonly (readonly Choice[])[]): readonly { id: Family; label: string }[] {
  const have = new Set(rows.flatMap((r) => r.map((c) => c.family)))
  return FAMILIES.filter((f) => have.has(f.id))
}

// Фильтр, которого уже нет среди семейств (подборка пересчиталась), — «Все».
export const activeFamily = (f: FamilyFilter, present: readonly { id: Family }[]): FamilyFilter =>
  f !== 'all' && present.some((p) => p.id === f) ? f : 'all'

// Подписи рядов — они же ключи «Похожих» (gallery.openTwins).
export const ROW = { rearranged: 'С перестановкой порядка', inOrder: 'В порядке ленты', extra: 'Другие варианты' } as const

export interface GalleryLine {
  label: string
  list: readonly Choice[]
  empty: string // текст в пустом ряду
  more: boolean // плитка «Ещё»
}

// Ряды галереи: «Другие варианты» — у разворота всегда (пустой — текст в ряду, как у соседних), у страницы их нет.
export function galleryLines(
  scope: 'spread' | 'left' | 'right',
  rows: { inOrder: readonly Choice[]; rearranged: readonly Choice[]; extra: readonly Choice[] },
  o: { why: string; canMore: boolean },
): GalleryLine[] {
  return [
    { label: ROW.rearranged, list: rows.rearranged, empty: o.why, more: false },
    { label: ROW.inOrder, list: rows.inOrder, empty: 'Нет раскладок', more: o.canMore },
    ...(scope === 'spread' ? [{ label: ROW.extra, list: rows.extra, empty: 'Нет других вариантов', more: false }] : []),
  ]
}

export interface ShownRow {
  shown: Choice[]
  hidden: number // свёрнуто в плитку «Похожие: N»
}

// Ряд под фильтром семейства; та же картинка (twin ≥ 0) сворачивается, кроме текущей раскладки. open — похожие раскрыты.
export function shownRow(list: readonly Choice[], o: { family: FamilyFilter; current?: string; open: boolean }): ShownRow {
  // Текущая раскладка видна всегда, даже если фильтр её семейства не выбрал (050).
  const byFamily = o.family === 'all' ? list : list.filter((c) => c.family === o.family || c.key === o.current)
  // Близнец — только того же семейства: иначе фильтр семейства спрятал бы «похожую» карточку другого вида.
  const twin = (c: Choice) => (c.twin ?? -1) >= 0 && c.key !== o.current && list[c.twin!]?.family === c.family
  const shown = o.open ? [...byFamily] : byFamily.filter((c) => !twin(c))
  return { shown, hidden: byFamily.filter(twin).length }
}

// ---------- 050: один ряд плиток и переключатель ряда слева ----------

export interface RowPart {
  line: string // подпись линии galleryLines (ключ «Похожих»)
  sep: string // подпись черты перед частью; '' — без черты
  shown: Choice[]
  hidden: number // свёрнуто в «Похожие: N»
  more: boolean // плитка «Ещё» в конце части
}

export interface GalleryRow {
  parts: RowPart[]
  empty: string // текст на месте ряда; '' — плитки есть
}

const NO_FAMILY = 'Нет раскладок этого семейства'

// Что показать в ряду. «В порядке ленты» — «Авто», затем за чертой «другие» («Другие варианты»); «С перестановкой» — свой ряд.
// Пустые части выкидываются, черта — только после непустой. «Ещё» стоит в конце своей части.
export function galleryRow(
  lines: readonly GalleryLine[],
  order: RowOrder,
  o: { family: FamilyFilter; current?: string; openTwins: readonly string[] },
): GalleryRow {
  const mine = lines.filter((l) => (order === 'rearranged' ? l.label === ROW.rearranged : l.label !== ROW.rearranged))
  const parts: RowPart[] = []
  for (const l of mine) {
    const r = shownRow(l.list, { family: o.family, current: o.current, open: o.openTwins.includes(l.label) })
    if (r.shown.length + r.hidden === 0 && !l.more) continue
    const sep = l.label === ROW.extra && parts.length > 0 ? 'другие' : ''
    parts.push({ line: l.label, sep, shown: r.shown, hidden: r.hidden, more: l.more })
  }
  if (parts.length > 0) return { parts, empty: '' }
  // Списки есть, а показывать нечего — фильтр семейства; иначе причина Go (у перестановки) или «Нет раскладок».
  return { parts, empty: mine.some((l) => l.list.length > 0) ? NO_FAMILY : (mine[0]?.empty ?? '') }
}

// Пункты переключателя ряда. Числа — с учётом фильтра семейства (текущая, показанная принудительно, не входит).
export function galleryOrders(lines: readonly GalleryLine[], family: FamilyFilter): SideItem<RowOrder>[] {
  const count = (perm: boolean) =>
    lines
      .filter((l) => (l.label === ROW.rearranged) === perm)
      .reduce((n, l) => n + l.list.filter((c) => family === 'all' || c.family === family).length, 0)
  const re = lines.find((l) => l.label === ROW.rearranged)
  const none = !re || re.list.length === 0
  return [
    { id: 'inOrder', label: 'В порядке ленты', count: count(false) },
    {
      id: 'rearranged',
      label: 'С перестановкой',
      count: count(true),
      off: none,
      title: none ? (re?.empty ?? 'Перестановка не даёт лучших раскладок') : 'Те же фото в другом порядке, чем в ленте',
    },
  ]
}

// Подсказка плитки (051): название формы от Go; у мозаики его нет — подсказки нет.
export const tileTitle = (c: Pick<Choice, 'formName'>): string | undefined => c.formName || undefined

// Подпись плитки «Похожие»: свёрнуты — «Похожие: N», раскрыты — «Свернуть».
export const twinsLabel = (hidden: number, open: boolean): string => (open ? 'Свернуть похожие' : `Похожие: ${hidden}`)

// Разворот, который «Разнообразить» может менять: есть фото, это не сетка («Ручная») и не виньетка (то же, что `varies` в Go).
export const varies = (s: Pick<SpreadView, 'photos' | 'grid' | 'kind'>): boolean => s.photos.length > 0 && !s.grid && !s.kind

// Есть ли среди целей хоть один такой разворот; пункты меню серые, когда делать нечего (одно правило для Правки и ленты).
export const anyVaries = (spreads: readonly SpreadView[], ids: readonly string[]): boolean =>
  ids.some((id) => {
    const s = spreads.find((x) => x.id === id)
    return !!s && varies(s)
  })

// Цель «Разнообразить раскладки»: выделенные в ленте (в порядке альбома), нет выделения — текущий разворот.
export function varyTargets(selected: readonly string[], current: string | undefined): string[] {
  if (selected.length > 0) return [...selected]
  return current ? [current] : []
}
