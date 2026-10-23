// Палитра окна цвета: проверка HEX, наборы с именами, HSV и перенос прежних «Последних».
// Наборы — вид окна (109-5), не расчёт: HEX и имена из макета дословно.

const LEGACY_KEY = 'recentColors'
const LEGACY_MAX = 10

export interface ColorEntry {
  hex: string
  name: string
}

export interface ColorSet {
  id: string
  title: string
  colors: readonly ColorEntry[]
}

const c = (hex: string, name: string): ColorEntry => ({ hex, name })

// Пять наборов по 8 цветов; шестая вкладка «С фото» приходит от Go и здесь не живёт.
export const SETS: readonly ColorSet[] = [
  {
    id: 'neutral',
    title: 'Нейтральные',
    colors: [
      c('#ffffff', 'Белый'),
      c('#faf6ef', 'Слоновая кость'),
      c('#f1ebe1', 'Молочный'),
      c('#e4ddd3', 'Светлый лён'),
      c('#cfc8be', 'Тёплый серый'),
      c('#9c958c', 'Серо-бежевый'),
      c('#4a4643', 'Графит'),
      c('#1f1d1c', 'Почти чёрный'),
    ],
  },
  {
    id: 'pastel',
    title: 'Пастель',
    colors: [
      c('#f6dfe4', 'Розовая пастель'),
      c('#f6dccf', 'Персиковый'),
      c('#f6ecd2', 'Ванильный'),
      c('#e1ebd8', 'Фисташковый'),
      c('#d6e6e4', 'Мятный'),
      c('#d8e4ee', 'Голубой туман'),
      c('#dcd8ec', 'Лавандовый'),
      c('#ead9ea', 'Сиреневый'),
    ],
  },
  {
    id: 'powder',
    title: 'Пудра и нюд',
    colors: [
      c('#f1ddd6', 'Пудра'),
      c('#e8c9c0', 'Светлая пыльная роза'),
      c('#d8a9a1', 'Пыльная роза'),
      c('#c48f86', 'Розовый нюд'),
      c('#ead6c4', 'Нюд'),
      c('#d7bba2', 'Капучино'),
      c('#b8957f', 'Мокко'),
      c('#8c6a5b', 'Какао'),
    ],
  },
  {
    id: 'gold',
    title: 'Шампань',
    colors: [
      c('#f7eedc', 'Светлая шампань'),
      c('#eedcb9', 'Шампань'),
      c('#e2c896', 'Светлое золото'),
      c('#d1ad6c', 'Золото'),
      c('#b8955a', 'Старое золото'),
      c('#e9c9b4', 'Светлое розовое золото'),
      c('#d4a58c', 'Розовое золото'),
      c('#9c7a52', 'Бронза'),
    ],
  },
  {
    id: 'nature',
    title: 'Природа',
    colors: [
      c('#dfe3d3', 'Светлый шалфей'),
      c('#b8c4a9', 'Шалфей'),
      c('#8f9d7f', 'Оливковый'),
      c('#66735c', 'Эвкалипт'),
      c('#d9cdb8', 'Овёс'),
      c('#bba98d', 'Песок'),
      c('#a9b8c0', 'Пыльный голубой'),
      c('#7b8a93', 'Грозовой'),
    ],
  },
]

// Вкладка «С фото» — шестая, после наборов.
export const PHOTO_TAB = 'photo'
const TAB_KEY = 'colorTab'

// Последняя вкладка окна (настройка окна, как viewMode; 109-8): первая — «Нейтральные», мусор — тоже.
export function loadTab(): string {
  try {
    const v = localStorage.getItem(TAB_KEY)
    if (v === PHOTO_TAB || SETS.some((s) => s.id === v)) return v as string
  } catch {
    // Нет хранилища — первая вкладка.
  }
  return SETS[0].id
}

export function saveTab(id: string): void {
  try {
    localStorage.setItem(TAB_KEY, id)
  } catch {
    // Без хранилища вкладка не запомнится.
  }
}

// Имя цвета без набора: цвет с фото, пипетки, из «Своих» или введённый руками.
export const CUSTOM_NAME = 'Свой цвет'

// Имя из наборов, иначе «Свой цвет». HEX любого регистра.
export function nameOf(hex: string): string {
  const h = normalizeHex(hex)
  if (h) for (const s of SETS) for (const e of s.colors) if (e.hex === h) return e.name
  return CUSTOM_NAME
}

// '#rrggbb' или 'rrggbb' (регистр любой) -> '#rrggbb' в нижнем регистре; иначе null.
export function normalizeHex(s: string): string | null {
  const m = /^#?([0-9a-f]{6})$/i.exec(s.trim())
  return m ? '#' + m[1].toLowerCase() : null
}

// Начало правильного HEX: пока вводят «#12a», ошибкой это не считаем.
export const HEX_PREFIX = /^#?[0-9a-f]{0,6}$/i

// Текст ошибки для поля HEX; null — ввод верный.
export function hexError(s: string): string | null {
  return normalizeHex(s) ? null : 'Нужен цвет вида #rrggbb'
}

// h в градусах [0, 360), s и v — доли [0, 1].
export interface Hsv {
  h: number
  s: number
  v: number
}

export function hsvToHex({ h, s, v }: Hsv): string {
  const hh = (((h % 360) + 360) % 360) / 60
  const k = Math.floor(hh)
  const f = hh - k
  const p = v * (1 - s)
  const q = v * (1 - s * f)
  const t = v * (1 - s * (1 - f))
  const [r, g, b] = [
    [v, t, p],
    [q, v, p],
    [p, v, t],
    [p, q, v],
    [t, p, v],
    [v, p, q],
  ][k % 6]
  const byte = (x: number) =>
    Math.min(255, Math.max(0, Math.round(x * 255)))
      .toString(16)
      .padStart(2, '0')
  return '#' + byte(r) + byte(g) + byte(b)
}

// Неверный HEX — чёрный. У серого оттенок 0 (окно оставит полосу там, где она была).
export function hexToHsv(hex: string): Hsv {
  const n = normalizeHex(hex)
  if (!n) return { h: 0, s: 0, v: 0 }
  const r = parseInt(n.slice(1, 3), 16) / 255
  const g = parseInt(n.slice(3, 5), 16) / 255
  const b = parseInt(n.slice(5, 7), 16) / 255
  const max = Math.max(r, g, b)
  const d = max - Math.min(r, g, b)
  let h = 0
  if (d > 0) {
    if (max === r) h = ((g - b) / d) % 6
    else if (max === g) h = (b - r) / d + 2
    else h = (r - g) / d + 4
    h = (h * 60 + 360) % 360
  }
  return { h, s: max === 0 ? 0 : d / max, v: max }
}

// Прежние «Последние» из localStorage (до 109): только верные цвета без повторов, не больше LEGACY_MAX.
export function legacyRecent(): string[] {
  try {
    const v: unknown = JSON.parse(localStorage.getItem(LEGACY_KEY) ?? '[]')
    if (!Array.isArray(v)) return []
    const out: string[] = []
    for (const x of v) {
      const h = typeof x === 'string' ? normalizeHex(x) : null
      if (h && !out.includes(h)) out.push(h)
    }
    return out.slice(0, LEGACY_MAX)
  } catch {
    return []
  }
}

// После переноса ключ стирается (без хранилища — молча).
export function dropLegacyRecent(): void {
  try {
    localStorage.removeItem(LEGACY_KEY)
  } catch {
    // Нет хранилища — нечего стирать.
  }
}
