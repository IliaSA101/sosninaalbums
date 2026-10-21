// Разрез и склейка в ленте (094 §1): мышью — только Ctrl+щелчок (на Mac ⌘), с клавиатуры — Enter/Пробел
// на зоне или черте без Ctrl (случайным такое нажатие не бывает). Обработчики компонентов SpreadChips и JoinLine.
import { toggleKey } from '../../base/core/platform'
import { store } from '../../base/core/store.svelte'
import { drag } from '../../base/drag/drag.svelte'
import { tip } from '../../base/ui/tip.svelte'
import type { CanDo } from './timeline'

export type PressEv = { ctrlKey: boolean; metaKey: boolean; stopPropagation: () => void }
export type KeyEv = { key: string; preventDefault: () => void; stopPropagation: () => void }

// Enter или Пробел на кнопке. Действие — по keydown с preventDefault: синтетического щелчка кнопки не будет
// (иначе с зажатым Ctrl сработало бы дважды).
function pressKey(e: KeyEv): boolean {
  if (e.key !== 'Enter' && e.key !== ' ') return false
  e.preventDefault()
  e.stopPropagation()
  return true
}

// Щелчок по зоне перед фото j. Без Ctrl событие не трогаем: зона его и не ловит (pointer-events), а если дошло —
// пусть идёт к миниатюре (выделение). С Ctrl — гасим, иначе миниатюра возьмёт его как «группу в выделение».
export function cutClick(e: PressEv, id: string, j: number): boolean {
  if (!toggleKey(e)) return false
  e.stopPropagation()
  if (!drag.clickAllowed()) return false
  tip.hide()
  void store.split(id, j)
  return true
}

export function cutKey(e: KeyEv, id: string, j: number): boolean {
  if (!pressKey(e)) return false
  tip.hide()
  void store.split(id, j)
  return true
}

// Черта между группами i и i+1: id — разворот слева, st — можно ли (joinState), cuts — функция на уровне.
export function joinClick(e: PressEv, id: string, st: CanDo, cuts: boolean): boolean {
  if (!cuts || !toggleKey(e) || !drag.clickAllowed() || !st.ok) return false
  tip.hide()
  void store.merge(id)
  return true
}

export function joinKey(e: KeyEv, id: string, st: CanDo, cuts: boolean): boolean {
  if (!cuts || !pressKey(e) || !st.ok) return false
  tip.hide()
  void store.merge(id)
  return true
}
