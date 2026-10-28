// Общий список меню (спека 108): меню правого щелчка по фото и список «⋯» мини-панели. Чистая часть без DOM;
// вид и клавиши — как у строки меню программы (shell/menu/MenuBar.svelte), рисует MenuList.svelte.
import { placeSubmenu, type Box, type Size } from './popover'
import type { IconName } from './Icon.svelte'

export interface MenuNode {
  id: string // ActId или свой (у пунктов подменю — 'layer', 'backdrop')
  arg?: string // параметр пункта подменю: операция слоя, область фона
  label: string
  icon?: IconName // значок слева; у меню без значков места под него нет
  kind?: 'item' | 'note' // note — серая строка без действия
  hint?: string // клавиша справа
  title?: string // подсказка пункта (у серого — причина)
  disabled?: boolean
  danger?: boolean
  sep?: boolean // линия перед пунктом
  mark?: 'check' | 'radio' // пункт-состояние: ✓ или • в левом столбце
  checked?: boolean
  sub?: MenuNode[] // подменю сбоку; подменю в подменю не рисуется
}

// Нужен ли столбец значков: хотя бы у одного пункта списка есть значок. Тогда пункты без значка держат пустое место
// (текст ровно), а меню без значков (меню фото) не сдвигается.
export const hasIcons = (list: MenuNode[]): boolean => list.some((n) => !!n.icon)

// Столбец отметок (✓ •) рисуется, если отметки есть или значков нет (прежний вид меню фото); в меню со значками
// без отметок его место занимает столбец значков.
export const hasMarks = (list: MenuNode[]): boolean => list.some((n) => !!n.mark) || !hasIcons(list)

// Пункт, на который можно встать: не серый и не заметка.
export const focusable = (n: MenuNode): boolean => !n.disabled && n.kind !== 'note'

// Следующий пункт по ↑/↓ по кругу, серые и заметки пропускаются. at < 0 — фокуса в списке нет:
// вниз — первый, вверх — последний. Нет ни одного доступного — -1.
export function nextIndex(list: MenuNode[], at: number, delta: 1 | -1): number {
  const n = list.length
  if (n === 0) return -1
  let i = at < 0 ? (delta > 0 ? -1 : 0) : at
  for (let k = 0; k < n; k++) {
    i = (i + delta + n) % n
    if (focusable(list[i])) return i
  }
  return -1
}

// Что делают → и ←: на пункте с подменю → входит в него, в подменю ← возвращает к пункту. Иначе ничего.
export function arrowKey(key: 'ArrowRight' | 'ArrowLeft', node: MenuNode | undefined, inSub: boolean): 'enter' | 'back' | null {
  if (key === 'ArrowRight') return !inSub && node?.sub?.length ? 'enter' : null
  return inSub ? 'back' : null
}

// Ключ пункта в списке: у пунктов подменю id общий (слой, фон), различает параметр.
export const nodeKey = (n: MenuNode): string => (n.arg === undefined ? n.id : `${n.id}:${n.arg}`)

// Роль пункта по отметке (ARIA): состояние читается экранным диктором.
export function nodeRole(n: MenuNode): 'menuitem' | 'menuitemcheckbox' | 'menuitemradio' {
  return n.mark === 'check' ? 'menuitemcheckbox' : n.mark === 'radio' ? 'menuitemradio' : 'menuitem'
}

export const markText = (n: MenuNode): string => (!n.checked ? '' : n.mark === 'radio' ? '•' : '✓')

// Список у точки щелчка: левый верх в точке, в окне держится целиком.
export function placeAtPoint(p: { x: number; y: number }, size: Size, view: Size, margin = 8): { left: number; top: number } {
  return {
    left: Math.max(margin, Math.min(p.x, view.width - size.width - margin)),
    top: Math.max(margin, Math.min(p.y, view.height - size.height - margin)),
  }
}

// Список рядом с кнопкой: в нужную сторону, не помещается — в другую; по вертикали вровень с кнопкой.
export function placeBeside(
  anchor: Box,
  size: Size,
  view: Size,
  side: 'right' | 'left',
  margin = 8,
): { left: number; top: number; flipped: boolean } {
  const right = placeSubmenu(anchor, size, view, margin)
  if (side === 'right' || anchor.left - size.width < margin) return right
  return { left: anchor.left - size.width, top: right.top, flipped: true }
}
