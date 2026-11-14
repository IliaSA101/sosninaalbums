// Положение всплывающего окна у якоря так, чтобы оно целиком оставалось в окне программы.

export interface Box {
  left: number
  top: number
  width: number
  height: number
}

export interface Size {
  width: number
  height: number
}

export interface Placed {
  left: number
  top: number
  above: boolean // окно развёрнуто вверх от якоря
}

const clamp = (v: number, lo: number, hi: number) => Math.max(lo, Math.min(v, hi))

// Предпочтительно под якорем по его левому краю. Не влезло справа — сдвиг влево; не влезло снизу —
// над якорем, а если и там тесно — прижато к краю окна (со стороны, где места больше).
export function placePopover(anchor: Box, size: Size, view: Size, gap = 6, margin = 8): Placed {
  const left = clamp(anchor.left, margin, Math.max(margin, view.width - size.width - margin))

  const below = anchor.top + anchor.height + gap
  const aboveTop = anchor.top - gap - size.height
  const maxTop = Math.max(margin, view.height - size.height - margin)
  if (below <= maxTop) return { left, top: below, above: false }
  if (aboveTop >= margin) return { left, top: aboveTop, above: true }
  const useAbove = anchor.top > view.height - (anchor.top + anchor.height)
  return { left, top: clamp(useAbove ? aboveTop : below, margin, maxTop), above: useAbove }
}

// Подменю рядом с пунктом: справа от него (с нахлёстом на border), у правого края окна — слева.
// По вертикали выровнено по пункту и не выходит за окно.
export function placeSubmenu(item: Box, size: Size, view: Size, margin = 8): { left: number; top: number; flipped: boolean } {
  const right = item.left + item.width
  const flipped = right + size.width > view.width - margin && item.left - size.width >= margin
  const left = flipped ? item.left - size.width : right
  return { left, top: clamp(item.top, margin, Math.max(margin, view.height - size.height - margin)), flipped }
}

// Высота выпадающего списка по вертикали [minTop, view.height - margin]: влез — maxHeight null, иначе список
// ограничен доступным местом и прокручивается внутри (так же, как FontPicker: max-height + overflow-y).
function fitY(want: number, natural: number, view: Size, minTop: number, margin: number): { top: number; maxHeight: number | null } {
  const room = Math.max(0, view.height - margin - minTop)
  if (natural > room) return { top: minTop, maxHeight: room }
  return { top: clamp(want, minTop, view.height - margin - natural), maxHeight: null }
}

// Выпадающий список строки меню: сразу под якорем (вверх не разворачивается — не наезжает на строку меню),
// низ с отступом от края окна. `size.height` — естественная высота списка без ограничения.
export function placeMenu(anchor: Box, size: Size, view: Size, margin = 8): { left: number; top: number; maxHeight: number | null } {
  const left = clamp(anchor.left, margin, Math.max(margin, view.width - size.width - margin))
  const y = fitY(anchor.top + anchor.height, size.height, view, anchor.top + anchor.height, margin)
  return { left, top: y.top, maxHeight: y.maxHeight }
}

// Подменю строки меню у своего пункта (как placeSubmenu), но не выше `minTop` (низ строки меню) и с ограничением
// высоты, если не влезает. Пункт — его текущий прямоугольник в окне, поэтому прокрутка родителя учтена.
export function placeSubmenuFit(
  item: Box,
  size: Size,
  view: Size,
  minTop: number,
  margin = 8,
): { left: number; top: number; flipped: boolean; maxHeight: number | null } {
  const side = placeSubmenu(item, size, view, margin)
  const y = fitY(item.top, size.height, view, minTop, margin)
  return { left: side.left, top: y.top, flipped: side.flipped, maxHeight: y.maxHeight }
}

// Окно панели открылось (Popover шлёт на window с якорем в detail): другие открытые окна панели закрываются, как при
// щелчке по чужой кнопке, — но когда окно открыла клавиша (B, Shift+B), щелчка нет. Вложенное окно (якорь внутри
// этого окна) его не закрывает.
export const BARPOP_OPEN = 'barpop-open'
export function closesOnPeer(opened: unknown, own: { anchor?: unknown; pop?: { contains(n: never): boolean } | null }): boolean {
  if (typeof opened !== 'object' || opened === null || opened === own.anchor) return false
  return !own.pop?.contains(opened as never)
}
