// Пункты меню правого щелчка по группе ленты. Место у курсора — общий placeAtPoint (base/ui/menulist.ts).
import type { MenuNode } from '../../base/ui/menulist'
import type { IconName } from '../../base/ui/Icon.svelte'

// Пункт виньетки (031) в меню правого щелчка: обычный лист → «Сделать виньеткой», виньетка → «Обычный разворот»
// (у листа-проекта — «Обычный лист»). Только для одной группы: на нескольких выделенных пункта нет.
export interface VigItem {
  act: 'make' | 'clear' | 'setup'
  label: string
}

// canMake — функция «Виньетка» входит в уровень (на «Простом» «Сделать виньеткой» не предлагается; готовую виньетку вернуть в обычный можно всегда).
export function vignetteItem(count: number, kind: string | undefined, sheet = false, canMake = true): VigItem | null {
  if (count !== 1) return null
  if (kind === 'vignette') return { act: 'clear', label: sheet ? 'Обычный лист' : 'Обычный разворот' }
  if (!canMake) return null
  return { act: 'make', label: 'Сделать виньеткой' }
}

// «Настроить виньетку…» (031 dialog §3): только у виньетки, одна группа, функция входит в уровень.
export const setupItem = (count: number, kind: string | undefined, canMake = true): VigItem | null =>
  count === 1 && kind === 'vignette' && canMake ? { act: 'setup', label: 'Настроить виньетку…' } : null

// Пункт удаления: одна группа или все выделенные.
export const deleteLabel = (count: number, sheet = false) =>
  sheet ? (count > 1 ? `Удалить листы (${count})` : 'Удалить лист') : count > 1 ? `Удалить развороты (${count})` : 'Удалить разворот'

// «Разнообразить раскладки» (041): как удаление — число в скобках при нескольких группах.
export const varyLabel = (count: number) => (count > 1 ? `Разнообразить раскладки (${count})` : 'Разнообразить раскладки')

// Пункт активен, если функция входит в уровень, это не лист проекта и среди целей есть разворот, который Go может менять
// (`anyVaries`: «Авто» с фото; сетки «Ручная», виньетки и пустые он пропускает). То же правило — в «Правке».
export const canVary = (varies: boolean, sheet: boolean, can: boolean) => can && !sheet && varies

// Пункты разреза, склейки и названия (094 §1.8, §3.3): только у одной группы; разрез и склейка — с функцией
// «Разрез и склейка» (timeline), «Название…» — с «Названиями» (names). Серый пункт несёт причину (why).
// cut — разрез перед фото под курсором (cutState; show = нет — пункта нет), join — со следующей (у последней — нет).
type Can = { ok: boolean; why: string }

export interface StripItems {
  rename: boolean
  cut: Can | null
  join: Can | null
}

export function stripItems(
  count: number,
  can: (feature: 'timeline' | 'names') => boolean,
  cut: (Can & { show: boolean }) | undefined,
  join: Can | undefined,
): StripItems {
  if (count !== 1) return { rename: false, cut: null, join: null }
  const strip = can('timeline')
  return {
    rename: can('names'),
    cut: strip && cut?.show ? { ok: cut.ok, why: cut.why } : null,
    join: strip && join ? { ok: join.ok, why: join.why } : null,
  }
}

// «Вставить разворот перед / после» (103 §2): только у одной группы, на всех уровнях (103-9); у альбома листов — «лист».
export interface InsertItem {
  where: 'before' | 'after'
  label: string
}

export function insertItems(count: number, unit: { one: string }): InsertItem[] {
  if (count !== 1) return []
  return [
    { where: 'before', label: `Вставить ${unit.one} перед` },
    { where: 'after', label: `Вставить ${unit.one} после` },
  ]
}

// Значок по месту вставки: новый лист слева («перед») или справа («после») от текущего. В ленте разворотов и во «Всех
// разворотах» порядок слева направо, «перед» = левее.
export const insertIcon = (where: 'before' | 'after'): IconName => (where === 'before' ? 'insert-before' : 'insert-after')

// Что умеет меню: какие обработчики заданы и в каком состоянии цели. Сюда SpreadMenu сводит свои props и prefs.
export interface SpreadMenuSpec {
  count: number
  kind?: string
  sheet: boolean
  unit: { one: string }
  can: (feature: 'timeline' | 'names' | 'vignette' | 'variants') => boolean
  rename: boolean // задан onRename
  cut?: Can & { show: boolean }
  join?: Can
  insert: boolean // задан onInsert
  vignette: boolean // задан onVignette
  newVignette: boolean // задан onNewVignette (плитка «+»)
  vary: boolean // задан onVary
  varies: boolean
  del: boolean // задан onDelete
  label?: string // свой текст удаления (плитка обложки)
  how: string // способ мышью для разреза и склейки («Alt+щелчок»…)
}

// Пункты разреза, склейки и названия (серый несёт причину в title, способ мышью — в hint).
function stripNodes(o: SpreadMenuSpec): MenuNode[] {
  const strip = stripItems(o.count, o.can, o.cut, o.join)
  const out: MenuNode[] = []
  if (o.rename && strip.rename) out.push({ id: 'rename', icon: 'text', label: 'Название…' })
  if (strip.cut) {
    out.push({
      id: 'cut',
      icon: 'scissors',
      label: 'Разрезать перед этим фото',
      hint: o.how,
      disabled: !strip.cut.ok,
      title: strip.cut.why || undefined,
    })
  }
  if (strip.join) {
    out.push({
      id: 'join',
      icon: 'merge',
      label: 'Склеить со следующим',
      hint: o.how,
      disabled: !strip.join.ok,
      title: strip.join.why || undefined,
    })
  }
  return out
}

// Пункты виньетки: «Настроить…» перед «Сделать виньеткой» / «Обычный разворот».
function vignetteNodes(o: SpreadMenuSpec): MenuNode[] {
  const out: MenuNode[] = []
  for (const it of [setupItem(o.count, o.kind, o.can('vignette')), vignetteItem(o.count, o.kind, o.sheet, o.can('vignette'))]) {
    if (it) out.push({ id: 'vignette', arg: it.act, icon: 'vignette', label: it.label })
  }
  return out
}

// Дерево меню группы для общего MenuList (спека 108, как PhotoMenu): пункты, порядок и тексты прежние.
// id пункта — действие, arg — «перед/после» или act виньетки.
export function spreadMenu(o: SpreadMenuSpec): MenuNode[] {
  const out = stripNodes(o)
  if (o.insert)
    for (const it of insertItems(o.count, o.unit)) out.push({ id: 'insert', arg: it.where, icon: insertIcon(it.where), label: it.label })
  if (o.vignette) out.push(...vignetteNodes(o))
  if (o.newVignette) out.push({ id: 'newVignette', icon: 'vignette', label: 'Новая виньетка' })
  if (o.vary && !o.sheet && o.can('variants')) {
    out.push({ id: 'vary', icon: 'shuffle', label: varyLabel(o.count), disabled: !canVary(o.varies, o.sheet, o.can('variants')) })
  }
  if (o.del) out.push({ id: 'delete', icon: 'spread-delete', label: o.label ?? deleteLabel(o.count, o.sheet), danger: true })
  return out
}
