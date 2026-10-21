import { beforeEach, describe, expect, it, vi } from 'vitest'

// Обработчики событий Go, которые store.init() кладёт в EventsOn.
const handlers: Record<string, (...a: unknown[]) => void> = {}
let resolveState: (st: unknown) => void = () => {}
// Ответы MissingNames по порядку вызовов.
const namesCalls: ((n: Record<string, string>) => void)[] = []

vi.mock('../../../wailsjs/runtime/runtime', () => ({
  EventsOn: (name: string, fn: (...a: unknown[]) => void) => {
    handlers[name] = fn
    return () => {}
  },
}))
vi.mock('../../../wailsjs/go/app/App', () => ({
  GetState: () => new Promise((r) => (resolveState = r)),
  MissingNames: () => new Promise<Record<string, string>>((r) => namesCalls.push(r)),
}))

vi.stubGlobal('window', globalThis) // notify ставит таймер на window
const { store } = await import('./store.svelte')

const photos = (n: number) => Array.from({ length: n }, (_, i) => ({ id: 'p' + i }))
const idle = { active: false, walking: false, found: 0, read: 0 }
const view = { spreads: [], folders: [] }

beforeEach(() => {
  for (const k of Object.keys(handlers)) delete handlers[k]
  namesCalls.length = 0
})

describe('store.init: события раньше ответа GetState', () => {
  it('library и scan не затираются устаревшим снимком', async () => {
    const done = store.init()
    handlers.library(photos(36))
    handlers.scan({ ...idle })
    resolveState({ view, photos: [], scan: { ...idle, active: true }, missing: [] })
    await done
    expect(store.photos.length).toBe(36)
    expect(store.scan.active).toBe(false)
    expect(store.loaded).toBe(true)
  })

  it('missing не затирается устаревшим снимком', async () => {
    const done = store.init()
    handlers.missing(['p1', 'p2'])
    resolveState({ view, photos: [], missing: [] })
    await done
    expect(store.missing).toEqual(['p1', 'p2'])
  })

  it('без событий снимок применяется', async () => {
    const done = store.init()
    resolveState({ view, photos: photos(3), scan: { ...idle, active: true }, missing: ['p0'] })
    await done
    expect(store.photos.length).toBe(3)
    expect(store.scan.active).toBe(true)
    expect(store.missing).toEqual(['p0'])
  })
})

describe('store: имена пропавших', () => {
  it('берёт имена у Go при смене missing, запоздавший ответ отбрасывается', async () => {
    const done = store.init()
    resolveState({ view, photos: [], missing: [] })
    await done
    handlers.missing(['a'])
    handlers.missing(['a', 'b'])
    expect(namesCalls.length).toBe(2)
    namesCalls[1]({ a: 'a.jpg', b: 'b.jpg' })
    await Promise.resolve()
    namesCalls[0]({ a: 'старый.jpg' })
    await Promise.resolve()
    expect(store.missingNames).toEqual({ a: 'a.jpg', b: 'b.jpg' })
    handlers.missing([])
    expect(store.missingNames).toEqual({})
    expect(namesCalls.length).toBe(2)
  })
})

describe('store.replaceState: отмена диалога «Открыть»', () => {
  const v = (path: string, rev: number) => ({ spreads: [], folders: [], path, rev })

  it('cancelled: true ничего не сбрасывает', async () => {
    store.apply(v('a.salbum', 3) as never)
    const seq = store.openSeq
    const nav = store.navSeq
    store.selected.add('p1')
    expect(await store.replaceState(Promise.resolve({ view: v('a.salbum', 3), photos: [], cancelled: true }))).toBe(true)
    expect(store.openSeq).toBe(seq)
    expect(store.navSeq).toBe(nav)
    expect(store.selected.has('p1')).toBe(true)
  })

  it('удачное открытие: view пришёл раньше ответа — состояние всё равно сбрасывается', async () => {
    const done0 = store.init() // регистрирует handlers.view
    resolveState({ view: v('a.salbum', 3), photos: [] })
    await done0
    const seq = store.openSeq
    const nav = store.navSeq
    store.selected.add('p1')
    let resolve: (st: unknown) => void = () => {}
    const done = store.replaceState(new Promise((r) => (resolve = r)))
    const next = v('b.salbum', 0)
    handlers.view(next) // как Go: emitState до ответа метода
    resolve({ view: next, photos: [] })
    expect(await done).toBe(true)
    expect(store.openSeq).toBe(seq + 1)
    expect(store.navSeq).toBe(nav + 1) // go(0)
    expect(store.selected.size).toBe(0)
  })

  it('повторное открытие того же файла без правок — открытие, не отмена', async () => {
    store.apply(v('a.salbum', 3) as never)
    const seq = store.openSeq
    store.selected.add('p1')
    await store.replaceState(Promise.resolve({ view: v('a.salbum', 3), photos: [] }))
    expect(store.openSeq).toBe(seq + 1)
    expect(store.selected.size).toBe(0)
  })
})

describe('State.info и freshFonts (082)', () => {
  const fresh: string[][] = []
  beforeEach(() => {
    fresh.length = 0
    store.toast = null
    store.plug({ ...store.hooks, freshFonts: (files) => void fresh.push(files) })
  })

  it('init: info — тост вида info, не ошибка', async () => {
    const done = store.init()
    resolveState({ view, photos: [], info: 'Добавлены шрифты из папки альбома: «Lato»' })
    await done
    expect(store.toast).toMatchObject({ text: 'Добавлены шрифты из папки альбома: «Lato»', kind: 'info' })
  })

  it('init: notice остаётся ошибкой; при обоих последним показан info', async () => {
    const done = store.init()
    resolveState({ view, photos: [], notice: 'не открылся', info: 'Добавлены шрифты' })
    await done
    expect(store.toast).toMatchObject({ text: 'Добавлены шрифты', kind: 'info' })
    const again = store.init()
    resolveState({ view, photos: [], notice: 'не открылся' })
    await again
    expect(store.toast).toMatchObject({ text: 'Не открылся', kind: 'error' })
  })

  it('init: freshFonts зовёт проверку окном, пустые и отсутствующие — нет', async () => {
    const done = store.init()
    resolveState({ view, photos: [], freshFonts: ['0123456789abcdef.ttf'] })
    await done
    expect(fresh).toEqual([['0123456789abcdef.ttf']])
    const a = store.init()
    resolveState({ view, photos: [], freshFonts: [] })
    await a
    const b = store.init()
    resolveState({ view, photos: [] })
    await b
    expect(fresh).toHaveLength(1)
  })

  it('replaceState (меню, «Последние»): info — тост info', async () => {
    await store.replaceState(Promise.resolve({ view, photos: [], info: 'Добавлены шрифты из папки альбома' }))
    expect(store.toast).toMatchObject({ text: 'Добавлены шрифты из папки альбома', kind: 'info' })
  })

  it('replaceState: freshFonts зовёт проверку; без них и при отмене диалога — нет', async () => {
    await store.replaceState(Promise.resolve({ view, photos: [], freshFonts: ['aaaaaaaaaaaaaaaa.otf'] }))
    expect(fresh).toEqual([['aaaaaaaaaaaaaaaa.otf']])
    await store.replaceState(Promise.resolve({ view, photos: [] }))
    await store.replaceState(Promise.resolve({ cancelled: true, info: 'не должно показаться', freshFonts: ['x'] }))
    expect(fresh).toHaveLength(1)
    expect(store.toast?.text).not.toBe('не должно показаться')
  })

  it('replaceState: ошибка открытия — тост ошибки, проверки нет', async () => {
    await store.replaceState(Promise.reject('нет доступа'))
    expect(store.toast).toMatchObject({ kind: 'error' })
    expect(fresh).toHaveLength(0)
  })
})
