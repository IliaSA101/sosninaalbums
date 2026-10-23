// Пункты меню правого щелчка по группе ленты. Место у курсора — общий placeAtPoint (base/ui/menulist.ts).

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
