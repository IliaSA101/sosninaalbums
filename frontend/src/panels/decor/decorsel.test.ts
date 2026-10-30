import { beforeEach, describe, expect, it, vi } from 'vitest'

// 133-10: выборки (isSelection / noOwn), своё из «Новых» и «Фирменных» → «Мои», themeItems('new'), firstSource, прыжок на «Новые»
// для всех четырёх видов окна. Go подменён.
const api = vi.hoisted(() => ({ DecorCatalog: vi.fn(), ImportDecor: vi.fn() }))
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', () => api)
vi.mock('../../editor/text/fonts', () => ({ loadFonts: async () => {} }))
vi.stubGlobal('window', globalThis)

const { decor } = await import('./decor.svelte')
const { store } = await import('../../base/core/store.svelte')
const { ALL, MINE, NEW, RECENT, firstSource, freshSource, importTheme, isSelection, noOwn, sideItems, themeItems } = await import('./decor')
const { saveTheme, saveThemes } = await import('./spreadtpl')
type Cat = import('./decor').DecorCatalog
type Item = import('./decor').DecorItem

const item = (id: string, theme: string, over: Partial<Item> = {}): Item => ({
  id,
  theme,
  name: id,
  url: `/media/lib/t/${id}`,
  own: false,
  aspect: 0,
  w: 0,
  h: 0,
  ...over,
})
// Каталог: новая «Айвори» (pk:1 в теме ivory, pk:2 в basic), старая «Школа», встроенное, своё
const cat = (over: Partial<Cat> = {}): Cat => ({
  kind: 'sticker',
  themes: [
    { id: NEW, name: 'Новые', count: 2 },
    { id: RECENT, name: 'Недавние', count: 1 },
    { id: ALL, name: 'Все', count: 5 },
    { id: 'basic', name: 'Универсальные', count: 2 },
    { id: 'ivory', name: 'Айвори', count: 1 },
    { id: 'brand', name: 'Фирменные', count: 1 },
    { id: MINE, name: 'Мои', count: 1 },
  ],
  items: [
    item('pk:1', 'ivory', { pack: 'Айвори', packId: 'ivory' }),
    item('pk:2', 'basic', { pack: 'Айвори', packId: 'ivory' }),
    item('pk:3', 'basic', { pack: 'Школа', packId: 'school' }),
    item('sa:b', 'brand', { packId: 'sa:builtin' }),
    item('my:0123456789abcdef', MINE),
  ],
  recent: ['sa:b', 'gone'],
  new: ['pk:2', 'pk:1', 'gone'],
  readOnly: false,
  ...over,
})

describe('выборки', () => {
  it('isSelection: «Все», «Недавние», «Новые»; темы и «Фирменные» — нет', () => {
    for (const id of [ALL, RECENT, NEW]) expect(isSelection(id)).toBe(true)
    for (const id of ['basic', MINE, 'brand', '', 'newish']) expect(isSelection(id)).toBe(false)
  })

  it('noOwn: выборки и «Фирменные» — куда своё не кладётся', () => {
    for (const id of [ALL, RECENT, NEW, 'brand']) expect(noOwn(id)).toBe(true)
    for (const id of ['basic', MINE, 'wedding']) expect(noOwn(id)).toBe(false)
  })

  it('своё из «Новых», «Недавних», «Все» и «Фирменных» ложится в «Мои», из темы — в тему', () => {
    for (const id of [NEW, RECENT, ALL, 'brand']) expect(importTheme(id)).toBe(MINE)
    expect(importTheme('wedding')).toBe('wedding')
    expect(importTheme(MINE)).toBe(MINE)
  })

  it('«Сохранить разворот…»: темы без выборок и «Фирменных», по умолчанию «Мои»', () => {
    expect(saveThemes(cat()).map((t) => t.id)).toEqual(['basic', 'ivory', MINE])
    for (const id of [NEW, RECENT, ALL, 'brand', '']) expect(saveTheme(id)).toBe('mine')
    expect(saveTheme('ivory')).toBe('ivory')
  })

  it('«Загрузить своё…» из «Новых» и «Фирменных»: Go получает «Мои», окно встаёт на «Мои»', async () => {
    store.apply({ rev: 1, folders: [], settings: { pageW: 300, pageH: 300 }, spreads: [{ id: 's1', w: 600, h: 300, mid: 300 }] } as never)
    for (const from of [NEW, 'brand']) {
      api.DecorCatalog.mockImplementation(async () => cat())
      api.ImportDecor.mockResolvedValue({ added: [item('my:1', MINE)], skipped: [] })
      decor.show('sticker', 's1', 'spread')
      await vi.waitFor(() => expect(decor.loading).toBe(false))
      decor.setTheme(from)
      await decor.upload()
      expect(api.ImportDecor).toHaveBeenLastCalledWith('sticker', MINE)
      expect(decor.theme).toBe(MINE)
      decor.close()
    }
  })

  it('после «Добавить свой…» отбор «Все»: набор, стиль, «Уголки» прячут свои (0.28-133-77); ничего не добавили — отбор остаётся', async () => {
    store.apply({ rev: 1, folders: [], settings: { pageW: 300, pageH: 300 }, spreads: [{ id: 's1', w: 600, h: 300, mid: 300 }] } as never)
    api.DecorCatalog.mockImplementation(async () => cat())
    decor.show('sticker', 's1', 'spread')
    await vi.waitFor(() => expect(decor.loading).toBe(false))
    const want = { pack: 'school', form: 'corner', style: 'Значки' } as const
    decor.filters = { ...want }
    api.ImportDecor.mockResolvedValue({ added: [], skipped: [{ name: 'x.png', why: 'не картинка' }] })
    await decor.upload()
    expect(decor.filters).toEqual(want)
    api.ImportDecor.mockResolvedValue({ added: [item('my:1', MINE)], skipped: [] })
    await decor.upload()
    expect(decor.filters).toEqual({ pack: '', form: '', style: '' })
    decor.close()
  })

  it('themeItems(«Новые») — предметы по cat.new в порядке каталога, пропавшие пропускаются; нет поля — пусто', () => {
    expect(themeItems(cat(), NEW).map((i) => i.id)).toEqual(['pk:2', 'pk:1'])
    expect(themeItems({ ...cat(), new: undefined }, NEW)).toEqual([])
    expect(themeItems(null, NEW)).toEqual([])
    expect(themeItems(cat(), RECENT).map((i) => i.id)).toEqual(['sa:b']) // «Недавние» по-прежнему по своему списку
  })

  it('themeItems(«Фирменные») — по теме предметов, а не выборка', () => {
    expect(themeItems(cat(), 'brand').map((i) => i.id)).toEqual(['sa:b'])
  })

  it('строки слева: «Новые» и «Недавние» с подсказкой, остальные без', () => {
    const rows = sideItems(cat())
    expect(rows[0]).toMatchObject({
      id: NEW,
      label: 'Новые',
      title: 'Из наборов, поставленных за последние 30 дней (кроме первой установки библиотеки)',
    })
    expect(rows[1].title).toBe('Что вы недавно ставили')
    expect(rows[2].title).toBeUndefined()
  })
})

describe('firstSource', () => {
  it('«Новые» — только при плитке свежего набора в cat.new', () => {
    expect(firstSource(cat(), '', new Set(['Айвори']))).toBe(NEW)
    expect(firstSource(cat(), 'basic', new Set(['Айвори']))).toBe(NEW) // свежее главнее прежнего выбора
  })

  it('свежий набор не новый (обновление): тема его первой плитки, как в 073-1', () => {
    // «Школа» поставлена, но в cat.new её нет
    expect(firstSource(cat(), '', new Set(['Школа']))).toBe('basic')
    expect(freshSource(cat(), new Set(['Школа']))).toBe('basic')
  })

  it('чужие свежие не уводят на «Новые» с чужим содержимым', () => {
    expect(firstSource(cat(), 'basic', new Set(['Нет такого']))).toBe('basic')
    expect(firstSource(cat(), '', new Set(['Нет такого']))).toBe(RECENT)
    expect(freshSource(cat(), new Set(['Нет такого']))).toBe('')
  })

  it('строки «Новые» нет в списке тем — на неё не прыгаем', () => {
    const noNew = cat({ themes: cat().themes.slice(1) })
    expect(firstSource(noNew, '', new Set(['Айвори']))).toBe('ivory')
  })

  it('без свежих — прежнее: выбор, «Недавние», «Все»; «Новые» по умолчанию не открываются', () => {
    expect(firstSource(cat())).toBe(RECENT)
    expect(firstSource(cat(), NEW)).toBe(NEW) // прежний выбор, если он есть
    expect(firstSource(cat(), 'исчезла')).toBe(RECENT)
    expect(firstSource(cat({ themes: cat().themes.filter((t) => t.id !== RECENT) }))).toBe(ALL)
    expect(firstSource(null, '', new Set(['Айвори']))).toBe(ALL)
  })
})

describe('прыжок на «Новые» при открытии (takeFresh)', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    decor.close()
    decor.cat = null
    decor.setTheme('')
    decor.setFilter('pack', '')
    api.DecorCatalog.mockImplementation(async (kind: string) => cat({ kind: kind as Cat['kind'] }))
    store.apply({ rev: 1, folders: [], settings: { pageW: 300, pageH: 300 }, spreads: [{ id: 's1', w: 600, h: 300, mid: 300 }] } as never)
  })
  const open = async (kind: 'bg' | 'sticker' | 'spread' | 'frame') => {
    decor.close()
    decor.setTheme('basic')
    decor.show(kind, 's1', 'spread')
    await vi.waitFor(() => expect(decor.loading).toBe(false))
    return decor.theme
  }

  it('все четыре вида получают «Новые» по разу; потом забыто', async () => {
    decor.note(['Айвори'])
    for (const kind of ['bg', 'sticker', 'spread', 'frame'] as const) expect(await open(kind), kind).toBe(NEW)
    for (const kind of ['bg', 'sticker', 'spread', 'frame'] as const) expect(await open(kind), kind).toBe(RECENT) // сменили вид — тема сброшена, не «Новые»
  })

  it('четвёртый вид получает своё и после трёх прежних (до 133 забывалось после трёх)', async () => {
    decor.note(['Айвори'])
    for (const kind of ['bg', 'sticker', 'spread'] as const) await open(kind)
    expect(await open('frame')).toBe(NEW)
  })

  it('вид открывали повторно — прыжка нет; новый вопрос note даёт его заново', async () => {
    decor.note(['Айвори'])
    expect(await open('bg')).toBe(NEW)
    expect(await open('bg')).toBe('basic')
    decor.note(['Айвори'])
    expect(await open('bg')).toBe(NEW)
  })

  it('прыжок снимает старый отбор, чтобы он не спрятал свежий набор', async () => {
    decor.note(['Айвори'])
    decor.setFilter('pack', 'school')
    await open('bg')
    expect(decor.theme).toBe(NEW)
    expect(decor.filters.pack).toBe('')
  })
})
