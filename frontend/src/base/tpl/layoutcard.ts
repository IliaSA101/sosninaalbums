// Плитка раскладки (LayoutCard): чистая часть — размеры и классы, под тесты. Геометрию считает Go (мм), здесь только проценты.
export interface CardRect {
  x: number
  y: number
  w: number
  h: number
}
export interface CardSheet {
  w: number
  h: number
}

// Ширина плитки высотой h — пропорции листа.
export const cardWidth = (h: number, sheet: CardSheet): number => (h * sheet.w) / sheet.h

// Рамка ячейки в процентах листа.
export const cellBox = (r: CardRect, sheet: CardSheet) => ({
  left: (r.x / sheet.w) * 100,
  top: (r.y / sheet.h) * 100,
  width: (r.w / sheet.w) * 100,
  height: (r.h / sheet.h) * 100,
})

// Классы плитки: current — текущая раскладка разворота, fallback — запасная (пунктир), hov — рамка при наведении (плитка нажимается).
export const cardClasses = (o: { current?: boolean; fallback?: boolean; clickable?: boolean }): string[] =>
  ['card', o.current && 'current', o.fallback && 'fallback', o.clickable && 'hov'].filter((c): c is string => !!c)
