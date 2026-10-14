// Что делать, когда payload брошен на цель: чистый план, исполняет его store.drop.
import type { BackdropScope, Payload, Target } from '../core/types'

export type DropPlan =
  | { op: 'moveSpread'; id: string; at: number }
  // at = -1 — в конец; fromSpread — фото уже в развороте (иначе из ленты: снять выделение, тост о переносе).
  // frac — доля ширины листа под курсором (042): ручному листу она выбирает страницу для новых мест; нет — без точки.
  | { op: 'add'; spread: string; ids: string[]; at: number; fromSpread: boolean; go: boolean; frac?: number }
  | { op: 'swap'; a: string; b: string }
  | { op: 'newSpread'; at: number; ids: string[]; strip: boolean }
  | { op: 'remove'; ids: string[] }
  // Фото на место сетки (Api.PlacePhotos): из ленты (fromSpread = false) или из альбома.
  | { op: 'place'; spread: string; place: string; ids: string[]; fromSpread: boolean }
  // Фото на фон области (Api.SetBackdrop): само остаётся там, где было.
  | { op: 'backdrop'; spread: string; scope: BackdropScope; photo: string }
  // Фото на плитку обложки (032): обложка создаётся с ним на лицевой.
  | { op: 'cover'; photo: string }
  // Бросок, который не выполняется, но о причине надо сказать (тост).
  | { op: 'reject'; text: string }

export const BACKDROP_ONE = 'На фон — только одно фото'
export const COVER_ONE = 'На обложку — только одно фото'

// Зона фона принимает ровно одно фото; несколько — не подсвечиваем, но при броске объясняем.
export const backdropAccepts = (p: Payload | null): boolean => p?.kind === 'photos' && p.ids.length === 1

type PhotoPayload = Extract<Payload, { kind: 'photos' }>

// Вставка «до/после» элемента с индексом index.
const insertAt = (side: 'before' | 'after' | 'center', index: number) => (side === 'after' ? index + 1 : index)

// Перенос разворота: только на миниатюру в ленте; на своё же место — ничего.
function spreadPlan(p: Extract<Payload, { kind: 'spread' }>, t: Target): DropPlan | null {
  if (t.kind !== 'mini') return null
  let at = insertAt(t.side, t.index)
  if (at > p.index) at--
  return at === p.index ? null : { op: 'moveSpread', id: p.id, at }
}

// Фото на ячейку: центр — обмен, иначе вставка рядом.
function cellPlan(p: PhotoPayload, t: Extract<Target, { kind: 'cell' }>): DropPlan | null {
  const ids = p.ids
  if (t.side === 'center') {
    // Центр бывает только у фото из разворота (из ленты hit даёт «после») — обмен.
    return ids.length === 1 && ids[0] !== t.photo ? { op: 'swap', a: ids[0], b: t.photo } : null
  }
  return { op: 'add', spread: t.spread, ids, at: insertAt(t.side, t.index), fromSpread: !!p.fromSpread, go: false }
}

// Фото на место сетки.
function placePlan(p: PhotoPayload, t: Extract<Target, { kind: 'place' }>): DropPlan | null {
  const ids = p.ids
  // Единственное фото на своё же место — ничего (hit такую цель не даёт, но план проверяет сам).
  if (t.photo && ids.length === 1 && ids[0] === t.photo) return null
  if (ids.length === 0) return null
  return { op: 'place', spread: t.spread, place: t.place, ids, fromSpread: !!p.fromSpread }
}

// Фото на фон: ровно одно, с несколькими — тост.
function backdropPlan(p: PhotoPayload, t: Extract<Target, { kind: 'backdrop' }>): DropPlan | null {
  const ids = p.ids
  if (ids.length === 0) return null
  return ids.length === 1 ? { op: 'backdrop', spread: t.spread, scope: t.scope, photo: ids[0] } : { op: 'reject', text: BACKDROP_ONE }
}

// Фото на весь лист (фон листа, соседний разворот, миниатюра): в конец; mini ещё и открывает лист.
function wholePlan(p: PhotoPayload, t: Extract<Target, { kind: 'spread' | 'mini' }>): DropPlan | null {
  if (p.fromSpread === t.spread) return null
  const plan: DropPlan = { op: 'add', spread: t.spread, ids: p.ids, at: -1, fromSpread: !!p.fromSpread, go: t.kind === 'mini' }
  // Доля ширины листа выбирает страницу для новых мест ручного листа (042).
  if (t.kind === 'spread' && t.at !== undefined) plan.frac = t.at
  return plan
}

export function dropPlan(p: Payload, t: Target): DropPlan | null {
  if (p.kind === 'spread') return spreadPlan(p, t)
  const ids = p.ids
  const fromSpread = !!p.fromSpread
  switch (t.kind) {
    case 'chip':
      return { op: 'add', spread: t.spread, ids, at: insertAt(t.side, t.index), fromSpread, go: false }
    case 'cell':
      return cellPlan(p, t)
    case 'spread':
    case 'mini':
      return wholePlan(p, t)
    case 'new':
      return { op: 'newSpread', at: t.at, ids, strip: !fromSpread }
    case 'place':
      return placePlan(p, t)
    case 'backdrop':
      return backdropPlan(p, t)
    case 'cover':
      if (ids.length === 0) return null
      return ids.length === 1 ? { op: 'cover', photo: ids[0] } : { op: 'reject', text: COVER_ONE }
    case 'library':
      return fromSpread ? { op: 'remove', ids } : null
  }
}
