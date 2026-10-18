// Запись сочетания в окне «Горячие клавиши» (074, design §8): что значит очередное нажатие, пока чип ждёт клавишу.
// Чистая функция; код клавиши читает только base/keys/chord.ts (recordChord).
import { recordChord, type ChordEvent } from '../../base/keys/chord'
import { modName } from '../../base/keys/chordtext'
import { isWidget } from '../menu/keys'

export interface RecEvent extends ChordEvent {
  repeat: boolean
  isComposing: boolean
}

export type RecStep =
  | { kind: 'skip' } // автоповтор и ввод IME: молча пропускаем
  | { kind: 'cancel' } // Esc: отмена записи, окно остаётся
  | { kind: 'partial'; text: string } // нажаты одни модификаторы: запись ждёт клавишу
  | { kind: 'bad' } // клавиша, которой нет в белом списке (медиа-клавиша, чужой модификатор)
  | { kind: 'chord'; chord: string }

// Сами модификаторы и «залипающие» клавиши — не сочетание.
const MODIFIER = /^(Control|Shift|Alt|AltGraph|Meta|OS|CapsLock|NumLock|ScrollLock|Fn|FnLock|Hyper|Super)$/

// Клавиша, которую отдаёт редактор ввода (IME), — не сочетание.
const IME_KEY = /^Process$/

// Недописанное сочетание для чипа: «Ctrl+Shift+…» (на Mac «⌥⇧…»); нет модификаторов — пусто.
export function partialText(e: ChordEvent, mac: boolean): string {
  const mod = mac ? e.metaKey : e.ctrlKey
  const names = (mac ? (['Alt', 'Shift', 'Mod'] as const) : (['Mod', 'Alt', 'Shift'] as const))
    .filter((m) => (m === 'Mod' ? mod : m === 'Alt' ? e.altKey : e.shiftKey))
    .map((m) => modName(m, mac))
  if (names.length === 0) return ''
  return mac ? names.join('') + '…' : names.join('+') + '+…'
}

export type PressStep =
  | { kind: 'pass' } // окно не берёт нажатие: Esc закроет окно, Tab перейдёт по полям, F12 откроет инструменты
  | { kind: 'select'; chord: string } // нажали клавишу: она выбирается на карте
  | { kind: 'mods' } // нажали что-то без сочетания (модификатор, медиа-клавиша): меняется только слой

// Элемент, которому принадлежат Enter, Пробел и стрелки: кнопка, флажок, пункт списка и виджеты (isWidget).
const OWNERS = "button, input[type='checkbox'], input[type='radio'], [role='button'], [role='option'], [role='tab']"
export function ownsKeys(el: { closest?(sel: string): unknown } | null | undefined): boolean {
  return isWidget(el) || (typeof el?.closest === 'function' && !!el.closest(OWNERS))
}
const isBare = (e: ChordEvent) => !e.ctrlKey && !e.altKey && !e.shiftKey && !e.metaKey
const OWN_KEYS = /^(Enter| |ArrowLeft|ArrowRight|ArrowUp|ArrowDown)$/

// Нажатие вне записи (окно открыто, фокус не в поле ввода). own — фокус на элементе, которому нужны Enter/Пробел/стрелки:
// им они и достаются, а не выбору клавиши на карте.
export function pressStep(e: ChordEvent, mac: boolean, own = false): PressStep {
  if (own && isBare(e) && OWN_KEYS.test(e.key)) return { kind: 'pass' }
  const bare = isBare(e)
  if (e.key === 'Escape' && bare) return { kind: 'pass' }
  if (e.key === 'Tab' && !e.altKey && !(mac ? e.metaKey : e.ctrlKey)) return { kind: 'pass' }
  const chord = recordChord(e)
  if (chord === 'F12') return { kind: 'pass' }
  return chord ? { kind: 'select', chord } : { kind: 'mods' }
}

export function recStep(e: RecEvent, mac: boolean): RecStep {
  if (e.repeat || e.isComposing || IME_KEY.test(e.key)) return { kind: 'skip' }
  if (e.key === 'Escape' && !e.ctrlKey && !e.altKey && !e.shiftKey && !e.metaKey) return { kind: 'cancel' }
  if (MODIFIER.test(e.key)) return { kind: 'partial', text: partialText(e, mac) }
  const chord = recordChord(e)
  return chord ? { kind: 'chord', chord } : { kind: 'bad' }
}
