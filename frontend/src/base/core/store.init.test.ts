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
  SetLevel: () => Promise.resolve({}),
  MissingNames: () => new Promise<Record<string, string>>((r) => namesCalls.push(r)),
}))

vi.stubGlobal('window', globalThis) // notify ставит таймер на window
const { store } = await import('./store.svelte')
const { prefs } = await import('../prefs/prefs.svelte')

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

describe('store.init: unread (117)', () => {
  it('снимок задаёт причины; событие заменяет целиком; null — пусто', async () => {
    const done = store.init()
    resolveState({ view, photos: [], unread: { a: 'файл повреждён' } })
    await done
    expect(store.unread).toEqual({ a: 'файл повреждён' })
    handlers.unread({ b: 'пустой файл' })
    expect(store.unread).toEqual({ b: 'пустой файл' })
    handlers.unread(null)
    expect(store.unread).toEqual({})
  })

  it('событие раньше ответа свежее устаревшего снимка; без поля в снимке — пусто', async () => {
    let done = store.init()
    handlers.unread({ c: 'не картинка' })
    resolveState({ view, photos: [], unread: {} })
    await done
    expect(store.unread).toEqual({ c: 'не картинка' })
    done = store.init()
    resolveState({ view, photos: [] })
    await done
    expect(store.unread).toEqual({})
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

describe('State.newer (112): альбом новее программы', () => {
  const v = (path: string, rev: number) => ({ spreads: [], folders: [], path, rev })
  const newer = { name: 'Свадьба.salbum', draft: false, need: '0.26', mine: '0.25', have: '' }
  const shown: unknown[] = []
  beforeEach(() => {
    shown.length = 0
    store.plug({ ...store.hooks, newer: (i) => void shown.push(i) })
  })

  it('ответ открытия cancelled + newer: окно показано, альбом не тронут', async () => {
    store.apply(v('a.salbum', 3) as never)
    const seq = store.openSeq
    expect(await store.replaceState(Promise.resolve({ view: v('a.salbum', 3), photos: [], cancelled: true, newer }))).toBe(true)
    expect(shown).toEqual([newer])
    expect(store.openSeq).toBe(seq)
  })

  it('просто cancelled (диалог закрыт) — окна нет', async () => {
    await store.replaceState(Promise.resolve({ view: v('a.salbum', 3), photos: [], cancelled: true }))
    expect(shown).toEqual([])
  })

  it('ошибка открытия — окна нет', async () => {
    await store.replaceState(Promise.reject('нет файла'))
    expect(shown).toEqual([])
  })

  it('старт на альбоме новее: первый GetState показывает окно один раз', async () => {
    const done = store.init()
    resolveState({ view, photos: [], missing: [], newer })
    await done
    expect(shown).toEqual([newer])
  })

  it('старт без newer — окна нет', async () => {
    const done = store.init()
    resolveState({ view, photos: [], missing: [] })
    await done
    expect(shown).toEqual([])
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

describe('store.init: стенд QA (State.stand)', () => {
  const start = async (st: Record<string, unknown>) => {
    const done = store.init()
    resolveState({ view, photos: [], missing: [], ...st })
    await done
  }

  it('stand: true — признак стенда', async () => {
    await start({ stand: true })
    expect(store.stand).toBe(true)
  })

  it('нет поля (выпуск) — не стенд', async () => {
    store.stand = true
    await start({})
    expect(store.stand).toBe(false)
  })
})

describe('store.init: запуск двойным щелчком по альбому (112)', () => {
  const start = async (extra: Record<string, unknown>, p: Record<string, unknown>) => {
    const done = store.init()
    resolveState({ view, photos: [], missing: [], prefs: { level: 'simple', showStart: true, first: false, ...p }, ...extra })
    await done
  }

  it('fromFile: стартовое окно с «Продолжить: …» не показывается', async () => {
    await start({ fromFile: true }, {})
    expect(prefs.startOpen).toBe(false)
  })

  it('обычный запуск: окно по флажку «Показывать при запуске» остаётся', async () => {
    await start({}, {})
    expect(prefs.startOpen).toBe(true)
  })

  it('fromFile, но флажок снят: окна и так нет', async () => {
    await start({ fromFile: true }, { showStart: false })
    expect(prefs.startOpen).toBe(false)
  })

  it('fromFile на первом запуске: выбор уровня остаётся, окно открыто', async () => {
    await start({ fromFile: true }, { first: true })
    expect(prefs.startOpen).toBe(true)
  })

  const newer = { name: 'Alpha.salbum', draft: false, need: '0.26', mine: '0.25', have: '0.25.1' }
  const shown: unknown[] = []
  beforeEach(() => {
    shown.length = 0
    store.plug({ ...store.hooks, newer: (i) => void shown.push(i) })
  })

  it('файл новее программы: окно «новее» показано, стартовое закрыто', async () => {
    await start({ newer }, {})
    expect(shown).toEqual([newer])
    expect(prefs.startOpen).toBe(false)
  })

  it('файл новее, флажок «Показывать при запуске» снят: стартового и так нет', async () => {
    await start({ newer }, { showStart: false })
    expect(shown).toEqual([newer])
    expect(prefs.startOpen).toBe(false)
  })

  it('файл новее на первом запуске: выбор уровня остаётся под окном «новее»', async () => {
    await start({ newer }, { first: true })
    expect(shown).toEqual([newer])
    expect(prefs.startOpen).toBe(true)
  })
})
