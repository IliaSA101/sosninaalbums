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

describe('export:done (047)', () => {
  // Регистрирует обработчики; init дозавершаем, чтобы он не висел.
  async function ready() {
    const done = store.init()
    resolveState({ view, photos: [], missing: [] })
    await done
    store.toast = null
    store.exportMemo = null
    store.exportNote = ''
  }
  const memo = ['Фотокнига 30×30', 'Папка: Иванов — 01…20.jpg']

  it('с памяткой — окно «Готово для Фотосферы», тоста нет; сводка цвета остаётся', async () => {
    await ready()
    handlers['export:done']('C:/out/Иванов', '', 'Пересчитано из Adobe RGB в sRGB: 2 фото', memo)
    expect(store.exportMemo).toEqual(memo)
    expect(store.exportNote).toBe('Пересчитано из Adobe RGB в sRGB: 2 фото')
    expect(store.toast).toBeNull()
  })

  it('без памятки (и с пустой) — прежний тост', async () => {
    await ready()
    handlers['export:done']('C:/out', '', '', null)
    expect(store.exportMemo).toBeNull()
    expect(store.toast).toMatchObject({ kind: 'info', text: 'Выгрузка готова' })
    store.toast = null
    handlers['export:done']('C:/out', '', '', [])
    expect(store.exportMemo).toBeNull()
    expect((store.toast as { text: string } | null)?.text).toBe('Выгрузка готова')
  })

  it('ошибка — красный тост, окна нет, даже если памятка пришла', async () => {
    await ready()
    handlers['export:done']('C:/out', 'диск полон', '', memo)
    expect(store.exportMemo).toBeNull()
    expect(store.toast).toMatchObject({ kind: 'error', text: 'Экспорт не удался: диск полон' })
  })
})
