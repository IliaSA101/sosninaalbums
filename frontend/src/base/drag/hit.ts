import type { Payload, Target } from '../core/types'

// Чистая часть определения цели перетаскивания (drag.svelte.ts): fx — доля ширины цели, 0..1.
export type CellSide = 'before' | 'after' | 'center'

// Доля ширины листа для Go (Target.at): при нулевой ширине fx = NaN — точки нет, -1 (иначе null → 0, левая страница).
export function sheetFrac(fx: number): number {
  return Number.isFinite(fx) ? Math.min(1, Math.max(0, fx)) : -1
}

// Ячейка: <25 % до, >75 % после, центр — обмен (из ленты фото, не из разворота, центр = «после»).
export function cellSide(fx: number, fromSpread: boolean): CellSide {
  const side: CellSide = fx < 0.25 ? 'before' : fx > 0.75 ? 'after' : 'center'
  return side === 'center' && !fromSpread ? 'after' : side
}

// Миниатюра ленты альбома и группа: делится пополам.
export const halfSide = (fx: number): 'before' | 'after' => (fx < 0.5 ? 'before' : 'after')

// Бросок фото из позиции from на сторону side миниатюры index того же разворота ничего не меняет.
export function isNoopMove(from: number, index: number, side: 'before' | 'after'): boolean {
  const at = side === 'after' ? index + 1 : index
  return from >= 0 && (at === from || at === from + 1)
}

// Место сетки (пустое или занятое): цель только для фото. Бросок единственного фото на его же место
// ничего не меняет — не подсвечиваем. d — dataset элемента с data-drop="place".
export function placeTarget(d: { spread?: string; place?: string; photo?: string }, p: Payload): Target | null {
  if (p.kind !== 'photos' || !d.spread || !d.place) return null
  if (d.photo && p.ids.length === 1 && p.ids[0] === d.photo) return null
  return { kind: 'place', spread: d.spread, place: d.place, photo: d.photo || null }
}

// Плитка обложки в ленте (032): на неё бросают фото, чтобы создать обложку. Несколько — цель с many
// (без подсветки; бросок объясняет тостом).
export function coverTarget(p: Payload): Target | null {
  if (p.kind !== 'photos' || p.ids.length === 0) return null
  return p.ids.length > 1 ? { kind: 'cover', many: true } : { kind: 'cover' }
}

// Зона «Фото фона» в карточке разворота: на фон встаёт одно фото. Несколько — цель с many (без подсветки,
// бросок даёт тост «только одно»), иначе бросок молча не срабатывал бы. d — dataset с data-drop="backdrop".
export function backdropTarget(d: { spread?: string; scope?: string }, p: Payload): Target | null {
  if (p.kind !== 'photos' || p.ids.length === 0 || !d.spread) return null
  if (d.scope !== 'spread' && d.scope !== 'left' && d.scope !== 'right' && d.scope !== 'spine') return null
  if (p.ids.length > 1) return { kind: 'backdrop', spread: d.spread, scope: d.scope, many: true }
  return { kind: 'backdrop', spread: d.spread, scope: d.scope }
}
