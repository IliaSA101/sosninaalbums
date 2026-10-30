// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render } from '@testing-library/svelte'
import { tick } from 'svelte'

// «Макеты альбома»: плитки «Можно скачать» (144). Щелчок → вопрос → скачивание → макет выбран и показан, но НЕ применён.
const api = vi.hoisted(() => ({
  catalog: vi.fn(),
  info: vi.fn(),
  count: vi.fn(),
  plan: vi.fn(),
  apply: vi.fn(),
  rename: vi.fn(),
  hide: vi.fn(),
  demoOffer: vi.fn(),
}))
const go = vi.hoisted(() => ({
  DownloadLibraryPack: vi.fn(),
  PackItem: vi.fn(),
  FetchPackPreviews: vi.fn(),
  LibraryPacks: vi.fn(),
  CancelLibrary: vi.fn(),
}))
vi.mock('./lookapi', () => ({ lookApi: api }))
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', async (orig) => {
  const all = await orig<Record<string, unknown>>()
  return { ...Object.fromEntries(Object.keys(all).map((k) => [k, vi.fn(async () => undefined)])), ...go }
})

const { default: LooksDialog } = await import('./LooksDialog.svelte')
const { looks } = await import('./looks.svelte')
const { store } = await import('../../base/core/store.svelte')
const { packsUi } = await import('../packs/packs.svelte')

const MB = 1048576
const SRC = '0123456789abcdef'
const A = 'my:0a1b2c3d4e5f6071'
const C = 'my:2c3d4e5f60718293'
const item = (id: string, name: string, theme: string, o: Record<string, unknown> = {}) => ({
  id,
  name,
  theme,
  url: '',
  own: false,
  aspect: 0,
  w: 0,
  h: 0,
  steps: 7,
  from: 18,
  ...o,
})
const tile = (o: Record<string, unknown> = {}) => ({
  id: `get:school:${SRC}`,
  pack: 'school',
  src: SRC,
  title: 'Школа — осень',
  url: '/media/lp/school/album/0',
  w: 240,
  h: 160,
  bytes: 12 * MB,
  with: [],
  bg: 0,
  sticker: 0,
  spread: 7,
  frame: 0,
  ...o,
})
const catalog = (items: unknown[], o: Record<string, unknown> = {}) => ({
  kind: 'album',
  themes: [
    { id: 'all', name: 'Все', count: items.length },
    { id: 'w', name: 'Свадьба', count: items.length },
    { id: 'get', name: 'Можно скачать', count: 1 },
  ],
  items,
  recent: [],
  new: [],
  get: [tile()],
  invite: false,
  readOnly: false,
  ...o,
})
const info = () => ({ steps: [{ name: 'Шаг 1', role: '', places: 3, pages: 2 }], cover: false, min: 1, max: 20 })
const plan = () => ({
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
})
const picked = () => ({ added: 1, updated: 0, skipped: [], bytes: 1, picked: true })

const btn = (label: string) => Array.from(document.querySelectorAll('button')).find((b) => b.textContent?.trim() === label)
const getTile = (id = `get:school:${SRC}`) => document.querySelector<HTMLButtonElement>(`[data-get-id="${id}"]`)
const toast = () => store.toast as { text: string; kind?: string } | null
const text = () => document.body.textContent ?? ''
const theme = async (name: string) => {
  fireEvent.click(Array.from(document.querySelectorAll('.vseg button')).find((b) => b.textContent?.includes(name))!)
  await tick()
}

async function opened(items: unknown[] = [item(A, 'Светлая', 'w')], o: Record<string, unknown> = {}) {
  api.catalog.mockResolvedValue(catalog(items, o))
  render(LooksDialog)
  await looks.show()
  await tick()
}

beforeEach(() => {
  for (const f of [...Object.values(api), ...Object.values(go)]) f.mockReset()
  api.info.mockImplementation(async () => info())
  api.count.mockImplementation(async (_id: string, n: number) => ({ n, places: n * 3, fit: 0 }))
  api.plan.mockImplementation(async () => plan())
  api.demoOffer.mockResolvedValue(false)
  go.FetchPackPreviews.mockResolvedValue(undefined)
  go.LibraryPacks.mockResolvedValue({ packs: [], net: { allowed: true }, busy: false, readOnly: false })
  go.PackItem.mockResolvedValue(item(C, 'Осень', 'school'))
  looks.close()
  Object.assign(looks, { cat: null, theme: '', sel: '', spreads: 0, pick: 'bg', step: 1, info: null, cnt: null, plan: null, busy: false })
  packsUi.close()
  packsUi.net.updating = false
  packsUi.net.progress = null
  store.toast = null
})
afterEach(() => {
  cleanup()
  looks.close()
  vi.restoreAllMocks()
})

describe('«Макеты альбома»: «Можно скачать» (144)', () => {
  it('строка последней в темах; плитка с подписью и подсказкой «показать этот макет»; образцы докачиваются при открытии', async () => {
    await opened()
    const rows = Array.from(document.querySelectorAll('.vseg button')).map((b) => b.textContent?.trim())
    expect(rows.at(-1)).toBe('Можно скачать1')
    expect(go.FetchPackPreviews).toHaveBeenCalledTimes(1)
    await theme('Можно скачать')
    const t = getTile()!
    expect(t.title).toBe('Скачать набор «Школа — осень» (12,0 МБ) и показать этот макет')
    expect(t.parentElement?.querySelector('.line1')?.textContent?.replace(/\s+/g, ' ').trim()).toBe('⤓ Школа — осень · 12,0 МБ')
    expect(t.querySelector('img')?.getAttribute('src')).toBe('/media/lp/school/album/0')
  })

  it('щелчок → вопрос → «Скачать и показать» → скачан, карточка выбрана в своей теме, «Применить» не вызван', async () => {
    go.DownloadLibraryPack.mockImplementation(async () => {
      api.catalog.mockResolvedValue(catalog([item(A, 'Светлая', 'w'), item(C, 'Осень', 'school')], { get: [] }))
      return picked()
    })
    await opened()
    await theme('Можно скачать')
    fireEvent.click(getTile()!)
    await tick()
    expect(text()).toContain('Скачать набор?')
    expect(text()).toContain('В наборе «Школа — осень» 7 разворотов — 12,0 МБ. Макет откроется в этом окне.')
    expect(btn('Отмена')).toBeTruthy()
    fireEvent.click(btn('Скачать и показать')!)
    await vi.waitFor(() => expect(looks.sel).toBe(C))
    expect(go.DownloadLibraryPack).toHaveBeenCalledWith('school')
    expect(go.PackItem).toHaveBeenCalledWith('album', 'school', SRC)
    expect(api.info).toHaveBeenCalledWith(C)
    expect(api.apply).not.toHaveBeenCalled()
    expect(toast()?.text).toBe('Набор «Школа — осень» скачан')
  })

  it('«Отмена» в вопросе: ничего не скачивается', async () => {
    await opened()
    await theme('Можно скачать')
    fireEvent.click(getTile()!)
    await tick()
    fireEvent.click(btn('Отмена')!)
    await tick()
    expect(btn('Скачать и показать')).toBeUndefined()
    expect(go.DownloadLibraryPack).not.toHaveBeenCalled()
  })

  it('окно закрыто во время хода: макет не выбирается, PackItem не зовётся, остаётся тост', async () => {
    let done: (v: unknown) => void = () => {}
    go.DownloadLibraryPack.mockReturnValue(new Promise((r) => (done = r)))
    await opened()
    await theme('Можно скачать')
    fireEvent.click(getTile()!)
    await tick()
    fireEvent.click(btn('Скачать и показать')!)
    await vi.waitFor(() => expect(packsUi.net.updating).toBe(true))
    packsUi.net.progressed({ stage: 'download', title: 'Школа — осень', done: 3 * MB, total: 12 * MB })
    await tick()
    expect(document.querySelector('[role="status"]')?.textContent).toBe('Скачиваем «Школа — осень» — 3 из 12 МБ')
    expect(getTile()!.getAttribute('aria-disabled')).toBe('true')
    looks.close()
    done(picked())
    await vi.waitFor(() => expect(packsUi.net.updating).toBe(false))
    expect(go.PackItem).not.toHaveBeenCalled()
    expect(looks.sel).toBe('')
    expect(toast()?.text).toBe('Набор «Школа — осень» скачан')
  })

  it('макета в наборе не оказалось — тост «выберите другой», выбора нет', async () => {
    go.DownloadLibraryPack.mockResolvedValue(picked())
    go.PackItem.mockRejectedValue('этого предмета в наборе нет — выберите другой')
    await opened()
    await theme('Можно скачать')
    fireEvent.click(getTile()!)
    await tick()
    fireEvent.click(btn('Скачать и показать')!)
    await vi.waitFor(() => expect(toast()?.text).toBe('Набор «Школа — осень» скачан, но этого макета в нём нет — выберите другой'))
    expect(looks.sel).toBe('')
  })

  it('каталог после скачивания без этого макета — молча, выбор не меняется', async () => {
    go.DownloadLibraryPack.mockResolvedValue(picked())
    await opened()
    await theme('Можно скачать')
    fireEvent.click(getTile()!)
    await tick()
    fireEvent.click(btn('Скачать и показать')!)
    await vi.waitFor(() => expect(go.PackItem).toHaveBeenCalled())
    await vi.waitFor(() => expect(packsUi.net.updating).toBe(false))
    await tick()
    expect(looks.sel).toBe('')
  })

  it('макетов нет, а образцы есть: окно не пустое, плитки видны; нет ни того ни другого — прежняя надпись', async () => {
    await opened([])
    expect(text()).not.toContain('Макетов пока нет')
    await theme('Можно скачать')
    expect(getTile()).not.toBeNull()
    cleanup()
    looks.close()
    await opened([], { get: [], themes: [{ id: 'all', name: 'Все', count: 0 }] })
    expect(text()).toContain('Макетов пока нет')
  })

  it('приглашение «⤓ Ещё из интернета» в конце «Все» открывает «Наборы библиотеки»', async () => {
    await opened([item(A, 'Светлая', 'w')], { invite: true, get: [], themes: [{ id: 'all', name: 'Все', count: 1 }] })
    await theme('Все')
    const inv = getTile('invite')!
    expect(inv.parentElement?.querySelector('.line1')?.textContent?.trim()).toBe('⤓ Ещё из интернета')
    fireEvent.click(inv)
    await vi.waitFor(() => expect(packsUi.open).toBe(true))
  })

  it('без invite и без образцов ни плиток, ни приглашения', async () => {
    await opened([item(A, 'Светлая', 'w')], { get: [], themes: [{ id: 'all', name: 'Все', count: 1 }] })
    await theme('Все')
    expect(document.querySelectorAll('[data-get-id]')).toHaveLength(0)
  })
})
