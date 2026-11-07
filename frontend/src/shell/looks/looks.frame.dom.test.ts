// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render } from '@testing-library/svelte'
import { tick } from 'svelte'

// Окно «Макеты альбома», каркас 163 U2: строка над отбором, ползунок, ряд «Набор · Стиль», «Новые», плитки в два этажа.
// Тесты просмотра и «Применить» — в looks.dom.test.ts, lookpeek.dom.test.ts и lookapply.dom.test.ts.
const api = vi.hoisted(() => ({
  catalog: vi.fn(),
  info: vi.fn(),
  count: vi.fn(),
  plan: vi.fn(),
  apply: vi.fn(),
  rename: vi.fn(),
  hide: vi.fn(),
}))
vi.mock('./lookapi', () => ({ lookApi: api }))
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', async (orig) => {
  const all = await orig<Record<string, unknown>>()
  return { ...Object.fromEntries(Object.keys(all).map((k) => [k, vi.fn(async () => undefined)])) }
})

const { default: LooksDialog } = await import('./LooksDialog.svelte')
const { looks } = await import('./looks.svelte')
const { store } = await import('../../base/core/store.svelte')
const { modes } = await import('../../base/prefs/view.svelte')
const { packsUi } = await import('../packs/packs.svelte')
const { saveLookUi } = await import('./savelook.svelte')

const A = 'my:0a1b2c3d4e5f6071'
const B = 'my:1b2c3d4e5f607182'
const item = (id: string, name: string, o: Record<string, unknown> = {}) => ({
  id,
  name,
  theme: 'w',
  url: '',
  own: false,
  aspect: 0,
  w: 0,
  h: 0,
  ...o,
})
const catalog = (items: unknown[]) => ({
  kind: 'album',
  themes: [
    { id: 'all', name: 'Все', count: items.length },
    { id: 'w', name: 'Свадьба', count: items.length },
    { id: 'mine', name: 'Мои', count: 0 },
  ],
  items,
  recent: [],
  readOnly: false,
})
const info = (n: number) => ({
  steps: Array.from({ length: n }, (_, k) => ({ name: `Шаг ${k + 1}`, role: '', places: 3, pages: 2 })),
  cover: false,
  min: 1,
  max: 20,
})
const plan = () => ({
  mode: 'series',
  trial: false,
  touched: 0,
  full: false,
  spreads: 9,
  places: 46,
  drafts: 0,
  replaced: 0,
  anchors: 0,
  cover: false,
  left: 0,
  prev: '',
  missing: 0,
})

const tiles = () => Array.from(document.querySelectorAll<HTMLButtonElement>('button.tile'))
const tile = (name: string) => tiles().find((b) => b.title.startsWith(name)) as HTMLButtonElement
const text = () => document.body.textContent ?? ''
const btn = (label: string) => Array.from(document.querySelectorAll('button')).find((b) => b.textContent?.trim() === label)

async function opened(items = [item(A, 'Светлая', { steps: 9, from: 26 }), item(B, 'Классика', { steps: 7, from: 18 })]) {
  api.catalog.mockResolvedValue(catalog(items))
  render(LooksDialog)
  await looks.show()
  await tick()
}

beforeEach(() => {
  for (const f of Object.values(api)) f.mockReset()
  api.info.mockImplementation(async () => info(7))
  api.count.mockImplementation(async (_id: string, n: number) => ({ n, places: n * 5, fit: 0 }))
  api.plan.mockImplementation(async () => plan())
  looks.close()
  Object.assign(looks, {
    cat: null,
    theme: '',
    sel: '',
    spreads: 0,
    step: 1,
    info: null,
    cnt: null,
    plan: null,
    modePlan: null,
    busy: false,
  })
  looks.filters = { pack: '', form: '', style: '' }
  store.view = { ...store.view, rev: 1, settings: { ...store.view.settings, pageW: 300, pageH: 300, sheet: false } } as never
  modes.details = false
  modes.decorTile = {}
})
afterEach(() => {
  cleanup()
  looks.close()
  vi.restoreAllMocks()
  store.photos = []
  modes.decorTile = {}
})

// 163 U2: каркас окна «Развороты» — строка над отбором, ползунок, ряд «Набор · Стиль», плитки с подписью в два этажа.
const slider = () => document.querySelector<HTMLInputElement>('input[aria-label="Размер миниатюр"]')
const tilesBox = () => document.querySelector<HTMLElement>('.tiles')
const sideBtn = (name: string) =>
  Array.from(document.querySelectorAll<HTMLButtonElement>('[aria-label="Темы"] button')).find(
    (b) => b.querySelector('.name')?.textContent === name,
  )
const sideCount = (name: string) => sideBtn(name)?.querySelector('em')?.textContent
const packSelect = () => document.querySelector<HTMLSelectElement>('#decor-pack')
const withPacks = () => [
  item(A, 'Светлая', { steps: 9, from: 26, pack: 'Акварель', packId: 'p1', style: 'Нежные' }),
  item(B, 'Классика', { steps: 7, from: 18, pack: 'Акварель', packId: 'p1', style: 'Нежные' }),
  item('my:2c3d4e5f60718293', 'Графика', { steps: 5, from: 12, pack: 'Линии', packId: 'p2', style: 'Строгие' }),
  item('my:3d4e5f6071829304', 'Контур', { steps: 5, from: 12, pack: 'Линии', packId: 'p2', style: 'Строгие' }),
]

describe('LooksDialog: каркас по образцу «Развороты» (163)', () => {
  it('плитка: имя целиком, второй строкой «от N фото · N разворотов»; подсказка с набором и «крупно»', async () => {
    await opened([
      item(A, 'Свадьба: синяя акварель', { steps: 9, from: 26, pack: 'Акварель', packId: 'p1' }),
      item(B, 'Мой', { steps: 7, own: true }),
    ])
    const [a, b] = tiles()
    expect(a.title).toBe('Свадьба: синяя акварель · набор «Акварель»\nЩелчок или правый щелчок — крупно')
    expect(b.title).toBe('Мой\nЩелчок или правый щелчок — крупно')
    expect(a.dataset.decorId).toBe(A)
    expect(document.querySelector('.sname')?.textContent).toBe('Свадьба: синяя акварель')
    expect(document.querySelector('.snote')?.textContent).toBe('от 26 фото · 9 разворотов')
  })

  it('ползунок «Размер миниатюр»: по умолчанию 168, --tile и память album меняются, пределы 128…220', async () => {
    modes.decorTile = {}
    await opened()
    const s = slider() as HTMLInputElement
    expect([s.min, s.max, s.value]).toEqual(['128', '220', '168'])
    expect(tilesBox()?.style.getPropertyValue('--tile')).toBe('168px')
    s.value = '200'
    await fireEvent.input(s)
    await tick()
    expect(tilesBox()?.style.getPropertyValue('--tile')).toBe('200px')
    expect(modes.decorTile.album).toBe(200)
    modes.decorTile = {}
  })

  it('шапка: «Наборы…» открывает окно наборов, «Сохранить альбом как макет…» — окно сохранения; в «только чтении» второе серое', async () => {
    const packs = vi.spyOn(packsUi, 'show').mockResolvedValue(undefined)
    const save = vi.spyOn(saveLookUi, 'show').mockResolvedValue(undefined)
    await opened()
    await fireEvent.click(document.querySelector('.bar button[title^="Наборы библиотеки"]') as HTMLElement)
    expect(packs).toHaveBeenCalledTimes(1)
    await fireEvent.click(btn('Сохранить альбом как макет…') as HTMLElement)
    expect(save).toHaveBeenCalledTimes(1)
    cleanup()
    api.catalog.mockResolvedValue({ ...catalog([item(A, 'Светлая', { steps: 9 })]), readOnly: true })
    render(LooksDialog)
    await looks.show()
    await tick()
    expect((btn('Сохранить альбом как макет…') as HTMLButtonElement).disabled).toBe(true)
  })

  it('строка «Добавьте фото в ленту…» — только в альбоме без фото', async () => {
    store.photos = []
    await opened()
    expect(text()).toContain('Добавьте фото в ленту — макеты покажутся на них')
    store.photos = [{ id: 'p1' }] as never
    await tick()
    expect(text()).not.toContain('Добавьте фото в ленту')
    store.photos = []
  })

  it('отбор скрыт при одном наборе и без стилей; «Вида» нет никогда', async () => {
    await opened()
    expect(document.querySelector('.filters')).toBeNull()
    cleanup()
    looks.close()
    await opened(withPacks())
    expect(packSelect()).not.toBeNull()
    expect(document.querySelector('[aria-label="Стиль макетов"]')).not.toBeNull() // по два предмета на стиль — порог макетов
    expect(text()).not.toContain('Вид')
  })

  it('набор в отборе оставляет плитки набора, числа тем считаются по отбору', async () => {
    await opened(withPacks())
    expect(tiles()).toHaveLength(4)
    expect(sideCount('Свадьба')).toBe('4')
    packSelect()!.value = 'p2'
    await fireEvent.change(packSelect() as HTMLElement)
    await tick()
    expect(tiles().map((t) => t.dataset.decorId)).toEqual(['my:2c3d4e5f60718293', 'my:3d4e5f6071829304'])
    expect(sideCount('Свадьба')).toBe('2')
    expect(sideCount('Все')).toBe('2')
    expect(looks.filters.pack).toBe('p2')
  })

  it('«Новые»: тема со своим числом, плитки — только из списка new', async () => {
    api.catalog.mockResolvedValue({
      ...catalog([item(A, 'Светлая', { steps: 9 }), item(B, 'Классика', { steps: 7 })]),
      themes: [
        { id: 'all', name: 'Все', count: 2 },
        { id: 'new', name: 'Новые', count: 1 },
        { id: 'w', name: 'Свадьба', count: 2 },
      ],
      new: [B],
    })
    render(LooksDialog)
    await looks.show()
    await tick()
    expect(sideCount('Новые')).toBe('1')
    await fireEvent.click(sideBtn('Новые') as HTMLElement)
    expect(tiles().map((t) => t.dataset.decorId)).toEqual([B])
  })

  it('✎ и × — только у своих макетов и не в «только чтении»', async () => {
    await opened([item(A, 'Светлая', { steps: 9 }), item(B, 'Мой', { steps: 7, own: true })])
    expect(document.querySelectorAll('[aria-label="Переименовать макет"]')).toHaveLength(1)
    expect(document.querySelectorAll('[aria-label="Убрать макет из библиотеки"]')).toHaveLength(1)
    expect(tile('Мой').closest('.item')?.querySelector('.tools')).not.toBeNull()
    expect(tile('Светлая').closest('.item')?.querySelector('.tools')).toBeNull()
    cleanup()
    api.catalog.mockResolvedValue({ ...catalog([item(B, 'Мой', { steps: 7, own: true })]), readOnly: true })
    render(LooksDialog)
    await looks.show()
    await tick()
    expect(document.querySelector('[aria-label="Переименовать макет"]')).toBeNull()
  })

  it('правый щелчок по плитке: меню браузера погашено, действие как у щелчка — крупный просмотр', async () => {
    await opened()
    const ev = new MouseEvent('contextmenu', { bubbles: true, cancelable: true })
    tile('Классика').dispatchEvent(ev)
    expect(ev.defaultPrevented).toBe(true)
    await vi.waitFor(() => expect(looks.sel).toBe(B))
    expect(looks.peek.id).toBe(B)
  })
})
