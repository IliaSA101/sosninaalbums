import { describe, expect, it, vi } from 'vitest'

// Плитки «Можно скачать» (144): тексты по видам, вопрос, тосты, выборка «Можно скачать» в темах. Чистое, без Go.
vi.mock('../../../wailsjs/go/app/App', () => ({}))
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))

const {
  ASK_TITLE,
  GET,
  askOk,
  getItems,
  getName,
  getNote,
  getQuestion,
  getRatio,
  getTitle,
  isGetItem,
  isNoItem,
  noItemText,
  otherWhereText,
  sampleUrl,
} = await import('./decorget')
const { NEW, READ_ONLY_HINT, SIDE_TIPS, firstSource, isSelection, noOwn, sideItems, themeItems } = await import('../decor/decor')
const { NO_SEL, browse, sideCounts } = await import('./browse')
const { getBoxFor, tplSideItems, tplVisible } = await import('../decor/spreadtpl')
const { getOffHint } = await import('./getoff')

const MB = 1048576
const tile = (o: Record<string, unknown> = {}) => ({
  id: 'get:school:0123456789abcdef',
  pack: 'school',
  src: '0123456789abcdef',
  title: 'Школа — осень',
  url: '/media/lp/school/bg/0',
  w: 240,
  h: 160,
  bytes: 38 * MB,
  with: [],
  bg: 7,
  sticker: 62,
  spread: 0,
  frame: 3,
  ...o,
})
const item = (id: string, o: Record<string, unknown> = {}) => ({
  id,
  theme: 'basic',
  name: id,
  url: '/media/gen/art',
  own: false,
  aspect: 0,
  w: 0,
  h: 0,
  packId: 'sa:builtin',
  pack: 'Встроенные',
  ...o,
})
const cat = (o: Record<string, unknown> = {}) => ({
  kind: 'bg' as const,
  themes: [
    { id: 'all', name: 'Все', count: 2 },
    { id: 'basic', name: 'Базовые', count: 2 },
    { id: GET, name: 'Можно скачать', count: 2 },
  ],
  items: [item('sa:a'), item('sa:b')],
  recent: [],
  new: [],
  readOnly: false,
  get: [tile(), tile({ id: 'get:school:fedcba9876543210', src: 'fedcba9876543210' })],
  ...o,
})

describe('подпись и подсказка плитки', () => {
  it('подпись «⤓ Название» и размер отдельно (TplItem склеит « · »)', () => {
    const g = tile()
    expect(getName(g)).toBe('⤓ Школа — осень')
    expect(getNote(g)).toBe('38,0 МБ')
    expect(getNote(tile({ bytes: 300 * 1024 }))).toBe('300 КБ')
  })

  it.each([
    ['bg', 'Скачать набор «Школа — осень» (38,0 МБ) и поставить этот фон'],
    ['sticker', 'Скачать набор «Школа — осень» (38,0 МБ) и поставить этот стикер'],
    ['frame', 'Скачать набор «Школа — осень» (38,0 МБ) и поставить эту рамочку'],
    ['spread', 'Скачать набор «Школа — осень» (38,0 МБ) и поставить этот разворот'],
    ['album', 'Скачать набор «Школа — осень» (38,0 МБ) и показать этот макет'],
  ] as const)('подсказка вида %s', (kind, text) => {
    expect(getTitle(tile(), kind)).toBe(text)
  })

  it('пропорция: стикер и рамочка — квадрат, остальным — box стоящей плитки, нет его — клетка образца', () => {
    expect(getRatio(tile(), 'bg', 2)).toBe('2')
    expect(getRatio(tile(), 'bg')).toBe('240 / 160')
    expect(getRatio(tile(), 'sticker')).toBe('1 / 1')
    expect(getRatio(tile(), 'frame', 2)).toBe('1 / 1')
    expect(getRatio(tile({ w: 240, h: 120 }), 'spread')).toBe('240 / 120')
    expect(getRatio(tile({ w: 240, h: 120 }), 'spread', 0.5)).toBe('0.5')
    expect(getRatio(tile(), 'album', 1.4)).toBe('1.4')
    expect(getRatio(tile({ w: 0, h: 0 }), 'album')).toBe('3 / 2')
  })

  it('box разворота: лист при «Весь разворот», страница при «Левая/Правая»; фон — лист; стикер — нет', () => {
    const sp = { w: 600, h: 300, mid: 300 }
    expect(getBoxFor('spread', sp, 'spread')).toBe(2)
    expect(getBoxFor('spread', sp, 'left')).toBe(1)
    expect(getBoxFor('spread', { w: 600, h: 300, mid: 0 }, 'left')).toBe(2) // лист без шва — одна «страница»
    expect(getBoxFor('bg', sp, 'left')).toBe(2)
    expect(getBoxFor('sticker', sp, 'spread')).toBeUndefined()
  })

  it('причина серости: ход или чужой ход — «обновляется», только чтение — подсказка, иначе пусто', () => {
    expect(getOffHint({ updating: true, progress: null }, false)).toBe('Библиотека обновляется')
    expect(getOffHint({ updating: false, progress: { stage: 'catalog' } }, true)).toBe('Библиотека обновляется')
    expect(getOffHint({ updating: false, progress: null }, true)).toBe(READ_ONLY_HINT)
    expect(getOffHint({ updating: false, progress: null }, false)).toBe('')
  })

  it('адрес клетки: выход из префикса (.., %2e, \\, //) — пусто', () => {
    for (const bad of [
      '/media/lp/../gen/art',
      '/media/lp/school/../../x',
      '/media/lp/%2e%2e/x',
      '/media/lp/%2E%2e/x',
      '/media/lp/school\\x',
      '/media/lp//evil.example/x',
      '/media/lp/school//0',
    ])
      expect(sampleUrl(bad), bad).toBe('')
  })

  it('адрес клетки — только /media/lp/', () => {
    expect(sampleUrl('/media/lp/school/bg/0')).toBe('/media/lp/school/bg/0')
    expect(sampleUrl('https://evil.example/x.png')).toBe('')
    expect(sampleUrl('/media/lib/x')).toBe('')
  })
})

describe('вопрос «Скачать набор?»', () => {
  it('состав набора, размер всего скачиваемого и что встанет', () => {
    expect(ASK_TITLE).toBe('Скачать набор?')
    expect(getQuestion(tile(), 'bg')).toBe(
      'В наборе «Школа — осень» 7 фонов, 62 стикера, 3 рамочки — 38,0 МБ. Этот фон встанет сразу после скачивания.',
    )
  })

  it('с дополнениями: «Вместе с ним»', () => {
    expect(getQuestion(tile({ with: ['Школа — осень: макеты', 'Рамки'] }), 'sticker')).toBe(
      'В наборе «Школа — осень» 7 фонов, 62 стикера, 3 рамочки — 38,0 МБ. Вместе с ним: «Школа — осень: макеты», «Рамки». Этот стикер встанет сразу после скачивания.',
    )
  })

  it('макет: «Макет откроется в этом окне», кнопка «Скачать и показать»', () => {
    expect(getQuestion(tile({ bg: 0, sticker: 0, frame: 0, spread: 1 }), 'album')).toBe(
      'В наборе «Школа — осень» 1 разворот — 38,0 МБ. Макет откроется в этом окне.',
    )
    expect(askOk('album')).toBe('Скачать и показать')
    expect(askOk('bg')).toBe('Скачать и поставить')
  })

  it('состав неизвестен — только размер', () => {
    expect(getQuestion(tile({ bg: 0, sticker: 0, frame: 0 }), 'frame')).toBe(
      'В наборе «Школа — осень» — 38,0 МБ. Эта рамочка встанет сразу после скачивания.',
    )
  })
})

describe('тосты, когда скачанное не ставится само (п. 6)', () => {
  it('разворот для другого «Куда»', () => {
    expect(otherWhereText('Школа — осень')).toBe('Набор «Школа — осень» скачан — этот разворот для другого «Куда», он в «Новых»')
  })
  it.each([
    ['bg', 'Набор «Школа — осень» скачан, но этого фона в нём нет — выберите другой'],
    ['sticker', 'Набор «Школа — осень» скачан, но этого стикера в нём нет — выберите другой'],
    ['frame', 'Набор «Школа — осень» скачан, но этой рамочки в нём нет — выберите другой'],
    ['spread', 'Набор «Школа — осень» скачан, но этого разворота в нём нет — выберите другой'],
    ['album', 'Набор «Школа — осень» скачан, но этого макета в нём нет — выберите другой'],
  ] as const)('предмета нет: %s', (kind, text) => {
    expect(noItemText('Школа — осень', kind)).toBe(text)
  })
  it('ошибку «предмета нет» узнаём по тексту Go, прочее — нет', () => {
    expect(isNoItem('этого предмета в наборе нет — выберите другой')).toBe(true)
    expect(isNoItem(new Error('Этого предмета в наборе нет — выберите другой'))).toBe(true)
    expect(isNoItem('нет связи')).toBe(false)
  })
})

describe('выборка «Можно скачать» в темах', () => {
  it('themeItems(cat, "get") — плитки образцов в порядке каталога; «Все» и темы их не содержат', () => {
    const c = cat()
    const got = themeItems(c, GET)
    expect(got.map((i) => i.id)).toEqual(['get:school:0123456789abcdef', 'get:school:fedcba9876543210'])
    expect(got.every(isGetItem)).toBe(true)
    expect(themeItems(c, 'all').map((i) => i.id)).toEqual(['sa:a', 'sa:b'])
    expect(themeItems(c, 'basic').some(isGetItem)).toBe(false)
    expect(themeItems(c, NEW)).toEqual([])
  })

  it('плитка — предмет каталога по форме: набор для ряда «Набор», название только текстом', () => {
    const [t] = getItems(cat({ get: [tile({ title: '<img src=x onerror=alert(1)>' })] }))
    expect(t).toMatchObject({
      theme: GET,
      own: false,
      packId: 'school',
      pack: '<img src=x onerror=alert(1)>',
      name: '<img src=x onerror=alert(1)>',
    })
    expect(isGetItem(item('sa:a'))).toBe(false)
  })

  it('старый Go без поля get — плиток нет; get: null — тоже', () => {
    const c = cat()
    delete (c as { get?: unknown }).get
    expect(getItems(c)).toEqual([])
    expect(getItems({ get: null as never })).toEqual([])
    expect(getItems(null)).toEqual([])
    expect(themeItems(c, GET)).toEqual([])
  })

  it('мусорная строка (нет id или набора) пропускается, with — не null', () => {
    const got = getItems(cat({ get: [tile({ id: undefined }), tile({ pack: undefined }), tile({ with: undefined })] }))
    expect(got).toHaveLength(1)
    expect(got[0].get.with).toEqual([])
  })

  it('это выборка: своё в неё не кладётся, подсказка строки есть', () => {
    expect(isSelection(GET)).toBe(true)
    expect(noOwn(GET)).toBe(true)
    expect(SIDE_TIPS[GET]).toBe('Образцы из наборов, которых ещё нет в библиотеке: щелчок скачает набор и поставит выбранное')
    const side = sideItems(cat())
    expect(side.at(-1)).toMatchObject({ id: GET, label: 'Можно скачать', count: 2, title: SIDE_TIPS[GET] })
  })

  it('число у строки считает тот же отбор «Набор»: выбран другой набор — плиток нет', () => {
    const c = cat({ get: [tile(), tile({ id: 'get:b:0123456789abcdef', pack: 'b', title: 'Б' })] })
    expect(sideCounts(c, NO_SEL).at(-1)?.count).toBe(2)
    expect(sideCounts(c, { ...NO_SEL, pack: 'b' }).at(-1)?.count).toBe(1)
    expect(browse(themeItems(c, GET), c.items, { ...NO_SEL, pack: 'school' }).items.map((i) => i.id)).toEqual([
      'get:school:0123456789abcdef',
    ])
    expect(browse(themeItems(c, GET), c.items, NO_SEL).packs.map((p) => p.label)).toEqual(['Школа — осень', 'Б'])
  })

  it('«Куда» у разворотов плитки скачивания не отбирает (счёт и показ)', () => {
    const sp = { mid: 300 }
    const c = cat({ kind: 'spread' })
    expect(tplVisible(themeItems(c, GET), 'left', sp)).toHaveLength(2)
    expect(tplSideItems(c, 'left', sp).at(-1)?.count).toBe(2)
    expect(tplVisible([item('sa:a')], 'left', sp)).toHaveLength(0)
  })

  it('выбранная строка пропала (всё скачано) — окно встаёт на «Все»', () => {
    const c = cat({ get: [], themes: [{ id: 'all', name: 'Все', count: 2 }] })
    expect(firstSource(c, GET)).toBe('all')
    expect(firstSource(cat(), GET)).toBe(GET)
  })
})
