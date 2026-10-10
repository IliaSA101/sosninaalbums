import { describe, expect, it } from 'vitest'
import { editorCss, faceSpecs, fontCss, fontUrl, styleSupport, svgTextCss } from './fonts'
import type { FontFamily } from '../../base/core/types'

const fonts: FontFamily[] = [
  {
    id: 'montserrat',
    name: 'Montserrat',
    css: 'SA Montserrat',
    faces: [
      { file: 'Montserrat-Regular.ttf', bold: false, italic: false },
      { file: 'Montserrat-Bold.ttf', bold: true, italic: false },
      { file: 'Montserrat-Italic.ttf', bold: false, italic: true },
    ],
  },
  { id: 'marck', name: 'Marck Script', css: 'SA Marck Script', faces: [{ file: 'MarckScript-Regular.ttf', bold: false, italic: false }] },
]

describe('faceSpecs', () => {
  it('по записи на начертание, адрес /media/font/<файл>', () => {
    const s = faceSpecs(fonts)
    expect(s).toHaveLength(4)
    expect(s[1]).toEqual({ family: 'SA Montserrat', source: 'url(/media/font/Montserrat-Bold.ttf)', weight: '700', style: 'normal' })
    expect(s[2].style).toBe('italic')
  })
  it('имя файла экранируется', () => expect(fontUrl('a b/../c.ttf')).toBe('url(/media/font/a%20b%2F..%2Fc.ttf)'))
})

describe('fontCss', () => {
  it('жирный и курсив — если начертание есть', () => {
    expect(fontCss({ font: 'montserrat', bold: true, italic: true }, fonts)).toEqual({
      family: '"SA Montserrat", sans-serif',
      weight: 700,
      style: 'italic',
    })
  })
  it('у Marck Script Ж и К не применяются', () => {
    const c = fontCss({ font: 'marck', bold: true, italic: true }, fonts)
    expect(c.weight).toBe(400)
    expect(c.style).toBe('normal')
  })
  it('неизвестный font — запасной, имя в CSS не попадает', () => {
    expect(fontCss({ font: 'x"; color:red' }, fonts).family).toBe('sans-serif')
  })
  it('кавычки в css-имени вырезаются', () => {
    const f = [{ ...fonts[0], css: 'A"B' }]
    expect(fontCss({ font: 'montserrat' }, f).family).toBe('"AB", sans-serif')
  })
})

describe('styleSupport', () => {
  it('по таблице шрифтов', () => {
    expect(styleSupport('montserrat', fonts)).toEqual({ bold: true, italic: true })
    expect(styleSupport('marck', fonts)).toEqual({ bold: false, italic: false })
    expect(styleSupport('нет', fonts)).toEqual({ bold: false, italic: false })
  })
})

describe('svgTextCss / editorCss', () => {
  const st = { font: 'montserrat', size: 72, color: '#112233', align: 'center' as const }
  it('кегль в мм для SVG и в px для редактора', () => {
    expect(svgTextCss(st, fonts)).toContain('font-size:25.4px')
    expect(editorCss(st, fonts, 2)).toContain('font-size:50.8px')
    expect(editorCss(st, fonts, 2)).toContain('line-height:1.2')
  })
  it('цвет из файла проверяется — инъекция в style невозможна', () => {
    const bad = { ...st, color: 'red;background:url(x)' }
    expect(svgTextCss(bad, fonts)).toContain('fill:#000000;')
    expect(editorCss(bad, fonts, 1)).not.toContain('url(')
  })
  it('шрифт не из таблицы — запасной', () => {
    expect(svgTextCss({ ...st, font: 'evil' }, fonts)).toContain('font-family:sans-serif')
  })
})
