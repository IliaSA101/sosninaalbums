// Шрифты текстовых блоков (спека 006, design §5.1). Go отдаёт State.fonts, файлы лежат
// в /media/font/<file>. FontFace создаём из кода, а не @font-face в CSS: Vite пытается
// разрешить url(/media/…) при сборке и пишет предупреждение.
import type { FontFamily, TextStyle } from '../../base/core/types'

// Go не делает подстановок GSUB, поэтому лигатуры и calt выключены и на экране (иначе ширины разойдутся).
export const FEATURE_SETTINGS = '"liga" 0, "clig" 0, "calt" 0'
// Для style="" у SVG-текста; font-synthesis: none — без поддельного жирного и курсива.
export const TEXT_RENDER_CSS = `font-synthesis:none;font-variant-ligatures:none;font-feature-settings:${FEATURE_SETTINGS};white-space:pre`

export interface FaceSpec {
  family: string
  source: string
  weight: '400' | '700'
  style: 'normal' | 'italic'
}

export const fontUrl = (file: string) => `url(/media/font/${encodeURIComponent(file)})`

// Описания всех начертаний (чистая часть, под тест).
export function faceSpecs(fonts: FontFamily[]): FaceSpec[] {
  return fonts.flatMap((f) =>
    f.faces.map((face): FaceSpec => ({
      family: f.css,
      source: fontUrl(face.file),
      weight: face.bold ? '700' : '400',
      style: face.italic ? 'italic' : 'normal',
    })),
  )
}

// Регистрирует начертания в document.fonts, но не грузит: браузер возьмёт файл при первом
// использовании (текст на холсте, образец в открытом списке). Гарнитур 33, файлов 62 —
// грузить всё на старте незачем. Возвращает число зарегистрированных.
export function loadFonts(fonts: FontFamily[]): number {
  let n = 0
  for (const s of faceSpecs(fonts)) {
    try {
      document.fonts.add(new FontFace(s.family, s.source, { weight: s.weight, style: s.style, featureSettings: FEATURE_SETTINGS }))
      n++
    } catch {
      // Битое описание одного файла не мешает остальным.
    }
  }
  return n
}

// Группы списка шрифтов: id из Go (FontFamily.group) и подпись. Порядок — порядок в списке.
export const GROUPS: readonly (readonly [string, string])[] = [
  ['script', 'Рукописные'],
  ['kids', 'Детские и округлые'],
  ['serif', 'С засечками'],
  ['sans', 'Без засечек'],
  ['display', 'Декоративные'],
]

export interface FontGroup {
  id: string
  title: string
  fonts: FontFamily[]
}

// Гарнитуры по группам в порядке GROUPS, внутри — как пришли из Go. Пустые группы пропускаются;
// гарнитура с незнакомой группой уходит в последнюю, чтобы её можно было выбрать.
export function groupFonts(fonts: FontFamily[]): FontGroup[] {
  const known = new Set(GROUPS.map(([id]) => id))
  const last = GROUPS[GROUPS.length - 1][0]
  return GROUPS.map(([id, title]) => ({
    id,
    title,
    fonts: fonts.filter((f) => (known.has(f.group) ? f.group : last) === id),
  })).filter((g) => g.fonts.length > 0)
}

// Образец в списке шрифтов.
export const SAMPLE = 'Анна и Павел'

// Клавиши навигации по списку: новая активная строка или null, если клавиша не навигационная.
export function moveActive(key: string, active: number, n: number): number | null {
  if (n <= 0) return null
  switch (key) {
    case 'ArrowDown':
      return (active + 1) % n
    case 'ArrowUp':
      return (active - 1 + n) % n
    case 'Home':
      return 0
    case 'End':
      return n - 1
    default:
      return null
  }
}

// Переход по набранным буквам: индекс первой гарнитуры, чьё имя начинается с query (без регистра).
// Если текущая уже подходит — остаётся на ней. Нет совпадения — -1.
export function findByPrefix(names: string[], query: string, from: number): number {
  const q = query.toLowerCase()
  if (!q) return -1
  const hit = (i: number) => names[i]?.toLowerCase().startsWith(q) ?? false
  return hit(from) ? from : names.findIndex((_, i) => hit(i))
}

// = text.DefaultFont в Go: запасная гарнитура для знака, которого нет в выбранной (решение №66).
export const DEFAULT_FONT = 'montserrat'

export interface FontCss {
  family: string // значение font-family (с запасным семейством)
  weight: 400 | 700
  style: 'normal' | 'italic'
}

// CSS-шрифт для стиля блока. font берётся только из таблицы State.fonts (текст из файла
// проекта недоверенный); неизвестный id — запасной шрифт. Жирный и курсив — только если
// у семейства есть такое начертание (у Marck Script их нет). Следом — Montserrat: браузер
// берёт знак, которого нет в шрифте (№, ₽), оттуда же, откуда Go в JPEG, а не из системного.
export function fontCss(st: Pick<TextStyle, 'font' | 'bold' | 'italic'>, fonts: FontFamily[]): FontCss {
  const f = fonts.find((x) => x.id === st.font)
  if (!f) return { family: 'sans-serif', weight: 400, style: 'normal' }
  const bold = !!st.bold && f.faces.some((x) => x.bold)
  const italic = !!st.italic && f.faces.some((x) => x.italic)
  const name = (x: FontFamily) => `"${x.css.replace(/["\\]/g, '')}"`
  const fb = f.id !== DEFAULT_FONT ? fonts.find((x) => x.id === DEFAULT_FONT) : undefined
  const family = [name(f), ...(fb ? [name(fb)] : []), 'sans-serif'].join(', ')
  return { family, weight: bold ? 700 : 400, style: italic ? 'italic' : 'normal' }
}

// Доступны ли Ж и К (у гарнитуры есть такое начертание).
export function styleSupport(fontId: string, fonts: FontFamily[]): { bold: boolean; italic: boolean } {
  const f = fonts.find((x) => x.id === fontId)
  return { bold: !!f?.faces.some((x) => x.bold), italic: !!f?.faces.some((x) => x.italic) }
}

// pt → мм (кегль в Go и в SVG разворота — в мм).
export const ptToMm = (pt: number) => (pt * 25.4) / 72

const COLOR_RE = /^#[0-9a-fA-F]{6}$/
// Цвет из файла проекта попадает в style="": только #rrggbb, иначе запасной.
export const safeColor = (c: string, fallback = '#000000') => (COLOR_RE.test(c) ? c : fallback)

// style="" строки SVG-текста: размер в единицах viewBox (мм разворота).
export function svgTextCss(st: TextStyle, fonts: FontFamily[]): string {
  const f = fontCss(st, fonts)
  return `font-family:${f.family};font-weight:${f.weight};font-style:${f.style};font-size:${ptToMm(st.size)}px;fill:${safeColor(st.color)};${TEXT_RENDER_CSS}`
}

// style="" редактора на странице: те же шрифт, кегль (px = мм × pxPerMm), цвет и выравнивание,
// межстрочный 1.2 — как в Go (internal/text).
export function editorCss(st: TextStyle, fonts: FontFamily[], pxPerMm: number): string {
  const f = fontCss(st, fonts)
  const align = st.align === 'left' || st.align === 'right' ? st.align : 'center'
  return `font-family:${f.family};font-weight:${f.weight};font-style:${f.style};font-size:${ptToMm(st.size) * pxPerMm}px;line-height:1.2;color:${safeColor(st.color)};text-align:${align}`
}
