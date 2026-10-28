// Показ сочетания: «Ctrl+Shift+Z» на Windows, «⇧⌘Z» на Mac (порядок Apple ⌃⌥⇧⌘). Буквы — латиница.
import { canonChord, codeOfChord, modsOfChord } from './chord'

const NAMES: Record<string, string> = {
  Space: 'Пробел',
  Delete: 'Del',
  Backspace: 'Backspace',
  Escape: 'Esc',
  Enter: 'Enter',
  Tab: 'Tab',
  Insert: 'Ins',
  Home: 'Home',
  End: 'End',
  PageUp: 'PgUp',
  PageDown: 'PgDn',
  ArrowLeft: '←',
  ArrowRight: '→',
  ArrowUp: '↑',
  ArrowDown: '↓',
  Minus: '−',
  Equal: '=',
  BracketLeft: '[',
  BracketRight: ']',
  Backslash: '\\',
  IntlBackslash: '<',
  Semicolon: ';',
  Quote: "'",
  Comma: ',',
  Period: '.',
  Slash: '/',
  Backquote: '`',
}

// Имя клавиши по коду: KeyZ → Z, Digit1 → 1, F5 → F5, остальное по таблице.
export function codeText(code: string): string {
  if (code.startsWith('Key')) return code.slice(3)
  if (code.startsWith('Digit')) return code.slice(5)
  return NAMES[code] ?? code
}

const MAC_MODS: Record<string, string> = { Mod: '⌘', Alt: '⌥', Shift: '⇧' }
const WIN_MODS: Record<string, string> = { Mod: 'Ctrl', Alt: 'Alt', Shift: 'Shift' }

// Имя модификатора для слоёв карты и недописанного сочетания: «Ctrl» / «⌘».
export const modName = (m: 'Mod' | 'Alt' | 'Shift', mac: boolean): string => (mac ? MAC_MODS : WIN_MODS)[m]

// Текст сочетания для подсказок, меню и окна. Не сочетание — как есть.
export function chordText(chord: string, mac: boolean): string {
  const c = canonChord(chord)
  if (!c) return chord
  if (c === 'Shift+Slash') return '?'
  const mods = modsOfChord(c)
  const key = codeText(codeOfChord(c))
  if (mac) return [...['Alt', 'Shift', 'Mod'].filter((m) => mods.includes(m)).map((m) => MAC_MODS[m]), key].join('')
  return [...mods.map((m) => WIN_MODS[m]), key].join('+')
}

// Несколько сочетаний одной строкой: «Ctrl+Y / Ctrl+Shift+Z». Стрелки со Shift («шаг ×10») отдельно не повторяются.
export function chordsText(chords: string[], mac: boolean): string {
  const plain = new Set(chords.filter((c) => !c.includes('+')))
  return chords
    .filter((c) => !(c.startsWith('Shift+Arrow') && plain.has(c.slice(6))))
    .map((c) => chordText(c, mac))
    .join(' / ')
}
