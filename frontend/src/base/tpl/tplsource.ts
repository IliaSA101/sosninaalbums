// Источник шаблонов галереи (050): «Все · Встроенные · Мои» слева от ряда. Чистая логика — тексты приходят снаружи.
import type { SideItem } from './SideList.svelte'

export type TplSource = 'all' | 'builtin' | 'mine'
export interface TplPart<T> {
  sep: string // подпись черты перед частью; '' — без черты
  list: readonly T[]
}
export interface TplView<T> {
  items: SideItem<TplSource>[]
  parts: TplPart<T>[]
  empty: string // '' — плитки есть
}

// builtin — уже под фильтром вида (049), если он есть; mine — все свои.
export function tplSources<T>(
  builtin: readonly T[],
  mine: readonly T[],
  src: TplSource,
  t: { empty: Record<TplSource, string>; off: { builtin: string; mine: string } },
): TplView<T> {
  const b = builtin.length
  const m = mine.length
  const items: SideItem<TplSource>[] = [
    { id: 'all', label: 'Все', count: b + m },
    { id: 'builtin', label: 'Встроенные', count: b, off: b === 0, title: b === 0 ? t.off.builtin : undefined },
    { id: 'mine', label: 'Мои', count: m, off: m === 0, title: m === 0 ? t.off.mine : undefined },
  ]
  let parts: TplPart<T>[] = []
  if (src === 'builtin') {
    if (b > 0) parts = [{ sep: '', list: builtin }]
  } else if (src === 'mine') {
    if (m > 0) parts = [{ sep: '', list: mine }]
  } else {
    if (b > 0) parts.push({ sep: '', list: builtin })
    if (m > 0) parts.push({ sep: b > 0 ? 'мои' : '', list: mine })
  }
  return { items, parts, empty: parts.length > 0 ? '' : t.empty[src] }
}
