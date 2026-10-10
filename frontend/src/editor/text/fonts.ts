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

// Регистрирует шрифты в document.fonts и ждёт загрузки. Ошибка одного файла
// не мешает остальным (возвращается число загруженных).
export async function loadFonts(fonts: FontFamily[]): Promise<number> {
  const results = await Promise.allSettled(
    faceSpecs(fonts).map(async (s) => {
      const ff = new FontFace(s.family, s.source, { weight: s.weight, style: s.style, featureSettings: FEATURE_SETTINGS })
      await ff.load()
      document.fonts.add(ff)
    }),
  )
  return results.filter((r) => r.status === 'fulfilled').length
}

export interface FontCss {
  family: string // значение font-family (с запасным семейством)
  weight: 400 | 700
  style: 'normal' | 'italic'
}

// CSS-шрифт для стиля блока. font берётся только из таблицы State.fonts (текст из файла
// проекта недоверенный); неизвестный id — запасной шрифт. Жирный и курсив — только если
// у семейства есть такое начертание (у Marck Script их нет).
export function fontCss(st: Pick<TextStyle, 'font' | 'bold' | 'italic'>, fonts: FontFamily[]): FontCss {
  const f = fonts.find((x) => x.id === st.font)
  if (!f) return { family: 'sans-serif', weight: 400, style: 'normal' }
  const bold = !!st.bold && f.faces.some((x) => x.bold)
  const italic = !!st.italic && f.faces.some((x) => x.italic)
  return { family: `"${f.css.replace(/["\\]/g, '')}", sans-serif`, weight: bold ? 700 : 400, style: italic ? 'italic' : 'normal' }
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
