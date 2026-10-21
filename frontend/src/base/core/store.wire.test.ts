import { beforeAll, beforeEach, describe, expect, it, vi } from 'vitest'

// Цепочка видов (075): дубль события и ответа, дельта не к тому виду → GetView, saved без вида.
const handlers: Record<string, (...a: unknown[]) => void> = {}
const getView = vi.fn<() => Promise<unknown>>()

vi.mock('../../../wailsjs/runtime/runtime', () => ({
  EventsOn: (name: string, fn: (...a: unknown[]) => void) => {
    handlers[name] = fn
    return () => {}
  },
}))
vi.mock('../../../wailsjs/go/app/App', () => ({
  GetState: () => Promise.resolve({ view: { spreads: [], folders: [] }, photos: [], missing: [] }),
  GetView: () => getView(),
  MissingNames: () => Promise.resolve({}),
}))

vi.stubGlobal('window', globalThis) // notify ставит таймер на window

type Sp = { id: string; name: string }
const sp = (id: string, name = ''): Sp => ({ id, name })
const base = { folders: [], rev: 1, dirty: false, path: '' }
const full = (seq: number, spreads: Sp[], extra = {}) => ({ ...base, seq, spreads, ...extra })
const delta = (seq: number, order: string[], fresh: Sp[], extra = {}) => ({ ...base, seq, base: seq - 1, order, spreads: fresh, ...extra })

// Свежий store на тест: номер цепочки (seq) у него свой.
const load = async () => {
  vi.resetModules()
  const { store } = await import('./store.svelte')
  await store.init()
  return store
}
const flush = () => new Promise((r) => setTimeout(r, 0))

// Первый разбор store — холодный и долгий, остальные тесты берут его из кеша.
beforeAll(async () => {
  await import('./store.svelte')
}, 60000)

beforeEach(() => {
  for (const k of Object.keys(handlers)) delete handlers[k]
  getView.mockReset()
})

describe('store.apply: дельта вида', () => {
  it('дубль (событие + ответ) применяется один раз', async () => {
    const store = await load()
    store.apply(full(1, [sp('a'), sp('b')]) as never)
    const d = delta(2, ['a', 'b'], [sp('b', 'x')], { rev: 2 })
    handlers.view(d)
    expect(store.view.rev).toBe(2)
    store.apply({ ...d, rev: 99 } as never) // тот же seq: ответ метода после события — пропуск
    expect(store.view.rev).toBe(2)
  })

  it('неизменённые развороты — те же объекты', async () => {
    const store = await load()
    store.apply(full(1, [sp('a'), sp('b'), sp('c')]) as never)
    const a = store.view.spreads[0]
    const c = store.view.spreads[2]
    store.apply(delta(2, ['a', 'b', 'c'], [sp('b', 'x')]) as never)
    expect(store.view.spreads[0]).toBe(a)
    expect(store.view.spreads[2]).toBe(c)
    expect(store.view.spreads[1].name).toBe('x')
  })

  it('used: правка без смены состава фото — тот же набор, смена — новый (075-16)', async () => {
    const store = await load()
    const ph = (id: string, name: string, photos: string[]) => ({ id, name, photos })
    store.apply(full(1, [ph('a', '', ['x', 'y']), ph('b', '', ['z'])] as never) as never)
    const first = store.used
    expect([...first].sort()).toEqual(['x', 'y', 'z'])
    store.apply(delta(2, ['a', 'b'], [ph('b', 'имя', ['z'])] as never) as never)
    expect(store.used).toBe(first)
    store.apply(delta(3, ['a', 'b'], [ph('b', 'имя', ['z', 'q'])] as never) as never)
    expect([...store.used].sort()).toEqual(['q', 'x', 'y', 'z'])
    store.apply(delta(4, ['a'], []) as never) // разворот b удалён
    expect([...store.used].sort()).toEqual(['x', 'y'])
  })

  it('пустая дельта (холостая правка) вид не трогает', async () => {
    const store = await load()
    store.apply(full(1, [sp('a')]) as never)
    const v = store.view
    store.apply({ ...base, seq: 1, base: 1, order: ['a'], spreads: [] } as never)
    expect(store.view).toBe(v)
  })

  it('дельта не к тому виду — один GetView, пока он в полёте', async () => {
    const store = await load()
    store.apply(full(1, [sp('a')]) as never)
    let done: (v: unknown) => void = () => {}
    getView.mockReturnValueOnce(new Promise((r) => (done = r)))
    handlers.view(delta(5, ['a'], [])) // base 4 ≠ 1
    handlers.view(delta(6, ['a'], [])) // за время просьбы — ещё одна дельта
    expect(getView).toHaveBeenCalledTimes(1)
    done(full(6, [sp('a', 'свежий')]))
    await flush()
    expect(getView).toHaveBeenCalledTimes(1)
    expect(store.view.spreads[0].name).toBe('свежий')
  })

  it('ответ GetView старше пришедших дельт — спрашиваем ещё раз', async () => {
    const store = await load()
    store.apply(full(1, [sp('a')]) as never)
    getView.mockResolvedValueOnce(full(4, [sp('a', 'старый')])).mockResolvedValueOnce(full(7, [sp('a', 'новый')]))
    handlers.view(delta(5, ['a'], []))
    handlers.view(delta(7, ['a'], []))
    await flush()
    expect(getView).toHaveBeenCalledTimes(2)
    expect(store.view.spreads[0].name).toBe('новый')
  })

  it('run дожидается пересинхрона: после него вызывающий видит новый вид и rev', async () => {
    const store = await load()
    store.apply(full(1, [sp('a')]) as never)
    let done: (v: unknown) => void = () => {}
    getView.mockReturnValueOnce(new Promise((r) => (done = r)))
    let res: boolean | null = null
    const run = store.run(Promise.resolve(delta(5, ['a', 'b'], [sp('b')], { rev: 7 }))).then((ok) => (res = ok)) // base 4 ≠ 1
    await flush()
    expect(res).toBeNull() // GetView ещё в пути: run не вернулся
    done(full(5, [sp('a'), sp('b')], { rev: 7 }))
    await run
    expect(res).toBe(true)
    expect(store.view.rev).toBe(7)
    expect(store.view.spreads.map((s) => s.id)).toEqual(['a', 'b'])
  })

  it('take дожидается пересинхрона: после него вызывающий видит новый вид и rev', async () => {
    const store = await load()
    store.apply(full(1, [sp('a')]) as never)
    let done: (v: unknown) => void = () => {}
    getView.mockReturnValueOnce(new Promise((r) => (done = r)))
    let res: boolean | null = null
    const take = store.take(delta(5, ['a', 'b'], [sp('b')], { rev: 7 }) as never).then((ok) => (res = ok)) // base 4 ≠ 1
    await flush()
    expect(res).toBeNull()
    expect(store.view.rev).toBe(1) // apply тут бы вернулся со старым видом
    done(full(5, [sp('a'), sp('b')], { rev: 7 }))
    await take
    expect(res).toBe(true)
    expect(store.view.rev).toBe(7)
  })

  it('take при ошибке пересинхрона — false и красный тост; без расхождения — true без GetView', async () => {
    const store = await load()
    store.apply(full(1, [sp('a')]) as never)
    expect(await store.take(delta(2, ['a'], [sp('a', 'x')], { rev: 2 }) as never)).toBe(true)
    expect(getView).not.toHaveBeenCalled()
    getView.mockRejectedValueOnce(new Error('нет связи'))
    expect(await store.take(delta(9, ['a'], []) as never)).toBe(false)
    expect(store.toast?.kind).toBe('error')
  })

  it('run при ошибке пересинхрона — false и красный тост', async () => {
    const store = await load()
    store.apply(full(1, [sp('a')]) as never)
    getView.mockRejectedValueOnce(new Error('нет связи'))
    expect(await store.run(Promise.resolve(delta(5, ['a'], [])))).toBe(false)
    expect(store.toast?.kind).toBe('error')
  })

  it('run без расхождения цепочки не ходит в GetView', async () => {
    const store = await load()
    store.apply(full(1, [sp('a')]) as never)
    expect(await store.run(Promise.resolve(delta(2, ['a'], [sp('a', 'x')], { rev: 2 })))).toBe(true)
    expect(store.view.rev).toBe(2)
    expect(getView).not.toHaveBeenCalled()
  })

  it('разворота нет нигде — GetView', async () => {
    const store = await load()
    store.apply(full(1, [sp('a')]) as never)
    getView.mockResolvedValueOnce(full(2, [sp('a'), sp('z')]))
    handlers.view(delta(2, ['a', 'z'], []))
    await flush()
    expect(getView).toHaveBeenCalledTimes(1)
    expect(store.view.spreads.map((s) => s.id)).toEqual(['a', 'z'])
  })

  it('полный вид с seq не больше своего — пропуск; seq 0 — применяется', async () => {
    const store = await load()
    store.apply(full(5, [sp('a')]) as never)
    store.apply(full(5, [sp('a'), sp('b')]) as never)
    expect(store.view.spreads.length).toBe(1)
    store.apply(full(0, [sp('a'), sp('b')]) as never)
    expect(store.view.spreads.length).toBe(2)
  })

  it('после вида вне цепочки (seq 0) следующая дельта просит GetView', async () => {
    const store = await load()
    store.apply(full(5, [sp('a')]) as never)
    store.apply(full(0, [sp('a')]) as never)
    getView.mockResolvedValueOnce(full(6, [sp('a')]))
    handlers.view(delta(6, ['a'], []))
    await flush()
    expect(getView).toHaveBeenCalledTimes(1)
  })

  it('ошибка GetView — красный тост, следующая дельта снова просит вид', async () => {
    const store = await load()
    store.apply(full(1, [sp('a')]) as never)
    getView.mockRejectedValueOnce(new Error('нет связи'))
    handlers.view(delta(5, ['a'], []))
    await flush()
    expect(store.toast?.kind).toBe('error')
    getView.mockResolvedValueOnce(full(5, [sp('a')]))
    handlers.view(delta(5, ['a'], []))
    await flush()
    expect(getView).toHaveBeenCalledTimes(2)
  })
})

describe('событие saved', () => {
  it('{dirty, path} ставится в вид, развороты те же', async () => {
    const store = await load()
    store.apply(full(1, [sp('a')], { dirty: true }) as never)
    const a = store.view.spreads[0]
    handlers.saved({ dirty: false, path: 'D:/x.salbum' })
    expect(store.view.dirty).toBe(false)
    expect(store.view.path).toBe('D:/x.salbum')
    expect(store.view.spreads[0]).toBe(a)
  })
})
