// Обложка на холсте (спека 032): чистые решения для слоёв листа. Геометрию считает Go (View.cover),
// здесь только перевод мм листа в проценты, выбор зон броска и сторонок для прилипания.
import { isCover } from '../../base/geom/sheet'
import type { BackdropScope, CoverLayout, CoverView, CoverZone, Rect } from '../../base/core/types'

export interface SheetBox {
  w: number
  h: number
}

// Прямоугольник мм листа -> проценты размера листа (как у мест и подписей в Spread).
export function pctBox(r: Rect, s: SheetBox): { left: number; top: number; width: number; height: number } {
  if (!(s.w > 0) || !(s.h > 0)) return { left: 0, top: 0, width: 0, height: 0 }
  return { left: (r.x / s.w) * 100, top: (r.y / s.h) * 100, width: (r.w / s.w) * 100, height: (r.h / s.h) * 100 }
}

// Те же проценты строкой для style (3 знака хватает: лист в тысячу px).
export function boxCss(r: Rect, s: SheetBox): string {
  const b = pctBox(r, s)
  const f = (v: number) => Math.round(v * 1000) / 1000
  return `left:${f(b.left)}%;top:${f(b.top)}%;width:${f(b.width)}%;height:${f(b.height)}%`
}

// Зона броска -> область фона (SetBackdrop): задняя — левая, лицевая — правая, через корешок — весь лист.
export const DROP_SCOPE: Record<string, BackdropScope> = { back: 'left', front: 'right', wrap: 'spread' }

export const scopeOfDrop = (kind: string): BackdropScope | null => DROP_SCOPE[kind] ?? null

// Подписи областей фона на обложке (в карточке вместо «Левая страница»).
export const COVER_SCOPE_LABEL: Record<BackdropScope, string> = {
  spread: 'Вся обложка',
  left: 'Задняя',
  right: 'Лицевая',
}

export const COVER_LAYOUTS: readonly { kind: CoverLayout; label: string }[] = [
  { kind: 'front', label: 'Фото на лицевой' },
  { kind: 'both', label: 'Лицевая + задняя' },
  { kind: 'wrap', label: 'Одно фото через корешок' },
]

// Какие области фона принимают фото при раскладке: front — лицевая, both — обе сторонки, wrap — весь лист.
export function layoutScopes(layout: CoverLayout | string | undefined): readonly BackdropScope[] {
  if (layout === 'both') return ['left', 'right']
  if (layout === 'wrap') return ['spread']
  return ['right']
}

export const zonesOf = (cv: Pick<CoverView, 'zones'>, kind: string): CoverZone[] => cv.zones.filter((z) => z.kind === kind)

// Зона корешка (плашка и надпись); нет — undefined.
export const spineZone = (cv: Pick<CoverView, 'zones'> | null | undefined): CoverZone | undefined =>
  cv?.zones.find((z) => z.kind === 'spine')

// Зона с подписью на холсте. «Заворот» и «Расстав» повторяются по нескольку раз — подпись только у первой.
export function labeled(zones: readonly CoverZone[]): { z: CoverZone; label: string }[] {
  const seen = new Set<string>()
  return zones.map((z) => {
    const repeat = z.kind === 'wrap' || z.kind === 'hinge'
    const show = !!z.label && (!repeat || !seen.has(z.kind))
    seen.add(z.kind)
    return { z, label: show ? z.label : '' }
  })
}

// Сторонки обложки по X (задняя, лицевая), мм: центры сторонок для прилипания вместо mid/2 (snap.ts).
// Не обложка (cv нет) — undefined, и snapLines считает по шву.
export function sidesOf(cv: Pick<CoverView, 'zones'> | null | undefined): [number, number][] | undefined {
  if (!cv) return undefined
  const sides = cv.zones
    .filter((z) => z.kind === 'back' || z.kind === 'front')
    .map((z): [number, number] => [z.rect.x, z.rect.x + z.rect.w])
    .sort((a, b) => a[0] - b[0])
  return sides.length ? sides : undefined
}

// Сторонки для прилипания жестов листа: только у обложки (по её id), иначе линии идут по шву.
export function coverSides(sv: { id: string }, cv: Pick<CoverView, 'zones'> | null | undefined): [number, number][] | undefined {
  return isCover(sv) ? sidesOf(cv) : undefined
}

// Корешок «Корешок 12 → 14 мм»: число без хвостов и с запятой, как подписи зон из Go.
export function mmText(v: number): string {
  return String(Math.round(v * 10) / 10).replace('.', ',')
}

export const spineNoteText = (from: number, to: number): string => `Корешок ${mmText(from)} → ${mmText(to)} мм`
