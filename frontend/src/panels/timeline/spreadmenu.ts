// Положение меню правого щелчка по группе ленты: у курсора, но целиком внутри окна.
const GAP = 8 // отступ от края окна, px

export interface MenuSize {
  w: number
  h: number
}

export function menuPos(x: number, y: number, size: MenuSize, view: MenuSize): { x: number; y: number } {
  return {
    x: Math.max(GAP, Math.min(x, view.w - size.w - GAP)),
    y: Math.max(GAP, Math.min(y, view.h - size.h - GAP)),
  }
}

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
