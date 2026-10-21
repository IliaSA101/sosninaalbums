import { beforeEach, describe, expect, it, vi } from 'vitest'
import {
  GROUPS,
  REFUSED,
  SAMPLE,
  SAMPLE_LAT,
  checkFaces,
  failedText,
  loadAndCheck,
  noCyrTail,
  ownFaceFiles,
  resetFonts,
  sampleOf,
  settleImport,
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
import * as api from './fontapi'

vi.mock('./fontapi', () => ({ DropFonts: vi.fn(async (faces: string[]) => faces) }))
const dropFonts = vi.mocked(api.DropFonts)

beforeEach(() => {
  resetFonts()
  dropFonts.mockClear()
})

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
  it('неизвестный font — Montserrat, имя из файла в CSS не попадает', () => {
    expect(fontCss({ font: 'x"; color:red' }, fonts).family).toBe('"SA Montserrat", sans-serif')
  })
  it('нет шрифта на компьютере (082-7): Montserrat с начертанием из стиля, как в Go', () => {
    expect(fontCss({ font: 'my:0123456789abcdef', bold: true, italic: true }, fonts)).toEqual({
      family: '"SA Montserrat", sans-serif',
      weight: 700,
      style: 'italic',
    })
    expect(fontCss({ font: 'my:0123456789abcdef' }, fonts)).toMatchObject({ weight: 400, style: 'normal' })
  })
  it('нет и Montserrat в таблице (список ещё не пришёл) — только sans-serif', () => {
    expect(fontCss({ font: 'my:0123456789abcdef', bold: true }, [])).toEqual({ family: 'sans-serif', weight: 400, style: 'normal' })
  })
  it('своя гарнитура: свой css, следом Montserrat', () => {
    const my = { id: 'my:0123456789abcdef', name: 'Lato', css: 'SA my 0123456789abcdef', group: 'my', faces: [] }
    expect(fontCss({ font: my.id }, [...fonts, my]).family).toBe('"SA my 0123456789abcdef", "SA Montserrat", sans-serif')
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
    expect(g.map((x) => x.id)).toEqual(GROUPS.map(([id]) => id).filter((id) => id !== 'my'))
    expect(g.map((x) => x.title)).toEqual(['Рукописные', 'Детские и округлые', 'С засечками', 'Без засечек', 'Декоративные'])
    expect(g[0].fonts.map((f) => f.id)).toEqual(['c1', 'c2'])
    expect(g[3].fonts.map((f) => f.id)).toEqual(['s1', 's2'])
  })
  it('пустые группы пропускаются, незнакомая группа уходит в последнюю', () => {
    const g = groupFonts([mk('a', 'script'), mk('b', ''), mk('c', 'нет')])
    expect(g.map((x) => x.id)).toEqual(['script', 'display'])
    expect(g[1].fonts.map((f) => f.id)).toEqual(['b', 'c'])
  })
  it('«Мои шрифты» первой (082), дальше порядок спеки 046, «Декоративные» — последняя', () => {
    expect(GROUPS.map(([id, title]) => `${id}:${title}`)).toEqual([
      'my:Мои шрифты',
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

describe('loadFonts: повторная регистрация', () => {
  it('тот же список вторым вызовом не дублирует, новое начертание добавляется', () => {
    const add = vi.fn()
    vi.stubGlobal('FontFace', class {})
    vi.stubGlobal('document', { fonts: { add } })
    try {
      expect(loadFonts(fonts)).toBe(4)
      expect(loadFonts(fonts)).toBe(0)
      expect(add).toHaveBeenCalledTimes(4)
      const more = [...fonts, mk('my:0123456789abcdef', 'my', ['0123456789abcdef.ttf'])]
      expect(loadFonts(more)).toBe(1)
      expect(add).toHaveBeenCalledTimes(5)
    } finally {
      vi.unstubAllGlobals()
    }
  })
  it('отказ add не помечает начертание зарегистрированным — следующий вызов попробует снова', () => {
    const add = vi.fn().mockImplementationOnce(() => {
      throw new Error('bad')
    })
    vi.stubGlobal('FontFace', class {})
    vi.stubGlobal('document', { fonts: { add } })
    try {
      expect(loadFonts([fonts[1]])).toBe(0)
      expect(loadFonts([fonts[1]])).toBe(1)
    } finally {
      vi.unstubAllGlobals()
    }
  })
})

function mk(id: string, group: string, files: string[], extra: Partial<FontFamily> = {}): FontFamily {
  return { id, name: id, css: `SA ${id}`, group, faces: files.map((file) => ({ file, bold: false, italic: false })), ...extra }
}

describe('«Мои шрифты» (082)', () => {
  const mine = mk('my:0123456789abcdef', 'my', ['0123456789abcdef.ttf'], { name: 'Lato' })
  it('группа первая, со всеми остальными', () => {
    const g = groupFonts([...fonts, mine])
    expect(g[0]).toMatchObject({ id: 'my', title: 'Мои шрифты' })
    expect(g[0].fonts.map((f) => f.id)).toEqual([mine.id])
    expect(g.map((x) => x.id)).toEqual(['my', 'script', 'sans'])
  })
  it('нет своих — группы нет', () => expect(groupFonts(fonts).map((x) => x.id)).not.toContain('my'))
  it('скрытые не в списке, кроме текущей', () => {
    const hidden = { ...mine, hidden: true }
    expect(groupFonts([...fonts, hidden]).map((x) => x.id)).not.toContain('my')
    expect(groupFonts([...fonts, hidden], hidden.id)[0].fonts).toEqual([hidden])
    // Скрытая другая гарнитура текущей не делает.
    expect(groupFonts([...fonts, hidden], 'marck').map((x) => x.id)).not.toContain('my')
  })
  it('скрытая гарнитура остаётся в таблице: тексты ею рисуются', () => {
    const hidden = { ...mine, hidden: true }
    expect(fontCss({ font: hidden.id }, [...fonts, hidden]).family).toBe('"SA my:0123456789abcdef", "SA Montserrat", sans-serif')
  })
  it('латинский образец у шрифта без кириллицы', () => {
    expect(SAMPLE_LAT).toBe('Anna & Paul')
    expect(sampleOf({ noCyr: true })).toBe(SAMPLE_LAT)
    expect(sampleOf({ noCyr: false })).toBe(SAMPLE)
    expect(sampleOf({})).toBe(SAMPLE)
  })
  it('ownFaceFiles — файлы только своих', () => {
    expect(ownFaceFiles([...fonts, mine])).toEqual(['0123456789abcdef.ttf'])
    expect(ownFaceFiles(fonts)).toEqual([])
  })
})

// FontFace с управляемым результатом load(): имена в bad не открываются.
function stubFace(bad: string[]) {
  const opened: string[] = []
  vi.stubGlobal(
    'FontFace',
    class {
      constructor(
        public family: string,
        public src: string,
      ) {}
      async load() {
        opened.push(this.src)
        if (bad.some((b) => this.src.includes(b))) throw new Error('OTS')
        return this
      }
    },
  )
  return opened
}

describe('checkFaces (082-14)', () => {
  const a = 'aaaaaaaaaaaaaaaa.ttf'
  const b = 'bbbbbbbbbbbbbbbb.otf'
  const c = 'cccccccccccccccc.ttf'
  it('DropFonts зовётся только с отказавшими, возвращает откатанные', async () => {
    stubFace([b])
    expect(await checkFaces([a, b, c])).toEqual([b])
    expect(dropFonts).toHaveBeenCalledTimes(1)
    expect(dropFonts).toHaveBeenCalledWith([b])
    vi.unstubAllGlobals()
  })
  it('все открылись — Go не зовётся, ответ пуст', async () => {
    stubFace([])
    expect(await checkFaces([a, b])).toEqual([])
    expect(dropFonts).not.toHaveBeenCalled()
    vi.unstubAllGlobals()
  })
  it('пустой список — ничего не делает', async () => {
    expect(await checkFaces([])).toEqual([])
    expect(dropFonts).not.toHaveBeenCalled()
  })
  it('Go откатил не всё — возвращается только удалённое', async () => {
    stubFace([a, b])
    dropFonts.mockResolvedValueOnce([a])
    expect(await checkFaces([a, b])).toEqual([a])
    expect(dropFonts).toHaveBeenCalledWith([a, b])
    vi.unstubAllGlobals()
  })
  it('открывшееся помнится: второй вызов не грузит и не зовёт Go; откатанное проверяется заново', async () => {
    const opened = stubFace([b])
    await checkFaces([a, b])
    expect(opened).toHaveLength(2)
    await checkFaces([a, b])
    expect(opened).toHaveLength(3) // a — из памяти, b — снова (его откатили, файл мог вернуться)
    expect(dropFonts).toHaveBeenCalledTimes(2)
    vi.unstubAllGlobals()
  })
  it('два вызова подряд про одни файлы (загрузка и событие fonts) откатывают один раз', async () => {
    stubFace([b])
    const [x, y] = await Promise.all([checkFaces([a, b]), checkFaces([b])])
    expect(x).toEqual([b])
    expect(y).toEqual([b])
    expect(dropFonts).toHaveBeenCalledTimes(1)
    vi.unstubAllGlobals()
  })
  it('ошибка DropFonts идёт наверх, а файл потом проверяется заново', async () => {
    stubFace([b])
    dropFonts.mockRejectedValueOnce(new Error('нет метода'))
    await expect(checkFaces([b])).rejects.toThrow('нет метода')
    expect(await checkFaces([b])).toEqual([b])
    vi.unstubAllGlobals()
  })
})

describe('settleImport', () => {
  const imp = {
    added: [
      { id: 'my:1', name: 'Lato', faces: 2, noCyr: false, files: [] },
      { id: 'my:2', name: 'Ruslan', faces: 1, noCyr: false, files: [] },
    ],
    faces: ['1111111111111111.ttf', '1111111111111112.ttf', '2222222222222222.otf'],
    skipped: [{ name: 'x.woff2', why: 'веб-шрифт' }],
  }
  const list = [mk('my:1', 'my', [imp.faces[0], imp.faces[1]], { name: 'Lato' }), mk('my:2', 'my', [imp.faces[2]], { name: 'Ruslan' })]
  it('всё открылось — итог как есть', async () => {
    stubFace([])
    expect(await settleImport(imp, list)).toEqual({ ...imp, dropped: [] })
    vi.unstubAllGlobals()
  })
  it('гарнитура без единого начертания уходит из added в skipped с причиной', async () => {
    stubFace(['2222'])
    const r = await settleImport(imp, list)
    expect(r.added.map((a) => a.id)).toEqual(['my:1'])
    expect(r.skipped).toEqual([...imp.skipped, { name: 'Ruslan', why: REFUSED }])
    vi.unstubAllGlobals()
  })
  it('откатано одно начертание из двух — гарнитура остаётся, начертание в пропусках', async () => {
    stubFace(['1111111111111112'])
    const r = await settleImport(imp, list)
    expect(r.added.map((a) => a.id)).toEqual(['my:1', 'my:2'])
    expect(r.skipped.at(-1)).toEqual({ name: 'Lato', why: REFUSED })
    vi.unstubAllGlobals()
  })
  it('событие fonts ещё не пришло — откатанный файл под общим именем', async () => {
    stubFace(['2222'])
    const r = await settleImport(imp, [])
    expect(r.skipped.at(-1)).toEqual({ name: 'Шрифт', why: REFUSED })
    vi.unstubAllGlobals()
  })
  // Гонка: событие fonts → loadAndCheck → DropFonts пришло раньше ответа ImportFonts, в списке гарнитуры уже нет.
  describe('по FontAdded.files (гонка с событием fonts)', () => {
    const withFiles = {
      ...imp,
      added: [
        { ...imp.added[0], files: [imp.faces[0], imp.faces[1]] },
        { ...imp.added[1], files: [imp.faces[2]] },
      ],
    }
    it('список уже без откатанной гарнитуры — она в пропусках под своим именем, не «Шрифт»', async () => {
      stubFace(['2222'])
      const r = await settleImport(withFiles, [list[0]])
      expect(r.added.map((a) => a.id)).toEqual(['my:1'])
      expect(r.skipped).toEqual([...imp.skipped, { name: 'Ruslan', why: REFUSED }])
      vi.unstubAllGlobals()
    })
    it('геттер списка читается после проверки окном', async () => {
      stubFace(['2222'])
      let current = list
      const getter = vi.fn(() => current)
      const p = settleImport(withFiles, getter)
      expect(getter).not.toHaveBeenCalled() // до проверки список не читали
      current = [list[0]] // пока шла проверка, событие fonts уже убрало Ruslan
      const r = await p
      expect(r.added.map((a) => a.id)).toEqual(['my:1'])
      expect(r.skipped.at(-1)).toEqual({ name: 'Ruslan', why: REFUSED })
      vi.unstubAllGlobals()
    })
    it('список устарел (гарнитура ещё в нём) — результат тот же', async () => {
      stubFace(['2222'])
      const r = await settleImport(withFiles, list)
      expect(r.added.map((a) => a.id)).toEqual(['my:1'])
      expect(r.skipped.at(-1)).toEqual({ name: 'Ruslan', why: REFUSED })
      vi.unstubAllGlobals()
    })
    it('откатан один файл из двух, списка нет — гарнитура остаётся, файл под её именем', async () => {
      stubFace(['1111111111111112'])
      const r = await settleImport(withFiles, [])
      expect(r.added.map((a) => a.id)).toEqual(['my:1', 'my:2'])
      expect(r.skipped.at(-1)).toEqual({ name: 'Lato', why: REFUSED })
      vi.unstubAllGlobals()
    })
    it('откатан новый файл гарнитуры, у которой остались прежние начертания, — гарнитура остаётся', async () => {
      stubFace(['2222'])
      const old = mk('my:2', 'my', ['9999999999999999.ttf', imp.faces[2]], { name: 'Ruslan' })
      const r = await settleImport(withFiles, [list[0], old])
      expect(r.added.map((a) => a.id)).toEqual(['my:1', 'my:2'])
      expect(r.skipped.at(-1)).toEqual({ name: 'Ruslan', why: REFUSED })
      vi.unstubAllGlobals()
    })
    it('files пуст (вернулась из скрытых) и откатывать нечего — ничего не меняется', async () => {
      stubFace([])
      const back = { ...imp, added: [{ ...imp.added[0], files: [] }], faces: [] }
      expect(await settleImport(back, [])).toEqual({ ...back, dropped: [] })
    })
  })
})

describe('loadAndCheck (событие fonts)', () => {
  it('проверяет только вновь пришедшие свои начертания', async () => {
    const add = vi.fn()
    stubFace(['2222'])
    vi.stubGlobal('document', { fonts: { add } })
    const first = mk('my:1', 'my', ['1111111111111111.ttf'])
    loadAndCheck([...fonts, first])
    await vi.waitFor(() => expect(add).toHaveBeenCalled())
    expect(dropFonts).not.toHaveBeenCalled()
    loadAndCheck([...fonts, first, mk('my:2', 'my', ['2222222222222222.ttf'])])
    await vi.waitFor(() => expect(dropFonts).toHaveBeenCalledWith(['2222222222222222.ttf']))
    expect(dropFonts).toHaveBeenCalledTimes(1)
    vi.unstubAllGlobals()
  })
  it('встроенные не проверяются', async () => {
    stubFace([])
    vi.stubGlobal('document', { fonts: { add: vi.fn() } })
    loadAndCheck(fonts)
    await Promise.resolve()
    expect(dropFonts).not.toHaveBeenCalled()
    vi.unstubAllGlobals()
  })
})

describe('тексты загрузки и отказа', () => {
  it('noCyrTail называет гарнитуры без кириллицы', () => {
    const a = (name: string, noCyr: boolean) => ({ id: name, name, faces: 1, noCyr, files: [] })
    expect(noCyrTail([a('Lato', false)])).toBe('')
    expect(noCyrTail([])).toBe('')
    expect(noCyrTail([a('Lato', false), a('Lato Script', true), a('Zeta', true)])).toBe('нет русских букв: Lato Script, Zeta')
  })
  it('failedText: тост по своей гарнитуре один раз за сеанс', () => {
    const my = mk('my:1', 'my', ['1111111111111111.ttf'], { name: 'Lato', css: 'SA my 1' })
    const told = new Set<string>()
    const text = failedText('SA my 1', [...fonts, my], told)
    expect(text).toBe(
      'Шрифт «Lato» не открылся в окне программы: на холсте вместо него Montserrat. Уберите его (×) и загрузите файл заново.',
    )
    expect(failedText('SA my 1', [...fonts, my], told)).toBe('')
    expect(failedText('"SA my 1"', [...fonts, my], new Set())).toBe(text) // имя в кавычках
  })
  it('failedText: встроенная и неизвестная гарнитуры молчат', () => {
    expect(failedText('SA Montserrat', fonts, new Set())).toBe('')
    expect(failedText('SA my 9', fonts, new Set())).toBe('')
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
    expect(svgTextCss({ ...st, font: 'evil' }, fonts)).toContain('font-family:"SA Montserrat", sans-serif')
  })
})
