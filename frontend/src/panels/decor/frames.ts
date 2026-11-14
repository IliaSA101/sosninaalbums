// Окно «Рамочки» (079): чистые решения — «Куда» (выделенные места или весь лист), ID для Go, плитка, что стоит сейчас.
// Образец — «Куда» окна «Фоны»; цели считаем по View (геометрии здесь нет), саму рамочку ставит Go.
import type { SpreadView } from '../../base/core/types'
import type { SegItem } from '../../base/tpl/Seg.svelte'
import type { DecorItem } from './decor'

export type FrameWhere = 'sel' | 'all'

export const FRAME_ALL = 'Все места разворота'
export const FRAME_NO_PLACES = 'На этом листе нет мест, куда поставить рамочку'

// Пункты «Куда»: «Выделенные (N)» есть, только если что-то выделено (owner-ux 2.7: без выделения не серый пункт, а его нет).
export function frameWhereItems(selN: number): SegItem<FrameWhere>[] {
  const all: SegItem<FrameWhere> = { id: 'all', label: FRAME_ALL }
  return selN > 0 ? [{ id: 'sel', label: `Выделенные (${selN})` }, all] : [all]
}

// При открытии: есть выделенные — они; иначе весь лист.
export const frameWhereDefault = (selN: number): FrameWhere => (selN > 0 ? 'sel' : 'all')

// Выбранный пункт мог исчезнуть (выделение пропало): тогда «Все места разворота».
export const frameWhereIn = (where: FrameWhere, selN: number): FrameWhere => (where === 'sel' && selN > 0 ? 'sel' : 'all')

// Все места листа для ApplyBorder: у сетки — места (и пустые), у «Авто» — его фото (Go закрепит разворот).
export function framePlaceIds(sp: Pick<SpreadView, 'places' | 'photos'> | undefined): string[] {
  if (!sp) return []
  return sp.places ? sp.places.map((p) => p.id) : [...sp.photos]
}

// ID целей «Куда»: выделенные — те, что ещё есть на листе (ссылки мест или фото, как их принимает Go), или все места.
export function frameTargets(
  sp: Pick<SpreadView, 'places' | 'photos'> | undefined,
  where: FrameWhere,
  selected: readonly string[],
): string[] {
  return where === 'sel' ? [...selected] : framePlaceIds(sp)
}

// Рамочка, что стоит на целях сейчас, — для отметки плитки (как текущая раскладка в галерее ⊞): у всех целей одна и та же.
// Встроенная — по плитке, что её поставила (preset), своя — по ключу библиотеки. Правили руками или цели разные — ''.
export function frameStanding(sp: Pick<SpreadView, 'places'> | undefined, ids: readonly string[], items: readonly DecorItem[]): string {
  const places = sp?.places
  if (!places || ids.length === 0) return ''
  let key = ''
  for (const id of ids) {
    const b = places.find((p) => p.id === id || (p.photo !== '' && p.photo === id))?.border
    const k = b ? b.preset || b.lib : ''
    if (!k || (key && k !== key)) return ''
    key = k
  }
  return items.some((it) => it.id === key) ? key : ''
}
