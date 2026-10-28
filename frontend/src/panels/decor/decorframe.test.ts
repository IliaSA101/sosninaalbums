import { beforeEach, describe, expect, it, vi } from 'vitest'

// Окно «Рамочки» (079 U1): состояние, «Куда», постановка. Go подменён моками биндингов.
const api = vi.hoisted(() => ({
  DecorCatalog: vi.fn(),
  ApplyBorder: vi.fn(),
  ImportDecor: vi.fn(),
  AddDecorPNG: vi.fn(),
  RenameDecor: vi.fn(),
  HideDecor: vi.fn(),
  GetView: vi.fn(),
}))
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', () => api)
vi.mock('../../editor/text/fonts', () => ({ loadFonts: async () => {} }))
vi.stubGlobal('window', globalThis)

const { decor } = await import('./decor.svelte')
const { store } = await import('../../base/core/store.svelte')
const { placesel } = await import('../../editor/places/placesel.svelte')

const place = (id: string, photo: string, preset = '') => ({
  id,
  photo,
  fit: '',
  spine: false,
  z: 0,
  x: 10,
  y: 10,
  w: 50,
  h: 50,
  rot: 0,
  border: preset
    ? {
        kind: '',
        photo: '',
        lib: '',
        preset,
        name: preset,
        inks: [],
        width: 0,
        scale: 0,
        ins: [0, 0, 0, 0],
        url: '/u',
        maskUrl: '',
        pic: false,
        open: false,
        known: true,
      }
    : undefined,
})
let rev = 1
const sheet = (id: string, over: object) => ({
  id,
  photos: [],
  rects: [],
  w: 600,
  h: 300,
  mid: 300,
  frames: [],
  texts: [],
  captions: [],
  bg: ['#fff', '#fff'],
  bgSet: ['', ''],
  ...over,
})
const grid = (presets: Record<string, string> = {}, places = ['p1', 'p2', 'p3']) =>
  ({
    rev: ++rev,
    folders: [],
    settings: { pageW: 300, pageH: 300 },
    spreads: [
      sheet('s1', {
        grid: true,
        photos: ['a', 'b'],
        places: places.map((id, i) => place(id, ['a', 'b', ''][i] ?? '', presets[id])),
      }),
      sheet('s2', { grid: false, photos: ['c', 'd'], places: null }),
    ],
  }) as never

const tile = (id: string, name: string) => ({ id, theme: 'basic', name, url: '/u', own: false, aspect: 1, w: 0, h: 0 })
const white = tile('sa:fr-white-line', 'Белая линия')
const black = tile('sa:fr-black-thin', 'Тонкая чёрная')
const gold = tile('sa:fr-gold', 'Золотая рамка')
const catalog = (readOnly = false) => ({
  kind: 'frame',
  themes: [{ id: 'all', name: 'Все', count: 3 }],
  items: [white, black, gold],
  recent: [],
  readOnly,
})

beforeEach(() => {
  vi.resetAllMocks()
  decor.close()
  store.toast = null
  api.DecorCatalog.mockImplementation(async () => catalog())
  api.ApplyBorder.mockImplementation(async () => grid())
  store.apply(grid())
  store.go(0)
  placesel.clear()
})

const open = async () => {
  decor.show('frame', 's1', 'spread')
  await vi.waitFor(() => expect(decor.cat).not.toBeNull())
  await vi.waitFor(() => expect(decor.loading).toBe(false))
}

describe('окно «Рамочки»: открытие и «Куда»', () => {
  it('каталог — DecorCatalog("frame"), тема «Все» (Недавние пусты)', async () => {
    await open()
    expect(api.DecorCatalog).toHaveBeenCalledWith('frame')
    expect(decor.cat?.kind).toBe('frame')
    expect(decor.cat?.items).toHaveLength(3)
    expect(decor.theme).toBe('all')
  })

  it('без выделения — «Все места разворота», места листа вместе с пустым', async () => {
    await open()
    expect(decor.frameSel()).toEqual([])
    expect(decor.frameWhereNow()).toBe('all')
    expect(decor.frameIds()).toEqual(['p1', 'p2', 'p3'])
  })

  it('с выделением — «Выделенные»; можно переключить на весь лист и обратно', async () => {
    placesel.select(['p2'])
    await open()
    expect(decor.frameWhereNow()).toBe('sel')
    expect(decor.frameIds()).toEqual(['p2'])
    decor.setFrameWhere('all')
    expect(decor.frameIds()).toEqual(['p1', 'p2', 'p3'])
    decor.setFrameWhere('sel')
    expect(decor.frameIds()).toEqual(['p2'])
  })

  it('повторное открытие снова выбирает по выделению, прежний выбор не помнится', async () => {
    placesel.select(['p1'])
    await open()
    decor.setFrameWhere('all')
    decor.close()
    await open()
    expect(decor.frameWhereNow()).toBe('sel')
  })

  it('выделение пропало, пока окно открыто, — «Все места»', async () => {
    placesel.select(['p1'])
    await open()
    placesel.clear()
    expect(decor.frameWhereNow()).toBe('all')
    expect(decor.frameIds()).toEqual(['p1', 'p2', 'p3'])
  })

  it('«Авто»: все места — фото разворота', async () => {
    store.go(1)
    decor.show('frame', 's2', 'spread')
    expect(decor.frameIds()).toEqual(['c', 'd'])
  })
})

describe('щелчок по плитке', () => {
  it('на выделенные: ApplyBorder(лист, места, плитка), окно закрыто, вид обновлён', async () => {
    api.ApplyBorder.mockImplementation(async () => grid({ p2: white.id }))
    placesel.select(['p2'])
    await open()
    const before = store.view.rev
    await decor.pick(white)
    expect(api.ApplyBorder).toHaveBeenCalledWith('s1', ['p2'], white.id)
    expect(decor.cur).toBeNull()
    expect(store.view.spreads[0].places!.map((p) => p.border?.preset)).toEqual([undefined, white.id, undefined])
    expect(store.view.rev).toBeGreaterThan(before)
  })

  it('на все места, включая пустое: уходят ID всех мест листа', async () => {
    await open()
    await decor.pick(black)
    expect(api.ApplyBorder).toHaveBeenCalledWith('s1', ['p1', 'p2', 'p3'], black.id)
  })

  it('«Авто»: уходят ID фото разворота', async () => {
    store.go(1)
    decor.show('frame', 's2', 'spread')
    await vi.waitFor(() => expect(decor.loading).toBe(false))
    await decor.pick(gold)
    expect(api.ApplyBorder).toHaveBeenCalledWith('s2', ['c', 'd'], gold.id)
  })

  it('та же уже везде — Go вернул тот же вид: версия не растёт, тоста нет', async () => {
    const same = grid({ p1: white.id, p2: white.id, p3: white.id })
    store.apply(same)
    api.ApplyBorder.mockImplementation(async () => same)
    await open()
    const before = store.view.rev
    await decor.pick(white)
    expect(store.view.rev).toBe(before)
    expect(store.toast).toBeNull()
  })

  it('нет мест на листе — подсказка, Go не зовётся, окно остаётся открытым', async () => {
    store.apply(grid({}, []))
    await open()
    await decor.pick(white)
    expect(api.ApplyBorder).not.toHaveBeenCalled()
    expect(decor.cur).not.toBeNull()
    expect(store.toast?.text).toBe('На этом листе нет мест, куда поставить рамочку')
  })

  it('ошибка Go: тост с текстом, окно уже закрыто', async () => {
    api.ApplyBorder.mockRejectedValue(new Error('нет такой рамочки'))
    await open()
    await decor.pick(gold)
    expect(decor.cur).toBeNull()
    expect(store.toast?.text).toMatch(/нет такой рамочки/i)
  })
})

describe('«Загрузить своё…»', () => {
  it('зовёт ImportDecor("frame", "mine") и перечитывает каталог', async () => {
    api.ImportDecor.mockResolvedValue({ added: [{ id: 'my:1' }], skipped: [] })
    await open()
    const reads = api.DecorCatalog.mock.calls.length
    await decor.upload()
    expect(api.ImportDecor).toHaveBeenCalledWith('frame', 'mine')
    expect(api.DecorCatalog.mock.calls.length).toBe(reads + 1)
    expect(decor.busy).toBe(false)
  })

  it('библиотека только для чтения — Go не зовётся', async () => {
    api.DecorCatalog.mockImplementation(async () => catalog(true))
    await open()
    await decor.upload()
    expect(api.ImportDecor).not.toHaveBeenCalled()
  })

  it('ошибка Go — тост, кнопка снова свободна', async () => {
    api.ImportDecor.mockRejectedValue('библиотека недоступна')
    await open()
    await decor.upload()
    expect(store.toast?.text).toMatch(/библиотека недоступна/i)
    expect(decor.busy).toBe(false)
  })
})
