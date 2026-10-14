// Ряд вариантов «Авто» (027): счётчик с «+», когда Go может досчитать ещё, и листание при нём.
// 041: семейства, свёртка похожих и цели «Разнообразить раскладки».
import type { Choice, Family, SpreadView } from '../../base/core/types'

type Row = Pick<SpreadView, 'variant' | 'variants' | 'more'>

// «1 / 12+»; variant −1 (раскладки нет в ряду) — тире.
export function variantCounter(s: Row): string {
  return `${s.variant >= 0 ? s.variant + 1 : '—'} / ${s.variants}${s.more ? '+' : ''}`
}

// Листать нечего: раскладка в ряду, а ряд из одной и без продолжения.
export function stillSpread(s: Row): boolean {
  return s.variant >= 0 && s.variants <= 1 && !s.more
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
]

export type FamilyFilter = Family | 'all'

// Семейства, что есть хотя бы в одном ряду: пустые сегменты не показываются. Go ещё не размечает — список пуст.
export function familiesIn(rows: readonly (readonly Choice[])[]): readonly { id: Family; label: string }[] {
  const have = new Set(rows.flatMap((r) => r.map((c) => c.family)))
  return FAMILIES.filter((f) => have.has(f.id))
}

// Фильтр, которого уже нет среди семейств (подборка пересчиталась), — «Все».
export const activeFamily = (f: FamilyFilter, present: readonly { id: Family }[]): FamilyFilter =>
  f !== 'all' && present.some((p) => p.id === f) ? f : 'all'

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
    { label: 'С перестановкой порядка', list: rows.rearranged, empty: o.why, more: false },
    { label: 'В порядке ленты', list: rows.inOrder, empty: 'Нет раскладок', more: o.canMore },
    ...(scope === 'spread' ? [{ label: 'Другие варианты', list: rows.extra, empty: 'Нет других вариантов', more: false }] : []),
  ]
}

export interface ShownRow {
  shown: Choice[]
  hidden: number // свёрнуто в плитку «Похожие: N»
}

// Ряд под фильтром семейства; та же картинка (twin ≥ 0) сворачивается, кроме текущей раскладки. open — похожие раскрыты.
export function shownRow(list: readonly Choice[], o: { family: FamilyFilter; current?: string; open: boolean }): ShownRow {
  const byFamily = o.family === 'all' ? list : list.filter((c) => c.family === o.family)
  // Близнец — только того же семейства: иначе фильтр семейства спрятал бы «похожую» карточку другого вида.
  const twin = (c: Choice) => (c.twin ?? -1) >= 0 && c.key !== o.current && list[c.twin!]?.family === c.family
  const shown = o.open ? [...byFamily] : byFamily.filter((c) => !twin(c))
  return { shown, hidden: byFamily.filter(twin).length }
}

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
