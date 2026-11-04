import { beforeEach, describe, expect, it, vi } from 'vitest'

// Раздел «Рамочка» и «Прозрачность» карточки (079 U3): правки идут через frameApi в SetBorder / ApplyBorder / SetPhotoFade.
// Проверяем, что и с какими merge уходит в Go.
const api = vi.hoisted(() => ({
  DecorCatalog: vi.fn(),
  GetView: vi.fn(),
  SetBorder: vi.fn(),
  SetPhotoFade: vi.fn(),
  ApplyBorder: vi.fn(),
}))
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', () => api)
vi.mock('../text/fonts', () => ({ loadFonts: async () => {} }))
vi.stubGlobal('window', globalThis)

const { store } = await import('../../base/core/store.svelte')
const { placesel } = await import('../places/placesel.svelte')
const { decor } = await import('../../panels/decor/decor.svelte')
const { borderAct } = await import('./borderact.svelte')
const { borderlive } = await import('./borderlive.svelte')
const { borderRows, targetsOf } = await import('./borderrows')

const border = (over: object = {}) => ({
  kind: 'line',
  photo: '',
  lib: '',
  preset: 'sa:fr-white-line',
  name: 'Белая линия',
  inks: ['#ffffff'],
  width: 1.5,
  defWidth: 1.5,
  scale: 0,
  ins: [1.5, 1.5, 1.5, 1.5],
  url: '/u',
  maskUrl: '',
  pic: false,
  open: false,
  known: true,
  ...over,
})
const place = (id: string, photo: string, b?: object) => ({
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
  border: b,
})
const fv = (fade?: number) => ({ s: 1, x: 0, y: 0, r: 0, img: { x: 10, y: 10, w: 50, h: 50 }, fade })
let rev = 1
const make = (fades: [number?, number?] = []) =>
  ({
    rev: ++rev,
    folders: [],
    settings: { pageW: 300, pageH: 300 },
    spreads: [
      {
        id: 's1',
        grid: true,
        photos: ['a', 'b'],
        rects: [],
        w: 600,
        h: 300,
        mid: 300,
        frames: [fv(fades[0]), fv(fades[1])],
        texts: [],
        captions: [],
        bg: ['#fff', '#fff'],
        bgSet: ['', ''],
        places: [
          place('p1', 'a', border()),
          place('p2', 'b', border({ preset: '', name: 'Линия', width: 4, ins: [4, 4, 4, 4] })),
          place('p3', ''),
        ],
      },
      {
        id: 's2',
        grid: false,
        photos: ['c'],
        rects: [],
        w: 600,
        h: 300,
        mid: 300,
        frames: [fv()],
        texts: [],
        captions: [],
        bg: ['#fff', '#fff'],
        bgSet: ['', ''],
        places: null,
      },
    ],
  }) as never

const sel = () =>
  borderRows(
    targetsOf(
      store.spread,
      placesel.selected.map((b) => b.id),
    ),
  )

beforeEach(() => {
  vi.clearAllMocks()
  api.SetBorder.mockImplementation(async () => make())
  api.SetPhotoFade.mockImplementation(async () => make())
  api.ApplyBorder.mockImplementation(async () => make())
  borderAct.end()
  store.toast = null
  store.apply(make())
  store.go(0)
  placesel.clear()
})

describe('вызовы Go: SetBorder / ApplyBorder / SetPhotoFade', () => {
  it('толщина — SetBorder(лист, места, {width}, merge); merge один на серию, новый после ухода фокуса', async () => {
    await borderAct.series('s1', 'width', ['p1', 'p2'], { width: 3 })
    await borderAct.series('s1', 'width', ['p1', 'p2'], { width: 3.1 })
    const m1 = api.SetBorder.mock.calls.map((c) => c[3])
    expect(m1[0]).not.toBe('')
    expect(m1[1]).toBe(m1[0])
    expect(api.SetBorder.mock.calls[0].slice(0, 3)).toEqual(['s1', ['p1', 'p2'], { width: 3 }])
    borderAct.session.end()
    await borderAct.series('s1', 'width', ['p1', 'p2'], { width: 3.2 })
    const m2 = api.SetBorder.mock.calls[2][3]
    expect(m2).not.toBe('')
    expect(m2).not.toBe(m1[0])
  })

  it('другая строка (размер) или другое выделение — другая серия', async () => {
    await borderAct.series('s1', 'width', ['p1'], { width: 3 })
    await borderAct.series('s1', 'size', ['p1'], { scale: 1.2 })
    await borderAct.series('s1', 'width', ['p2'], { width: 3 })
    const m = api.SetBorder.mock.calls.map((c) => c[3])
    expect(new Set(m).size).toBe(3)
  })

  it('цвет — merge пустой: шаг на выбор', async () => {
    await borderAct.ink('s1', ['p1'], { inks: ['#112233'] })
    expect(api.SetBorder).toHaveBeenCalledWith('s1', ['p1'], { inks: ['#112233'] }, '')
  })

  it('«Убрать» — ApplyBorder(лист, места, "")', async () => {
    await borderAct.clear('s1', ['p1', 'p2'])
    expect(api.ApplyBorder).toHaveBeenCalledWith('s1', ['p1', 'p2'], '')
  })

  it('прозрачность — SetPhotoFade(фото, доля, merge) с сессией фокуса; без сессии merge пуст', async () => {
    await borderAct.fade(['a'], 0.3, 'fade:s1:a')
    expect(api.SetPhotoFade).toHaveBeenLastCalledWith(['a'], 0.3, '')
    borderAct.session.begin('fade:s1:a')
    await borderAct.fade(['a'], 0.4, 'fade:s1:a')
    await borderAct.fade(['a'], 0.5, 'fade:s1:a')
    const calls = api.SetPhotoFade.mock.calls
    expect(calls[1][2]).not.toBe('')
    expect(calls[2][2]).toBe(calls[1][2])
    await borderAct.fade(['a'], 0.6, 'fade:s1:другое')
    expect(calls[3][2]).toBe('')
  })

  it('ошибка Go: просмотр снимается, очередь работает дальше', async () => {
    api.SetPhotoFade.mockRejectedValueOnce(new Error('нельзя')).mockResolvedValue(make())
    borderlive.set({ spread: 's1', ids: ['a'], fade: 0.9 })
    await borderAct.fade(['a'], 0.9, '')
    expect(borderlive.fadeOf('s1', 'a', 0)).toBe(0)
    expect(store.toast?.text).toMatch(/нельзя/i)
    await borderAct.fade(['a'], 0.2, '')
    expect(api.SetPhotoFade).toHaveBeenCalledTimes(2)
  })
})

describe('выделение и строки', () => {
  it('одно место — его рамочка; группа с разной толщиной — «—»; пустое место — «Выбрать…»', () => {
    placesel.select(['p1'])
    expect(sel()).toMatchObject({ name: 'Белая линия', widthMixed: false })
    placesel.select(['p1', 'p2'])
    expect(sel()).toMatchObject({ widthMixed: true, tile: '', all: ['p1', 'p2'] })
    placesel.select(['p3'])
    expect(sel()).toBeNull()
  })

  it('«Авто» (мест нет): рамочки нет — «Выбрать…»', () => {
    store.go(1)
    placesel.select(['c'])
    expect(sel()).toBeNull()
  })
})

describe('плитки для карточки', () => {
  const gold = { id: 'sa:fr-gold', theme: 'basic', name: 'Золотая рамка', url: '/u', own: false, aspect: 1, w: 0, h: 0 }
  beforeEach(() => {
    api.DecorCatalog.mockImplementation(async () => ({ kind: 'frame', themes: [], items: [gold], recent: [], readOnly: false }))
  })

  it('ensureFrames читает каталог один раз, frameItem находит плитку по ключу', async () => {
    expect(decor.frameItem('sa:fr-gold')).toBeUndefined()
    await decor.ensureFrames()
    await decor.ensureFrames()
    expect(api.DecorCatalog).toHaveBeenCalledTimes(1)
    expect(api.DecorCatalog).toHaveBeenCalledWith('frame')
    expect(decor.frameItem('sa:fr-gold')?.name).toBe('Золотая рамка')
    expect(decor.frItems).toHaveLength(1)
    expect(decor.frameItem(undefined)).toBeUndefined()
    expect(decor.frameItem('')).toBeUndefined()
  })
})
