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

// Пункт удаления: одна группа или все выделенные.
export const deleteLabel = (count: number) => (count > 1 ? `Удалить развороты (${count})` : 'Удалить разворот')
