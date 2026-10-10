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
