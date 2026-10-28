// Карта клавиатуры окна «Горячие клавиши» (074, design §8): ряды клавиш ANSI — основной блок, Ins/Del и стрелки.
// Цифрового блока нет: он приводится к верхнему ряду (eventChord). Размеры — в долях «единицы» клавиши.

export interface KeyDef {
  kind: 'key'
  code: string // код клавиши сочетания (KeyQ, Digit1, Escape…)
  en: string // подпись слева вверху
  ru: string // русская буква справа внизу, если есть
  w: number
}
export interface ModDef {
  kind: 'mod'
  mod: 'mod' | 'alt' | 'shift' // слой карты, который клавиша включает
  w: number
}
export interface DeadDef {
  kind: 'dead' // клавиша без сочетания (Caps, Win, Menu)
  label: string
  w: number
}
export interface GapDef {
  kind: 'gap'
  w: number
}
export type Cell = KeyDef | ModDef | DeadDef | GapDef
export interface BoardRow {
  cells: Cell[]
  fn?: boolean // верхний ряд Esc/F1–F12: ниже обычного
}

// Русская буква на клавише ЙЦУКЕН (по коду клавиши).
export const RU: Readonly<Record<string, string>> = {
  Backquote: 'Ё',
  KeyQ: 'Й',
  KeyW: 'Ц',
  KeyE: 'У',
  KeyR: 'К',
  KeyT: 'Е',
  KeyY: 'Н',
  KeyU: 'Г',
  KeyI: 'Ш',
  KeyO: 'Щ',
  KeyP: 'З',
  BracketLeft: 'Х',
  BracketRight: 'Ъ',
  KeyA: 'Ф',
  KeyS: 'Ы',
  KeyD: 'В',
  KeyF: 'А',
  KeyG: 'П',
  KeyH: 'Р',
  KeyJ: 'О',
  KeyK: 'Л',
  KeyL: 'Д',
  Semicolon: 'Ж',
  Quote: 'Э',
  KeyZ: 'Я',
  KeyX: 'Ч',
  KeyC: 'С',
  KeyV: 'М',
  KeyB: 'И',
  KeyN: 'Т',
  KeyM: 'Ь',
  Comma: 'Б',
  Period: 'Ю',
}

const k = (code: string, en: string, w = 1): KeyDef => ({ kind: 'key', code, en, ru: RU[code] ?? '', w })
const mod = (m: ModDef['mod'], w: number): ModDef => ({ kind: 'mod', mod: m, w })
const dead = (label: string, w: number): DeadDef => ({ kind: 'dead', label, w })
const gap = (w: number): GapDef => ({ kind: 'gap', w })
const letters = (s: string): KeyDef[] => [...s].map((l) => k('Key' + l, l))
const fkeys = (from: number): KeyDef[] => [0, 1, 2, 3].map((i) => k('F' + (from + i), 'F' + (from + i)))

export const MAIN: readonly BoardRow[] = [
  { fn: true, cells: [k('Escape', 'Esc'), gap(1), ...fkeys(1), gap(0.5), ...fkeys(5), gap(0.5), ...fkeys(9)] },
  {
    cells: [
      k('Backquote', '`'),
      ...[...'1234567890'].map((d) => k('Digit' + d, d)),
      k('Minus', '-'),
      k('Equal', '='),
      k('Backspace', '⌫ Backspace', 2),
    ],
  },
  {
    cells: [k('Tab', 'Tab', 1.5), ...letters('QWERTYUIOP'), k('BracketLeft', '['), k('BracketRight', ']'), k('Backslash', '\\', 1.5)],
  },
  { cells: [dead('Caps', 1.75), ...letters('ASDFGHJKL'), k('Semicolon', ';'), k('Quote', "'"), k('Enter', 'Enter', 2.25)] },
  {
    cells: [mod('shift', 2.25), ...letters('ZXCVBNM'), k('Comma', ','), k('Period', '.'), k('Slash', '/'), mod('shift', 2.75)],
  },
  {
    cells: [
      mod('mod', 1.25),
      dead('Win', 1.25),
      mod('alt', 1.25),
      k('Space', 'Пробел', 6.25),
      mod('alt', 1.25),
      dead('Win', 1.25),
      dead('Menu', 1.25),
      mod('mod', 1.25),
    ],
  },
]

export const NAV: readonly BoardRow[] = [
  { fn: true, cells: [] },
  { cells: [k('Insert', 'Ins'), k('Home', 'Home'), k('PageUp', 'PgUp')] },
  { cells: [k('Delete', 'Del'), k('End', 'End'), k('PageDown', 'PgDn')] },
  { cells: [] },
  { cells: [gap(1), k('ArrowUp', '↑')] },
  { cells: [k('ArrowLeft', '←'), k('ArrowDown', '↓'), k('ArrowRight', '→')] },
]

// Размер платы в единицах клавиши: основной блок 15, зазор 0,25 и блок навигации 3; по высоте — ряд F (0,8 + 0,25) и пять рядов.
export const BOARD_W = 18.25
export const BOARD_H = 6.05

// Коды клавиш, которые есть на карте (для проверки, что ни одна клавиша белого списка не потерялась).
export function boardCodes(): string[] {
  return [...MAIN, ...NAV].flatMap((r) => r.cells.filter((c): c is KeyDef => c.kind === 'key').map((c) => c.code))
}

// Ширина ряда в единицах (для проверки: основной блок — 15, блок навигации — 3).
export const rowWidth = (r: BoardRow): number => r.cells.reduce((s, c) => s + c.w, 0)

// Подпись команды на клавише: переносится только по словам. Слово не влезает в ширину клавиши — сначала мельче шрифт
// (до CMD_MIN), потом слово обрезается с «…» (полное имя — в title клавиши). Знак жирной кириллицы ≈ 0,62 em.
export const CMD_MIN = 7
const CMD_EM = 0.62
const CMD_CHROME = 9 // рамка кнопки, поля .kw и отступы подписи по бокам, px
export function fitCmd(short: string, w: number, u: number): { text: string; font: number } {
  const base = Math.min(11, Math.max(8, u * 0.19))
  const avail = Math.max(0, w * u - CMD_CHROME)
  const words = short.split(/\s+/).filter(Boolean)
  const longest = Math.max(1, ...words.map((x) => [...x].length))
  const font = Math.min(base, avail / (longest * CMD_EM))
  if (font >= CMD_MIN) return { text: short, font }
  const max = Math.max(2, Math.floor(avail / (CMD_MIN * CMD_EM)))
  const cut = (x: string) => ([...x].length > max ? [...x].slice(0, max - 1).join('') + '…' : x)
  return { text: words.map(cut).join(' '), font: CMD_MIN }
}
