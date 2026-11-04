// Источник шаблонов галереи (050): «Все · Встроенные · Мои» слева от ряда. Чистая логика — тексты приходят снаружи.
// Галерея сторонки обложки (127) добавляет ряд «По фото» — третьим списком; без него всё как прежде.
import type { SideItem } from './SideList.svelte'

export type TplSource = 'all' | 'byPhoto' | 'builtin' | 'mine'
export interface TplPart<T> {
  sep: string // подпись черты перед частью; '' — без черты
  list: readonly T[]
}
export interface TplView<T> {
  items: SideItem<TplSource>[]
  parts: TplPart<T>[]
  empty: string // '' — плитки есть
}

// Тексты: пустой ряд и причина серого пункта; byPhoto — только у галереи сторонки.
export interface TplTexts {
  empty: { all: string; builtin: string; mine: string; byPhoto?: string }
  off: { builtin: string; mine: string; byPhoto?: string }
}

// Пункт переключателя: серый с причиной, когда в ряду пусто.
const item = (id: TplSource, label: string, count: number, why?: string): SideItem<TplSource> => ({
  id,
  label,
  count,
  off: count === 0,
  title: count === 0 ? why : undefined,
})

// builtin — уже под фильтром вида (049), если он есть; mine — все свои.
// byPhoto: не передан — пункта «По фото» нет; передан (даже пустой) — пункт первым после «Все»,
// в «Все» идёт первой частью, а перед встроенными и своими встают черты «встроенные» и «мои».
export function tplSources<T>(builtin: readonly T[], mine: readonly T[], src: TplSource, t: TplTexts, byPhoto?: readonly T[]): TplView<T> {
  const p = byPhoto?.length ?? 0
  const items: SideItem<TplSource>[] = [{ id: 'all', label: 'Все', count: p + builtin.length + mine.length }]
  if (byPhoto) items.push(item('byPhoto', 'По фото', p, t.off.byPhoto))
  items.push(item('builtin', 'Встроенные', builtin.length, t.off.builtin), item('mine', 'Мои', mine.length, t.off.mine))
  const parts: TplPart<T>[] = []
  if (src === 'all') {
    // Черта — только когда перед частью уже что-то есть.
    for (const [label, list] of [
      ['', byPhoto ?? []],
      ['встроенные', builtin],
      ['мои', mine],
    ] as const) {
      if (list.length > 0) parts.push({ sep: parts.length > 0 ? label : '', list })
    }
  } else {
    const list = { byPhoto: byPhoto ?? [], builtin, mine }[src]
    if (list.length > 0) parts.push({ sep: '', list })
  }
  return { items, parts, empty: parts.length > 0 ? '' : (t.empty[src] ?? '') }
}
