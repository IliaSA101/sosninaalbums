import { beforeEach, describe, expect, it, vi } from 'vitest'

// «Разнообразить раскладки» (041) и очередь удалений: Go подменён, ответы задаёт тест.
const varyCalls: { ids: string[]; variety: number; families?: string[] }[] = []
let answer: () => unknown = () => ({})
let mergeAnswer: () => unknown = () => ({})

// Отложенные ответы удаления: тест отпускает их по одному.
const calls: string[] = []
const pending: Array<{ resolve: (v: unknown) => void; reject: (e: unknown) => void }> = []

vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', () => ({
  VaryLayouts: async (ids: string[], variety: number, families?: string[]) => {
    varyCalls.push({ ids, variety, families })
    const a = answer()
    if (a instanceof Error) throw a
    return a
  },
  MergeSpreads: async () => mergeAnswer(),
  DeleteSpread: (id: string) =>
    new Promise((resolve, reject) => {
      calls.push(id)
      pending.push({ resolve, reject })
    }),
}))

// Окружение vitest — node: notify() ждёт window.setTimeout.
vi.stubGlobal('window', globalThis)

const { store } = await import('./store.svelte')

const view = (rev: number, layouts: Record<string, string>) => ({
  rev,
  folders: [],
  spreads: Object.entries(layouts).map(([id, layout]) => ({ id, layout, photos: [] })),
})

describe('store.varyLayouts', () => {
  beforeEach(() => {
    varyCalls.length = 0
    store.toast = null
    store.busy = false
    store.apply(view(1, { a: 'x', b: 'y', c: 'z' }) as never)
  })

  it('идёт в Go с выделенными и уровнем; перемену показывает тостом «Вернуть» с числом изменённых', async () => {
    answer = () => view(2, { a: 'x', b: 'y2', c: 'z2' })
    expect(await store.varyLayouts(['b', 'c'], 2)).toBe(true)
    expect(varyCalls).toEqual([{ ids: ['b', 'c'], variety: 2, families: [] }])
    expect(store.toast?.text).toBe('Раскладки изменены: 2')
    expect(store.toast?.action?.label).toBe('Вернуть')
  })

  it('семейства (065) уходят третьим аргументом', async () => {
    answer = () => view(2, { a: 'x', b: 'y2', c: 'z' })
    expect(await store.varyLayouts(['b'], 1, ['strip'])).toBe(true)
    expect(varyCalls).toEqual([{ ids: ['b'], variety: 1, families: ['strip'] }])
  })

  it('вид без изменений (rev тот же) — тост «Менять нечего: раскладки уже разные или других подходящих нет», без «Вернуть»', async () => {
    answer = () => view(1, { a: 'x', b: 'y', c: 'z' })
    expect(await store.varyLayouts(['a'], 1)).toBe(false)
    expect(store.toast?.text).toBe('Менять нечего: раскладки уже разные или других подходящих нет')
    expect(store.toast?.action).toBeUndefined()
  })

  it('пустой список и занятое окно — Go не зовём', async () => {
    expect(await store.varyLayouts([], 1)).toBe(false)
    store.busy = true
    expect(await store.varyLayouts(['a'], 1)).toBe(false)
    expect(varyCalls).toEqual([])
  })

  it('ошибка Go — тост с ошибкой', async () => {
    answer = () => new Error('нельзя')
    expect(await store.varyLayouts(['a'], 1)).toBe(false)
    expect(store.toast?.kind).toBe('error')
  })
})

const ids = ['a', 'b', 'c', 'd']
const viewWithout = (gone: string[], rev: number) => ({
  rev,
  folders: [],
  spreads: ids.filter((i) => !gone.includes(i)).map((id) => ({ id })),
})
const tick = () => new Promise((r) => setTimeout(r, 0))

describe('store.deleteSpread: очередь', () => {
  it('три быстрых щелчка — три удаления подряд, id берётся после ответа предыдущего', async () => {
    calls.length = 0
    pending.length = 0
    store.apply(viewWithout([], 1) as never)
    store.goId('a')
    const all = [store.deleteSpread(), store.deleteSpread(), store.deleteSpread()]
    await tick()
    // До ответа Go ушёл только первый.
    expect(calls).toEqual(['a'])
    pending[0].resolve(viewWithout(['a'], 2))
    await tick()
    expect(calls).toEqual(['a', 'b'])
    pending[1].resolve(viewWithout(['a', 'b'], 3))
    await tick()
    expect(calls).toEqual(['a', 'b', 'c'])
    pending[2].resolve(viewWithout(['a', 'b', 'c'], 4))
    await Promise.all(all)
    expect(store.view.spreads.map((s) => s.id)).toEqual(['d'])
  })

  it('ошибка одного удаления не рушит очередь', async () => {
    calls.length = 0
    pending.length = 0
    store.apply(viewWithout([], 1) as never)
    store.goId('a')
    const all = [store.deleteSpread(), store.deleteSpread()]
    await tick()
    pending[0].reject(new Error('сбой'))
    await tick()
    // Первое не прошло, второе берёт тот же текущий разворот и выполняется.
    expect(calls).toEqual(['a', 'a'])
    pending[1].resolve(viewWithout(['a'], 2))
    await Promise.all(all)
    expect(store.view.spreads.map((s) => s.id)).toEqual(['b', 'c', 'd'])
    expect(store.toast?.kind).toBe('info')
  })
})

// Склейка (094 §1.4): тост, как у разреза; снятый фон второго — второй фразой того же тоста.
describe('store.merge: тост', () => {
  const bd = [{ id: 'b1' }]
  const two = (rev: number, lostBg = false) => ({
    rev,
    folders: [],
    spreads: [
      { id: 'a', photos: ['1'], backdrops: lostBg ? bd : undefined },
      { id: 'b', photos: ['2'], backdrops: lostBg ? bd : undefined },
    ],
  })
  const one = (rev: number) => ({ rev, folders: [], spreads: [{ id: 'a', photos: ['1', '2'] }] })

  beforeEach(() => {
    store.toast = null
    store.busy = false
  })

  it('склейка — тост «Развороты склеены» с «Вернуть»', async () => {
    store.apply(two(10) as never)
    mergeAnswer = () => one(11)
    await store.merge('a')
    expect(store.toast?.text).toBe('Развороты склеены')
    expect(store.toast?.action?.label).toBe('Вернуть')
  })

  it('с потерей фона — обе фразы', async () => {
    store.apply(two(20, true) as never)
    mergeAnswer = () => one(21)
    await store.merge('a')
    expect(store.toast?.text).toBe('Развороты склеены. Фон второго разворота снят')
    expect(store.toast?.action?.label).toBe('Вернуть')
  })

  it('холостая — тоста нет', async () => {
    store.apply(two(30) as never)
    mergeAnswer = () => two(30)
    await store.merge('a')
    expect(store.toast).toBeNull()
  })
})
