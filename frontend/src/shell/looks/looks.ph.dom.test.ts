// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render } from '@testing-library/svelte'
import { tick } from 'svelte'

// Метка ph у /media/look (163 U4): плитки и крупный просмотр показывают макет на фото ленты. Сам показ рисует Go, здесь —
// что адрес несёт метку, метка меняется с лентой, правкой и отметками, потолок кадра просмотра 2048, bg не шлётся.
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
  steps: 7,
  from: 18,
  ...o,
})
const catalog = (items: unknown[]) => ({
  kind: 'album',
  themes: [
    { id: 'all', name: 'Все', count: items.length },
    { id: 'w', name: 'Свадьба', count: items.length },
  ],
  items,
  recent: [],
  readOnly: false,
})
const info = (cover = false) => ({ steps: [{ name: 'Шаг 1', role: '', places: 3, pages: 2 }], cover, min: 1, max: 20 })
const plan = (o: Record<string, unknown> = {}) => ({
  mode: 'series',
  trial: false,
  touched: 0,
  full: false,
  spreads: 1,
  places: 3,
  drafts: 0,
  replaced: 0,
  anchors: 0,
  cover: false,
  left: 0,
  prev: '',
  missing: 0,
  ...o,
})
const photo = (id: string) => ({ id }) as never

const tiles = () => Array.from(document.querySelectorAll<HTMLButtonElement>('button.tile'))
const tile = (name: string) => tiles().find((b) => b.title.startsWith(name)) as HTMLButtonElement
const tileSrc = (name: string) => tile(name).querySelector('img')?.getAttribute('src') ?? ''
const sheetImgs = () => Array.from(document.querySelectorAll<HTMLImageElement>('.sheet img'))
const bigSrc = () => sheetImgs().at(-1)?.getAttribute('src') ?? ''
const phOf = (src: string) => Number(/[?&]ph=(\d+)/.exec(src)?.[1] ?? 0)
const pxOf = (src: string) => Number(/[?&]px=(\d+)/.exec(src)?.[1] ?? 0)

async function opened() {
  api.catalog.mockResolvedValue(catalog([item(A, 'Светлая', { cover: true }), item(B, 'Классика')]))
  render(LooksDialog)
  await looks.show()
  await tick()
}
async function peekOf(name: string) {
  await fireEvent.click(tile(name))
  await vi.waitFor(() => expect(looks.ready).toBe(true))
}

beforeEach(() => {
  for (const f of Object.values(api)) f.mockReset()
  api.info.mockImplementation(async (id: string) => info(id === A))
  api.count.mockImplementation(async (_id: string, n: number) => ({ n, places: n * 3, fit: 0 }))
  api.plan.mockImplementation(async (_id: string, o: { mode: string; spreads: number }) => plan({ mode: o.mode, spreads: o.spreads }))
  looks.close()
  Object.assign(looks, {
    cat: null,
    theme: '',
    sel: '',
    spreads: 0,
    pick: 'bg',
    step: 1,
    info: null,
    cnt: null,
    plan: null,
    modePlan: null,
    busy: false,
  })
  looks.filters = { pack: '', form: '', style: '' }
  store.photos = []
  store.toast = null
  store.view = { ...store.view, rev: 1, marksRev: 0, settings: { ...store.view.settings, pageW: 300, pageH: 300, sheet: false } } as never
})
afterEach(() => {
  cleanup()
  looks.close()
  vi.restoreAllMocks()
})

describe('плитки: ph в адресе (163 U4)', () => {
  it('у обеих плиток метка > 0, в том числе в альбоме без фото (ответ Go — серые места в JPEG)', async () => {
    await opened()
    expect(store.photos).toHaveLength(0)
    expect(phOf(tileSrc('Светлая'))).toBeGreaterThan(0)
    expect(phOf(tileSrc('Классика'))).toBe(phOf(tileSrc('Светлая')))
    expect(tileSrc('Светлая')).toContain('&f=1&ph=') // лицевая обложки, метка в конце
  })

  it('смена ленты (фото добавили) меняет метку у плиток — кэш WebView сброшен', async () => {
    await opened()
    const before = phOf(tileSrc('Классика'))
    store.photos = [photo('a'), photo('b'), photo('c')]
    await tick()
    const after = phOf(tileSrc('Классика'))
    expect(after).not.toBe(before)
    expect(after).toBe((1 * 1000 + 3) * 1000 + 0 + 1)
  })

  it('правка альбома (rev) и отметки (marksRev) тоже меняют метку', async () => {
    await opened()
    const base = phOf(tileSrc('Классика'))
    store.view = { ...store.view, rev: 2 } as never
    await tick()
    const byRev = phOf(tileSrc('Классика'))
    store.view = { ...store.view, marksRev: 1 } as never
    await tick()
    const byMarks = phOf(tileSrc('Классика'))
    expect(new Set([base, byRev, byMarks]).size).toBe(3)
  })

  it('без смен метка стоит: лишних перезапросов нет', async () => {
    await opened()
    const a = tileSrc('Классика')
    store.photos = [photo('a')]
    store.photos = []
    await tick()
    expect(tileSrc('Классика')).toBe(a)
  })
})

describe('крупный просмотр: ph, потолок 2048, без bg (163 U4, 163-15)', () => {
  it('лист и миниатюра плитки под ним несут одну метку; bg=1 нет', async () => {
    store.photos = [photo('a'), photo('b')]
    await opened()
    await peekOf('Классика')
    const ph = phOf(tileSrc('Классика'))
    expect(ph).toBeGreaterThan(0)
    expect(phOf(bigSrc())).toBe(ph)
    for (const img of sheetImgs()) expect(img.getAttribute('src')).not.toContain('bg=1')
  })

  it('собранный альбом: серия на фото (ph) и по-прежнему без bg=1', async () => {
    api.plan.mockImplementation(async (_id: string, o: { mode: string; spreads: number }) =>
      plan({ mode: o.mode, spreads: o.spreads, full: true }),
    )
    store.photos = [photo('a')]
    await opened()
    await peekOf('Классика')
    expect(looks.mode).toBe('bg')
    expect(phOf(bigSrc())).toBeGreaterThan(0)
    expect(bigSrc()).toContain('bg=0')
  })

  it('фото в ленте добавили при открытом просмотре — адрес листа получает новую метку', async () => {
    await opened()
    await peekOf('Классика')
    const before = phOf(bigSrc())
    store.photos = [photo('a'), photo('b')]
    await tick()
    expect(phOf(bigSrc())).not.toBe(before)
  })

  it('потолок кадра: на большом окне px просмотра — 2048, не выше и не 1024', async () => {
    const w = window.innerWidth
    const h = window.innerHeight
    try {
      Object.assign(window, { innerWidth: 6000, innerHeight: 4000 })
      await opened()
      await peekOf('Классика')
      expect(pxOf(bigSrc())).toBe(2048)
    } finally {
      Object.assign(window, { innerWidth: w, innerHeight: h })
    }
  })
})
