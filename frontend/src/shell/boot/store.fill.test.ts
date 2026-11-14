import { beforeEach, describe, expect, it, vi } from 'vitest'

// 132-13: «Заполнить места» — число «отсюда», тост «где» вместо вызова Go, тост итога разностью видов.
const calls: { ids: string[]; at: string }[] = []
let answer: () => unknown = () => ({})

const sp = (id: string, free = 0) => ({ id, photos: [], places: Array.from({ length: free }, () => ({})) })
// Вид с обложкой: coverFill — места обложки и всех разворотов, fillFrom[i] — с разворота i.
const view = (rev: number, f: { cover: number; from: number[]; empty?: number; spreads?: object[] }) => ({
  rev,
  marksRev: 1,
  folders: [],
  spreads: f.spreads ?? f.from.map((_, i) => sp(`s${i + 1}`)),
  cover: { sheet: sp('cover') },
  emptyPlaces: f.empty ?? f.from.reduce((a, b) => a + b, 0),
  fillFrom: f.from,
  coverFill: f.cover + (f.from[0] ?? 0),
})

vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', () => ({
  Marks: async () => ({}),
  FillPlaces: async (ids: string[], at: string) => {
    calls.push({ ids, at })
    return answer()
  },
}))
vi.mock('../../editor/text/fonts', () => ({ loadFonts: async () => {} }))
vi.stubGlobal('window', globalThis)

const { store } = await import('../../base/core/store.svelte')
const { wire } = await import('./wire')
const { ownMarks } = await import('../../panels/marks/ownmarks.svelte')

const ph = (id: string) => ({ id, path: id, name: id, w: 100, h: 100 })
wire()

beforeEach(() => {
  calls.length = 0
  store.selected.clear()
  store.toast = null
  store.filter = 'all'
  store.photos = ['a', 'b', 'c', 'd', 'e'].map(ph) as never
  ownMarks.invalidate()
})

describe('«Заполнить места»: тост «где»', () => {
  it('отсюда 0, но места есть: тост без вызова Go', async () => {
    // пустое место только в развороте 1; открыт разворот 3 — отсюда вперёд мест нет
    store.apply(view(2, { cover: 0, from: [1, 0, 0], spreads: [sp('s1', 1), sp('s2'), sp('s3')] }) as never)
    store.go(2)
    await store.fillPlaces()
    expect(calls).toEqual([])
    expect(store.toast?.text).toBe('Пустые места — в развороте 1')
  })

  it('на обложке и в развороте 3', async () => {
    // два места на обложке и одно в развороте 3; открыт разворот 4 — отсюда вперёд мест нет
    store.apply(view(2, { cover: 2, from: [1, 1, 1, 0], spreads: [sp('s1'), sp('s2'), sp('s3', 1), sp('s4')] }) as never)
    store.go(3)
    await store.fillPlaces()
    expect(calls).toEqual([])
    expect(store.toast?.text).toBe('Пустые места — на обложке, в развороте 3')
  })

  it('отсюда есть места: Go зовётся, без тоста «где»', async () => {
    answer = () => ({ view: view(3, { cover: 0, from: [0, 0, 0] }), left: 0 })
    store.apply(view(1, { cover: 0, from: [2, 1, 1] }) as never)
    store.go(1)
    await store.fillPlaces()
    expect(calls).toEqual([{ ids: ['a', 'b', 'c', 'd', 'e'], at: 's2' }])
    expect(store.toast?.text ?? '').not.toContain('Пустые места')
  })

  it('мест в альбоме нет вовсе (emptyPlaces 0): прежний путь — Go зовётся; выделенное не влезло — остаток назван', async () => {
    answer = () => ({ view: view(2, { cover: 0, from: [0] }), left: 5 })
    store.apply(view(1, { cover: 0, from: [0] }) as never)
    for (const id of ['a', 'b', 'c', 'd', 'e']) store.selected.add(id)
    await store.fillPlaces()
    expect(calls).toHaveLength(1)
    expect(store.toast?.text).toBe('Не хватило мест: 5 фото остались в ленте')
  })

  it('без выделения остаток ленты не сообщается (132-22): тоста нет', async () => {
    answer = () => ({ view: view(2, { cover: 0, from: [0] }), left: 5 })
    store.apply(view(1, { cover: 0, from: [0] }) as never)
    await store.fillPlaces()
    expect(calls).toHaveLength(1)
    expect(store.toast?.text ?? '').toBe('')
  })

  it('поля fillFrom нет вовсе (старый вид): не падает, на «где» не уходит при emptyPlaces 0', async () => {
    answer = () => ({ view: { rev: 2, marksRev: 1, folders: [], spreads: [] }, left: 0 })
    store.apply({ rev: 1, marksRev: 1, folders: [], spreads: [sp('s1')] } as never)
    await store.fillPlaces()
    expect(calls).toHaveLength(1)
  })
})

describe('«Заполнить места»: тост итога — разность видов', () => {
  it('с обложки: и на обложку, и в развороты', async () => {
    store.apply(view(1, { cover: 3, from: [4, 2] }) as never)
    store.goCover()
    answer = () => ({ view: view(2, { cover: 0, from: [0, 0] }), left: 0 })
    store.photos = ['a', 'b', 'c', 'd', 'e', 'f', 'g', 'h', 'i'].map(ph) as never
    await store.fillPlaces()
    expect(calls[0].at).toBe('cover')
    expect(store.toast?.text).toBe('На обложку: 3, в развороты: 4')
  })

  it('выделенное фото с фона Go пропустил: число — по видам, а не по выделению', async () => {
    store.apply(view(1, { cover: 3, from: [4] }) as never)
    store.goCover()
    store.selected.add('a')
    store.selected.add('b')
    store.selected.add('c')
    store.selected.add('d') // 4 выделено, Go посадил 3 (одно с фона): 3 на обложку, 0 в развороты — тоста нет
    answer = () => ({ view: view(2, { cover: 0, from: [4] }), left: 0 })
    await store.fillPlaces()
    expect(store.toast?.text ?? '').toBe('')
    // Теперь выделено 5: Go посадил 4 (одно с фона): 3 на обложку + 1 в развороты
    store.apply(view(3, { cover: 3, from: [4] }) as never)
    store.selected.clear()
    for (const id of ['a', 'b', 'c', 'd', 'e']) store.selected.add(id)
    answer = () => ({ view: view(4, { cover: 0, from: [3] }), left: 0 })
    await store.fillPlaces()
    expect(store.toast?.text).toBe('На обложку: 3, в развороты: 1')
  })

  it('не хватило мест: итог и остаток через точку', async () => {
    store.apply(view(1, { cover: 1, from: [3] }) as never)
    store.goCover()
    store.photos = ['a', 'b'].map(ph) as never
    answer = () => ({ view: view(2, { cover: 0, from: [2] }), left: 0 })
    await store.fillPlaces()
    expect(store.toast?.text).toBe('На обложку: 1, в развороты: 1')
    // без выделения остаток ленты в тост не идёт — только итог
    store.apply(view(5, { cover: 1, from: [3] }) as never)
    answer = () => ({ view: view(6, { cover: 0, from: [2] }), left: 2 })
    await store.fillPlaces()
    expect(store.toast?.text).toBe('На обложку: 1, в развороты: 1')
    // с выделением — итог и остаток через точку
    store.apply(view(7, { cover: 1, from: [3] }) as never)
    store.selected.add('a')
    store.selected.add('b')
    answer = () => ({ view: view(8, { cover: 0, from: [2] }), left: 2 })
    await store.fillPlaces()
    expect(store.toast?.text).toBe('На обложку: 1, в развороты: 1. Не хватило мест: 2 фото остались в ленте')
  })

  it('с разворота — итога «На обложку» нет, обложка не берётся', async () => {
    store.apply(view(1, { cover: 2, from: [2, 1] }) as never)
    store.go(0)
    answer = () => ({ view: view(2, { cover: 2, from: [0, 0] }), left: 0 })
    await store.fillPlaces()
    expect(calls[0].at).toBe('s1')
    expect(store.toast?.text ?? '').toBe('')
  })
})
