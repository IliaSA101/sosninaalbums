// Сочетание клавиш из события и разбор записи (спека 074, design §3). Чистые функции; те же
// векторы, что у Go (testdata/parity/keymap.json): chords — Canon, events — eventChord.
// Единственное место фронта, где читается KeyboardEvent.code (страж keys.guard).
import { isMac } from '../core/platform'

// Событие клавиатуры в том, что нужно разбору (KeyboardEvent подходит как есть).
export interface ChordEvent {
  key: string
  code: string
  ctrlKey: boolean
  shiftKey: boolean
  altKey: boolean
  metaKey: boolean
}

const NAMED = [
  'Minus',
  'Equal',
  'BracketLeft',
  'BracketRight',
  'Backslash',
  'Semicolon',
  'Quote',
  'Comma',
  'Period',
  'Slash',
  'Backquote',
  'IntlBackslash',
  'Space',
  'Enter',
  'Tab',
  'Escape',
  'Backspace',
  'Delete',
  'Insert',
  'Home',
  'End',
  'PageUp',
  'PageDown',
  'ArrowLeft',
  'ArrowRight',
  'ArrowUp',
  'ArrowDown',
]

// Белый список кодов — копия keymap.Codes() в Go (сверяет chord.test.ts).
export const KEY_CODES: readonly string[] = [
  ...Array.from({ length: 26 }, (_, i) => 'Key' + String.fromCharCode(65 + i)),
  ...Array.from({ length: 10 }, (_, i) => 'Digit' + i),
  ...Array.from({ length: 12 }, (_, i) => 'F' + (i + 1)),
  ...NAMED,
]
const CODE_SET: ReadonlySet<string> = new Set(KEY_CODES)

// Предел длины записи (keymap.MaxChord).
export const MAX_CHORD = 32
const MODS = ['Mod', 'Alt', 'Shift']

// Каноническая запись «[Mod+][Alt+][Shift+]<Код>»; null — не сочетание (как keymap.Canon).
export function canonChord(s: string): string | null {
  if (!s || s.length > MAX_CHORD) return null
  const parts = s.split('+')
  const code = parts[parts.length - 1]
  if (!CODE_SET.has(code)) return null
  const have = new Set<string>()
  for (const p of parts.slice(0, -1)) {
    if (!MODS.includes(p) || have.has(p)) return null
    have.add(p)
  }
  return [...MODS.filter((m) => have.has(m)), code].join('+')
}

// Код клавиши записи (после модификаторов).
export const codeOfChord = (chord: string): string => chord.slice(chord.lastIndexOf('+') + 1)

// Модификаторы записи (без кода).
export function modsOfChord(chord: string): string[] {
  const i = chord.lastIndexOf('+')
  return i < 0 ? [] : chord.slice(0, i).split('+')
}

// Сочетание без Shift: команды «со сдвигом ×10» (стрелки) ищутся по записи без него.
export function stripShift(chord: string): string {
  return modsOfChord(chord).includes('Shift') ? chord.replace('Shift+', '') : chord
}

// Модификаторы сами по себе: сочетанием не бывают, запись ждёт клавишу (074-23).
const MODIFIER_KEYS = new Set([
  'Control',
  'Shift',
  'Alt',
  'AltGraph',
  'Meta',
  'OS',
  'CapsLock',
  'NumLock',
  'ScrollLock',
  'Fn',
  'FnLock',
  'Hyper',
  'Super',
])

// Пустой code (экранная клавиатура, удалённый стол, синтетика; 074-26): код по символу.
// Раскладки QWERTY и ЙЦУКЕН (+ украинские і ї є ґ); со Shift символ другой, а клавиша та же.
const BY_CHAR: Record<string, string> = {}
const put = (chars: string, codes: string[]) => [...chars].forEach((c, i) => (BY_CHAR[c] = codes[i]))
const latin = (s: string) => [...s].map((c) => 'Key' + c)
const PUNCT = ['Minus', 'Equal', 'BracketLeft', 'BracketRight', 'Backslash', 'Semicolon', 'Quote', 'Comma', 'Period', 'Slash', 'Backquote']
put('йцукенгшщзхъфывапролджэячсмитьбюё', [
  ...latin('QWERTYUIOP'),
  'BracketLeft',
  'BracketRight',
  ...latin('ASDFGHJKL'),
  'Semicolon',
  'Quote',
  ...latin('ZXCVBNM'),
  'Comma',
  'Period',
  'Backquote',
])
put('іїєґ', ['KeyS', 'BracketRight', 'Quote', 'Backslash'])
put("-=[]\\;',./`", PUNCT)
put('_+{}|:"<>?~', PUNCT)
put(
  ')!@#$%^&*(',
  Array.from({ length: 10 }, (_, i) => 'Digit' + i),
)
BY_CHAR[' '] = 'Space'

function codeByChar(key: string): string | null {
  const k = key.toLowerCase()
  if (/^[a-z]$/.test(k)) return 'Key' + k.toUpperCase()
  if (/^[0-9]$/.test(k)) return 'Digit' + k
  if (CODE_SET.has(key)) return key // именованные: Delete, ArrowLeft, F5…
  return BY_CHAR[k] ?? null
}

// Цифровой блок → обычные коды (NumLock включён: key = цифра; выключен: key = имя навигации).
const NUMPAD_KEYS: Record<string, string> = { NumpadAdd: 'Equal', NumpadSubtract: 'Minus', NumpadEnter: 'Enter', NumpadDecimal: 'Period' }

function codeOfEvent(e: ChordEvent): string | null {
  if (/^[A-Za-z]$/.test(e.key)) return 'Key' + e.key.toUpperCase() // Dvorak, AZERTY — по букве
  if (!e.code) return codeByChar(e.key)
  if (/^Numpad[0-9]$/.test(e.code)) {
    if (/^[0-9]$/.test(e.key)) return 'Digit' + e.key
    return CODE_SET.has(e.key) ? e.key : null // NumLock выключен: ArrowLeft, Home, Delete…
  }
  if (e.code.startsWith('Numpad')) {
    if (e.code === 'NumpadDecimal' && CODE_SET.has(e.key)) return e.key // NumLock выключен: Delete
    return NUMPAD_KEYS[e.code] ?? null
  }
  return CODE_SET.has(e.code) ? e.code : null
}

function build(e: ChordEvent, allowModAlt: boolean): string | null {
  if (typeof e.key !== 'string') return null // синтетическое событие без key: сочетания нет, а не падение в toLowerCase
  if (MODIFIER_KEYS.has(e.key)) return null
  const mac = isMac()
  // Mod — строго основной модификатор платформы: Win на Windows и Ctrl на Mac — не наши (074-20).
  if (mac ? e.ctrlKey : e.metaKey) return null
  const mod = mac ? e.metaKey : e.ctrlKey
  // Ctrl+Alt на Windows — AltGr, Cmd+Alt в Go служебное: сочетанием не считаем (074-23).
  if (mod && e.altKey && !allowModAlt) return null
  const code = codeOfEvent(e)
  if (!code) return null
  return [mod && 'Mod', e.altKey && 'Alt', e.shiftKey && 'Shift', code].filter(Boolean).join('+')
}

// Сочетание из события; null — не наше (одни модификаторы, неизвестная клавиша, чужой модификатор,
// Ctrl+Alt). Зовётся синхронно в keydown диспетчера.
export const eventChord = (e: ChordEvent): string | null => build(e, false)

// То же для записи нового сочетания в окне: Mod+Alt остаётся, чтобы Go отказал с причиной (AltGr).
export const recordChord = (e: ChordEvent): string | null => build(e, true)

// Пробел — для keyup (короткий Пробел листает раскладки), модификаторы не важны.
export const isSpace = (e: { key: string; code: string }): boolean => e.code === 'Space' || (!e.code && e.key === ' ')
