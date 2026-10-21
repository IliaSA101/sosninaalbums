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

// Щель между элементами ряда (095): «после j» = «до j+1» — одна щель, одна черта.
export const gapOf = (index: number, side: 'before' | 'after'): number => (side === 'after' ? index + 1 : index)

// Итоговый индекс элемента, брошенного в щель at; from — его нынешний индекс.
export const gapIndex = (at: number, from: number): number => (at > from ? at - 1 : at)

// Элемент с индексом from в щель at ничего не меняет: щель прямо перед ним или сразу после него.
export const gapNoop = (from: number, at: number): boolean => from >= 0 && (at === from || at === from + 1)

// Бросок фото из позиции from на сторону side миниатюры index того же разворота ничего не меняет.
export const isNoopMove = (from: number, index: number, side: 'before' | 'after'): boolean => gapNoop(from, gapOf(index, side))

type PhotoPayload = Extract<Payload, { kind: 'photos' }>

// Холостой бросок фото (095 сц. 7): не подсвечиваем ни черты, ни «+». spread — разворот цели; на свой лист и свою
// группу ленты — всегда холостой; в щель своего разворота (chip, cell до/после) — только одно фото у своего места.
// from — нынешний индекс фото на этом развороте, at — щель.
export function photoNoop(kind: 'chip' | 'cell' | 'mini' | 'spread', p: PhotoPayload, spread: string, from = -1, at = -1): boolean {
  if (!p.fromSpread || p.fromSpread !== spread) return false
  if (kind === 'mini' || kind === 'spread') return true
  return p.ids.length === 1 && gapNoop(from, at)
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

// Кнопка «Подборки» (083): цель только для фото из ленты. С холста бросок в ленту убирает фото из альбома —
// эти два смысла не смешиваем.
export function binTarget(p: Payload): Target | null {
  return p.kind === 'photos' && !p.fromSpread && p.ids.length > 0 ? { kind: 'bin' } : null
}

// Зона «Фото фона» в карточке разворота: на фон встаёт одно фото. Несколько — цель с many (без подсветки,
// бросок даёт тост «только одно»), иначе бросок молча не срабатывал бы. d — dataset с data-drop="backdrop".
export function backdropTarget(d: { spread?: string; scope?: string }, p: Payload): Target | null {
  if (p.kind !== 'photos' || p.ids.length === 0 || !d.spread) return null
  if (d.scope !== 'spread' && d.scope !== 'left' && d.scope !== 'right' && d.scope !== 'spine') return null
  if (p.ids.length > 1) return { kind: 'backdrop', spread: d.spread, scope: d.scope, many: true }
  return { kind: 'backdrop', spread: d.spread, scope: d.scope }
}

// Цель из стопки элементов под курсором (document.elementsFromPoint, сверху вниз).
// Плавающая панель над холстом (data-chrome: карточка листа, рейка, меню, окна) закрывает всё под собой: бросок
// «никуда», а не на лист, спрятанный под панелью; своя цель внутри панели (зона «Фото фона») остаётся.
// Блок текста лежит поверх мест сетки (data-through): бросок на него попадает в место под ним, а не в разворот.
export function pickDrop(stack: Element[]): HTMLElement | null {
  const top = stack[0]
  if (!top) return null
  const own = top.closest<HTMLElement>('[data-drop]')
  const chrome = top.closest('[data-chrome]')
  if (chrome) return own && chrome.contains(own) ? own : null
  if (!top.closest('[data-through]')) return own
  return (stack.find((e) => (e as HTMLElement).dataset?.drop === 'place') as HTMLElement | undefined) ?? own
}
