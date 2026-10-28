// Полоса «‹ ▦ ›» над сторонками обложки (127): чистая логика — какие группы, где стоят, что делает ‹ ›.
// Состояние сторонки (places/known/alone) считает Go и отдаёт в виде; тут только решение, что показать.
import type { CoverSide } from '../../base/types/print'

export const NO_OTHER = 'Других раскладок для этих фото нет'
export const MANUAL_TITLE = 'Места сдвинуты вручную: ‹ › откроют раскладки сторонки'
// Больше семи мест — вкладка «7+» галереи (число мест в Go зажато так же, `Seg` «Число мест»).
export const MAX_TAB = 7

// «лицевой» / «задней» — для подписей кнопок и подсказок.
export const sideWho = (scope: CoverSide['scope']): string => (scope === 'right' ? 'лицевой' : 'задней')

// Центр безопасной зоны сторонки в процентах ширины листа; ширина листа — из вида, геометрию на фронте не считаем.
export function groupLeft(side: Pick<CoverSide, 'box'>, sheetW: number): number {
  if (!(sheetW > 0)) return 50
  return ((side.box.x + side.box.w / 2) / sheetW) * 100
}

// Вкладка «Число мест» при открытии ▦ по ‹ ›: по местам сторонки, пусто — «1».
export function openN(side: Pick<CoverSide, 'places'>): number {
  return side.places <= 0 ? 1 : Math.min(side.places, MAX_TAB)
}

// Что делает › (и Пробел) на сторонке: листает, открывает ▦ или объясняет тостом (0.26-127-15, -18).
export type CoverAct = { kind: 'step' } | { kind: 'open'; n: number } | { kind: 'toast'; text: string }
export function coverAct(side: Pick<CoverSide, 'places' | 'known' | 'alone'>): CoverAct {
  if (side.places <= 0 || !side.known) return { kind: 'open', n: openN(side) }
  if (side.alone) return { kind: 'toast', text: NO_OTHER }
  return { kind: 'step' }
}

// Группа полосы: ручная — метка «· Ручная ·»; still — стрелки недоступны (в ряду одна раскладка, и она стоит).
export interface BarGroup {
  side: CoverSide
  left: number
  who: string
  manual: boolean
  still: boolean
}
export function barGroups(sides: readonly CoverSide[] | undefined, sheetW: number): BarGroup[] {
  return (sides ?? [])
    .filter((s) => s.scope === 'left' || s.scope === 'right')
    .map((side) => ({
      side,
      left: groupLeft(side, sheetW),
      who: sideWho(side.scope),
      manual: side.places > 0 && !side.known,
      still: side.known && side.alone,
    }))
}
