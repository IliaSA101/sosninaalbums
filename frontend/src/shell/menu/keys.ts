// Горячие клавиши: событие и контекст -> действие. Само действие выполняет App.svelte.
// Сами клавиши — в таблице hotkeys.ts (она же даст справку).
import { findAction, type KeyAction, type KeyContext, type KeyEvent } from './hotkeys'

export type { KeyAction, KeyContext, KeyEvent }

// Поля ввода: текст и числа, а также блоки с contenteditable (редактор текста на развороте).
// Ползунок, флажок, радио и кнопки хоткеи не глушат.
export const TEXT_FIELDS =
  "textarea, input:not([type='range']):not([type='checkbox']):not([type='radio']):not([type='button']):not([type='submit']), [contenteditable]:not([contenteditable='false'])"

// Цель события бывает document или window — у них нет closest.
export const isTextField = (el: { closest?(sel: string): unknown } | null | undefined) =>
  typeof el?.closest === 'function' && !!el.closest(TEXT_FIELDS)

// Действия, у которых нужно гасить поведение браузера по умолчанию.
const PREVENT = new Set<KeyAction>([
  'undo',
  'redo',
  'save',
  'saveAs',
  'new',
  'open',
  'export',
  'reload',
  'toggleRulers',
  'zoomIn',
  'zoomOut',
  'fitZoom',
  'selectAll',
  'selectSpread',
  'nudgePlace',
  'layerFront',
  'layerUp',
  'layerDown',
  'layerBack',
  'spaceDown',
  'help',
  'bgNudge',
])
export const preventsDefault = (a: KeyAction) => PREVENT.has(a)

// В поле ввода хоткеи молчат; иначе первая подошедшая строка таблицы.
export function keyAction(e: KeyEvent, c: KeyContext): KeyAction | null {
  if (c.inTextField) return null
  return findAction(e, c)
}
