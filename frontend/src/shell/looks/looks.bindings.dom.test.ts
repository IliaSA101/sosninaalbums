// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render } from '@testing-library/svelte'
import { tick } from 'svelte'

// Окно «Макеты альбома» на настоящем lookApi, мок — биндинги Go (131 U4): «Применить» → ApplyLook с id и запросом → вид принят
// цепочкой; library:changed → перечитывание вида, чтобы View.look.found обновился после установки и удаления набора.
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', async (orig) => {
  const all = await orig<Record<string, unknown>>()
  return { ...Object.fromEntries(Object.keys(all).map((k) => [k, vi.fn(async () => undefined)])) }
})

const { default: LooksDialog } = await import('./LooksDialog.svelte')
const { looks } = await import('./looks.svelte')
const { store } = await import('../../base/core/store.svelte')
const { packsUi } = await import('../packs/packs.svelte')
const Api = await import('../../../wailsjs/go/app/App')
const M = vi.mocked

const A = 'my:0a1b2c3d4e5f6071'
// seq растёт на весь файл: цепочка вида хранит последний номер и пропускает «старые» виды
let seq = 0
const view = (rev: number, extra: Record<string, unknown> = {}) => ({
  rev,
  seq: ++seq,
  folders: [],
  settings: { pageW: 300, pageH: 300, sheet: false },
  spreads: [],
  ...extra,
})
const plan = (o: Record<string, unknown> = {}) => ({
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
  ...o,
})
const btn = (label: string) =>
  Array.from(document.querySelectorAll('button'))
    .filter((b) => b.textContent?.trim() === label)
    .at(-1) as HTMLButtonElement

async function opened() {
  M(Api.DecorCatalog).mockResolvedValue({
    kind: 'album',
    themes: [{ id: 'all', name: 'Все', count: 1 }],
    items: [{ id: A, name: 'Светлая', theme: 'w', url: '', own: false, aspect: 0, w: 0, h: 0, steps: 4, from: 10 }],
    recent: [],
    readOnly: false,
  } as never)
  render(LooksDialog)
  await looks.show()
  await tick()
  await fireEvent.click(document.querySelector('button.tile') as HTMLElement)
  await vi.waitFor(() => expect(btn('Применить').disabled).toBe(false))
}

beforeEach(() => {
  vi.clearAllMocks()
  M(Api.LookInfo).mockResolvedValue({ steps: [{ name: 'Титул', role: '', places: 2, pages: 2 }], cover: false, min: 1, max: 20 } as never)
  M(Api.LookCount).mockImplementation((async (_id: string, n: number) => ({ n, places: n * 5, fit: 0 })) as never)
  M(Api.PlanLook).mockImplementation((async (_id: string, o: { mode: string; spreads: number }) =>
    plan({ mode: o.mode, spreads: o.spreads })) as never)
  M(Api.ApplyLook).mockReset()
  M(Api.GetView).mockReset()
  looks.close()
  Object.assign(looks, {
    cat: null,
    theme: '',
    sel: '',
    spreads: 0,
    addN: 1,
    pick: 'bg',
    step: 1,
    info: null,
    cnt: null,
    plan: null,
    addPlan: null,
    busy: false,
  })
  store.toast = null
  store.apply(view(1) as never)
})
afterEach(() => {
  cleanup()
  looks.close()
})

describe('«Применить» через биндинги', () => {
  it('ApplyLook получает id плитки и запрос; вид из ответа принят, окно закрыто, тост «Вернуть»', async () => {
    M(Api.ApplyLook).mockResolvedValue({ view: view(2), plan: plan({ spreads: 9 }) } as never)
    await opened()
    await fireEvent.click(btn('Применить'))
    await vi.waitFor(() => expect(looks.open).toBe(false))
    expect(Api.ApplyLook).toHaveBeenCalledExactlyOnceWith(A, { mode: 'series', spreads: 4 })
    expect(store.view.rev).toBe(2)
    expect(store.toast?.action).toBeTruthy()
  })

  it('ошибка Go: вид не тронут, красный тост, окно открыто', async () => {
    M(Api.ApplyLook).mockRejectedValue(new Error('Макета нет в библиотеке'))
    await opened()
    await fireEvent.click(btn('Применить'))
    await vi.waitFor(() => expect(store.toast?.kind).toBe('error'))
    expect(store.view.rev).toBe(1)
    expect(looks.open).toBe(true)
  })

  it('«Разворот макета»: NewLookSpread, вид принят, переход на новый разворот', async () => {
    store.apply(view(1, { spreads: [{ id: 's1', photos: [], w: 600, h: 300, mid: 300 }] }) as never)
    M(Api.NewLookSpread).mockResolvedValue(
      view(2, {
        spreads: [
          { id: 's1', photos: [], w: 600, h: 300, mid: 300 },
          { id: 's2', photos: [], w: 600, h: 300, mid: 300 },
        ],
      }) as never,
    )
    await looks.newSpread()
    expect(Api.NewLookSpread).toHaveBeenCalledTimes(1)
    expect(store.view.rev).toBe(2)
    expect(store.current).toBe(1)
  })
})

describe('library:changed → вид заново', () => {
  it('альбом по макету: GetView, found обновился', async () => {
    store.apply(view(1, { look: { key: A, name: 'Светлая', found: false } }) as never)
    M(Api.GetView).mockResolvedValue(view(1, { look: { key: A, name: 'Светлая', found: true } }) as never)
    packsUi.changed(['Свадьба'])
    await vi.waitFor(() => expect(store.view.look?.found).toBe(true))
    expect(Api.GetView).toHaveBeenCalledTimes(1)
  })

  it('альбом без макета: вид не перечитывается', async () => {
    packsUi.changed()
    await tick()
    expect(Api.GetView).not.toHaveBeenCalled()
  })

  it('вид старше текущего (seq меньше) идёт через цепочку и не откатывает вид', async () => {
    store.apply(view(5, { look: { key: A, name: 'Светлая', found: false } }) as never)
    M(Api.GetView).mockResolvedValue(view(4, { seq: 1, look: { key: A, name: 'Светлая', found: true } }) as never)
    await store.resync()
    expect(Api.GetView).toHaveBeenCalledTimes(1)
    expect(store.view.rev).toBe(5)
    expect(store.view.look?.found).toBe(false)
  })

  it('GetView упал: красный тост, вид прежний', async () => {
    store.apply(view(1, { look: { key: A, name: 'Светлая', found: false } }) as never)
    M(Api.GetView).mockRejectedValue(new Error('нет связи'))
    packsUi.changed()
    await vi.waitFor(() => expect(store.toast?.kind).toBe('error'))
    expect(store.view.look?.found).toBe(false)
  })
})
