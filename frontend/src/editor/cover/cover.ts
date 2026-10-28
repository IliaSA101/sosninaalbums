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

// Зона броска -> область фона (SetBackdrop): задняя — левая, лицевая — правая, корешок — своя (061),
// через корешок — весь лист.
export const DROP_SCOPE: Record<string, BackdropScope> = { back: 'left', front: 'right', spine: 'spine', wrap: 'spread' }

export const scopeOfDrop = (kind: string): BackdropScope | null => DROP_SCOPE[kind] ?? null

// Слои обложки на холсте (z-index внутри .spread): плашка корешка < фото корешка (061) <= зоны броска < .content (3).
// Фото корешка идёт в DOM раньше зон, поэтому при равном z зона сверху.
export const COVER_Z = { plate: 1, spine: 2, drop: 2 } as const

// Полоса попадания у зоны корешка, px экрана: на «вписать» сам корешок 5–13 px, в него не попасть (087-1).
export const SPINE_HIT_PX = 24

// Порядок зон в слое: корешок последним — при равном z он ложится поверх лицевой, и его полоса шире зоны
// не уходит под соседние; остальные — в прежнем порядке.
export function dropOrder<T extends { kind: string }>(zones: readonly T[]): T[] {
  return [...zones.filter((z) => z.kind !== 'spine'), ...zones.filter((z) => z.kind === 'spine')]
}

// Запасные подписи для цвета и узора: их области не зависят от раскладки (всегда сторонки и вся обложка),
// а `cover.scopes` — только области фото фона.
const SIDE_LABEL: Record<BackdropScope, string> = { spread: 'Вся обложка', left: 'Задняя', right: 'Лицевая', spine: 'Корешок' }

// Области фото фона раскладки — от Go (CoverView.scopes), порядок слева направо; обложки нет — пусто.
export const coverScopes = (cv: Pick<CoverView, 'scopes'> | null | undefined): readonly BackdropScope[] =>
  (cv?.scopes ?? []).map((s) => s.scope)

// Подпись области: у Go, если область в раскладке, иначе запасная.
export function coverLabel(cv: Pick<CoverView, 'scopes'> | null | undefined, scope: BackdropScope): string {
  return cv?.scopes?.find((s) => s.scope === scope)?.label ?? SIDE_LABEL[scope]
}

export const coverLabels = (cv: Pick<CoverView, 'scopes'> | null | undefined): Record<BackdropScope, string> => ({
  spread: coverLabel(cv, 'spread'),
  left: coverLabel(cv, 'left'),
  right: coverLabel(cv, 'right'),
  spine: coverLabel(cv, 'spine'),
})

// Кнопки раскладки обложки: порядок и подписи по спеке 061 (ключи прежние). short — слово под схемой в узкой
// карточке (плитка ≈ 55 px, 0.26-124-30); полная подпись остаётся в title и aria-label.
export const COVER_LAYOUTS: readonly { kind: CoverLayout; label: string; short: string }[] = [
  { kind: 'front', label: 'Лицевая', short: 'Лицевая' },
  { kind: 'wrap', label: 'Одно фото через корешок', short: 'Через корешок' },
  { kind: 'both', label: 'Задняя + корешок + лицевая', short: 'Три части' },
]

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
