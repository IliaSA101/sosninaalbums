import { describe, expect, it, vi } from 'vitest'
import {
  GROUPS,
  editorCss,
  faceSpecs,
  findByPrefix,
  fontCss,
  fontUrl,
  groupFonts,
  loadFonts,
  moveActive,
  styleSupport,
  svgTextCss,
} from './fonts'
import type { FontFamily } from '../../base/core/types'

const fonts: FontFamily[] = [
  {
    id: 'montserrat',
    name: 'Montserrat',
    css: 'SA Montserrat',
    group: 'sans',
    faces: [
      { file: 'Montserrat-Regular.ttf', bold: false, italic: false },
      { file: 'Montserrat-Bold.ttf', bold: true, italic: false },
      { file: 'Montserrat-Italic.ttf', bold: false, italic: true },
    ],
  },
  {
    id: 'marck',
    name: 'Marck Script',
    css: 'SA Marck Script',
    group: 'script',
    faces: [{ file: 'MarckScript-Regular.ttf', bold: false, italic: false }],
  },
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
  it('запасное семейство Montserrat — знак вне шрифта рисуется им, как в JPEG', () => {
    expect(fontCss({ font: 'marck' }, fonts).family).toBe('"SA Marck Script", "SA Montserrat", sans-serif')
  })
  it('у самого Montserrat запасного нет; без Montserrat в таблице — только sans-serif', () => {
    expect(fontCss({ font: 'montserrat' }, fonts).family).toBe('"SA Montserrat", sans-serif')
    expect(fontCss({ font: 'marck' }, [fonts[1]]).family).toBe('"SA Marck Script", sans-serif')
  })
  it('запасное семейство попадает в style="" SVG и редактора', () => {
    const st = { font: 'marck', size: 72, color: '#000000', align: 'center' as const }
    expect(svgTextCss(st, fonts)).toContain('font-family:"SA Marck Script", "SA Montserrat", sans-serif;')
    expect(editorCss(st, fonts, 1)).toContain('font-family:"SA Marck Script", "SA Montserrat", sans-serif;')
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

describe('styleSupport: два начертания', () => {
  const two: FontFamily[] = [
    {
      id: 'caveat',
      name: 'Caveat',
      css: 'SA Caveat',
      group: 'script',
      faces: [
        { file: 'Caveat-Regular.ttf', bold: false, italic: false },
        { file: 'Caveat-Bold.ttf', bold: true, italic: false },
      ],
    },
  ]
  it('жирный есть, курсива нет', () => expect(styleSupport('caveat', two)).toEqual({ bold: true, italic: false }))
  it('курсив не применяется, жирный — да', () => {
    const c = fontCss({ font: 'caveat', bold: true, italic: true }, two)
    expect(c.weight).toBe(700)
    expect(c.style).toBe('normal')
  })
})

describe('groupFonts', () => {
  const mk = (id: string, group: string): FontFamily => ({ id, name: id, css: id, group, faces: [] })
  it('группы по порядку GROUPS, внутри — как пришли', () => {
    const list = [
      mk('s1', 'sans'),
      mk('c1', 'script'),
      mk('k1', 'kids'),
      mk('s2', 'sans'),
      mk('r1', 'serif'),
      mk('c2', 'script'),
      mk('d1', 'display'),
    ]
    const g = groupFonts(list)
    expect(g.map((x) => x.id)).toEqual(GROUPS.map(([id]) => id))
    expect(g.map((x) => x.title)).toEqual(['Рукописные', 'Детские и округлые', 'С засечками', 'Без засечек', 'Декоративные'])
    expect(g[0].fonts.map((f) => f.id)).toEqual(['c1', 'c2'])
    expect(g[3].fonts.map((f) => f.id)).toEqual(['s1', 's2'])
  })
  it('пустые группы пропускаются, незнакомая группа уходит в последнюю', () => {
    const g = groupFonts([mk('a', 'script'), mk('b', ''), mk('c', 'нет')])
    expect(g.map((x) => x.id)).toEqual(['script', 'display'])
    expect(g[1].fonts.map((f) => f.id)).toEqual(['b', 'c'])
  })
  it('пять групп в порядке спеки 046, «Декоративные» — последняя', () => {
    expect(GROUPS.map(([id, title]) => `${id}:${title}`)).toEqual([
      'script:Рукописные',
      'kids:Детские и округлые',
      'serif:С засечками',
      'sans:Без засечек',
      'display:Декоративные',
    ])
    const g = groupFonts([mk('p', 'display'), mk('s', 'sans')])
    expect(g.map((x) => x.id)).toEqual(['sans', 'display'])
  })
})

describe('findByPrefix', () => {
  const names = ['Bad Script', 'Caveat', 'Comfortaa', 'Lora']
  it('первое имя с таким началом, без регистра', () => {
    expect(findByPrefix(names, 'c', -1)).toBe(1)
    expect(findByPrefix(names, 'COM', 1)).toBe(2)
  })
  it('текущая уже подходит — остаётся', () => expect(findByPrefix(names, 'c', 2)).toBe(2))
  it('нет совпадения и пустой запрос — -1', () => {
    expect(findByPrefix(names, 'z', 0)).toBe(-1)
    expect(findByPrefix(names, '', 0)).toBe(-1)
  })
})

describe('moveActive', () => {
  it('стрелки идут по кругу, Home и End — к краям', () => {
    expect(moveActive('ArrowDown', 2, 3)).toBe(0)
    expect(moveActive('ArrowUp', 0, 3)).toBe(2)
    expect(moveActive('Home', 2, 3)).toBe(0)
    expect(moveActive('End', 0, 3)).toBe(2)
  })
  it('прочие клавиши и пустой список — null', () => {
    expect(moveActive('a', 0, 3)).toBeNull()
    expect(moveActive('ArrowDown', 0, 0)).toBeNull()
  })
})

describe('loadFonts', () => {
  it('регистрирует начертания, но не грузит', () => {
    const add = vi.fn()
    const load = vi.fn()
    class FakeFace {
      load = load
    }
    vi.stubGlobal('FontFace', FakeFace)
    vi.stubGlobal('document', { fonts: { add } })
    try {
      expect(loadFonts(fonts)).toBe(4)
      expect(add).toHaveBeenCalledTimes(4)
      expect(load).not.toHaveBeenCalled()
    } finally {
      vi.unstubAllGlobals()
    }
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
