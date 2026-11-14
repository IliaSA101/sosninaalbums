import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushSync } from 'svelte'

// Открытие .salbum извне (112 U2): событие `open:external`, очередь за окнами, State.pending при init.
const handlers: Record<string, (...a: unknown[]) => void> = {}
const off = vi.fn()
const openExternal = vi.fn<() => Promise<unknown>>()
const idle = vi.fn<() => Promise<void>>()
let getState: () => Promise<unknown>

vi.mock('../../../wailsjs/runtime/runtime', () => ({
  EventsOn: (name: string, fn: (...a: unknown[]) => void) => {
    handlers[name] = fn
    return off
  },
}))
vi.mock('../../../wailsjs/go/app/App', () => ({
  GetState: () => getState(),
  OpenExternal: () => openExternal(),
  MissingNames: () => Promise.resolve({}),
}))
vi.mock('../../editor/text/edit.svelte', () => ({ edit: { idle: () => idle() } }))

vi.stubGlobal('window', globalThis) // notify ставит таймер на window
const { store } = await import('../../base/core/store.svelte')
const { jobs } = await import('../../base/core/jobs.svelte')
const { noHooks } = await import('../../base/core/hooks')
const { modalStack } = await import('../../base/modal/modalstack.svelte')
const { prefs } = await import('../../base/prefs/prefs.svelte')
const { extOpen } = await import('./extopen.svelte')

const view = { spreads: [], folders: [], rev: 1, dirty: false, path: '' }
const opened = { view: { ...view, rev: 2, path: 'C:/a/Свадьба.salbum' }, photos: [] }
const cancelled = { view, photos: [], cancelled: true }

// Эффекты Svelte и цепочка промисов: кадр, микрозадачи, ещё кадр.
async function settle() {
  flushSync()
  await new Promise((r) => setTimeout(r, 0))
  flushSync()
  await new Promise((r) => setTimeout(r, 0))
}
const event = (name: string) => handlers['open:external'](name)

let stop: () => void
let ids: number[]
// Окно в стеке, как Modal: id возвращается, чтобы закрыть.
const openWindow = () => {
  const id = modalStack.push()
  ids.push(id)
  return id
}

beforeEach(() => {
  for (const k of Object.keys(handlers)) delete handlers[k]
  vi.clearAllMocks()
  idle.mockResolvedValue(undefined)
  openExternal.mockResolvedValue(cancelled)
  getState = () => Promise.resolve({ view, photos: [], missing: [] })
  ids = []
  store.toast = null
  store.plug({ ...noHooks, external: (n) => extOpen.request(n) })
  prefs.startOpen = false
  stop = extOpen.listen()
})

afterEach(() => {
  stop()
  for (const id of ids) modalStack.remove(id)
  flushSync()
  extOpen.name = null
  jobs.openShown = false
  prefs.startOpen = false
  vi.restoreAllMocks()
})

describe('extOpen: событие open:external', () => {
  it('нет окон: ждёт edit.idle и зовёт OpenExternal', async () => {
    let release = () => {}
    idle.mockReturnValue(new Promise<void>((r) => (release = r)))
    event('Свадьба.salbum')
    await settle()
    expect(idle).toHaveBeenCalledTimes(1)
    expect(openExternal).not.toHaveBeenCalled() // набранный текст ещё уходит в Go
    release()
    await settle()
    expect(openExternal).toHaveBeenCalledTimes(1)
  })

  it('открыто окно: ждёт, говорит об этом тостом, после закрытия открывает один раз', async () => {
    const id = openWindow()
    event('Свадьба.salbum')
    await settle()
    expect(openExternal).not.toHaveBeenCalled()
    expect(store.toast?.text).toBe('Альбом «Свадьба» откроется, когда закроете окно')
    expect(store.toast?.kind).toBe('info')
    modalStack.remove(id)
    await settle()
    expect(openExternal).toHaveBeenCalledTimes(1)
    // следующее окно и его закрытие ничего не открывают: очередь пуста
    const again = openWindow()
    await settle()
    modalStack.remove(again)
    await settle()
    expect(openExternal).toHaveBeenCalledTimes(1)
  })

  it('открыто только окно хода «Открываем альбом»: тоста нет, после его ухода открывает', async () => {
    jobs.openShown = true
    const id = openWindow()
    event('Свадьба.salbum')
    await settle()
    expect(store.toast).toBeNull()
    expect(openExternal).not.toHaveBeenCalled()
    modalStack.remove(id)
    await settle()
    expect(openExternal).toHaveBeenCalledTimes(1)
  })

  it('окно хода и другое окно: тост остаётся — второе закрыть можно', async () => {
    jobs.openShown = true
    openWindow()
    openWindow()
    event('Свадьба.salbum')
    await settle()
    expect(store.toast?.text).toBe('Альбом «Свадьба» откроется, когда закроете окно')
  })

  it('два окна друг на друге: открывает после закрытия последнего', async () => {
    const a = openWindow()
    const b = openWindow()
    event('A.salbum')
    await settle()
    modalStack.remove(b)
    await settle()
    expect(openExternal).not.toHaveBeenCalled()
    modalStack.remove(a)
    await settle()
    expect(openExternal).toHaveBeenCalledTimes(1)
  })

  it('стартовое окно: закрывается через prefs.closeStart, тоста нет, открывает', async () => {
    prefs.startOpen = true
    const id = openWindow()
    const close = vi.spyOn(prefs, 'closeStart').mockImplementation(() => {
      prefs.startOpen = false
      modalStack.remove(id) // Modal стартового окна уходит из стека вместе с окном
    })
    event('Свадьба.salbum')
    await settle()
    expect(close).toHaveBeenCalledTimes(1)
    expect(store.toast).toBeNull()
    expect(openExternal).toHaveBeenCalledTimes(1)
  })

  it('программа уже с альбомом: после открытия извне стартовое окно не показывается', async () => {
    event('Свадьба.salbum') // окна не было: startOpen уже false
    await settle()
    expect(openExternal).toHaveBeenCalledTimes(1)
    expect(prefs.startOpen).toBe(false)
  })

  it('стартовое окно и справочник поверх него: окно закрыто, справочник ещё держит очередь', async () => {
    prefs.startOpen = true
    const start = openWindow()
    const help = openWindow()
    vi.spyOn(prefs, 'closeStart').mockImplementation(() => {
      prefs.startOpen = false
      modalStack.remove(start)
    })
    event('Свадьба.salbum')
    await settle()
    expect(openExternal).not.toHaveBeenCalled()
    expect(store.toast?.text).toContain('откроется, когда закроете окно')
    modalStack.remove(help)
    await settle()
    expect(openExternal).toHaveBeenCalledTimes(1)
  })

  it('два события сразу: открывается один раз (в очереди Go побеждает последний)', async () => {
    event('A.salbum')
    event('B.salbum')
    await settle()
    expect(openExternal).toHaveBeenCalledTimes(1)
  })

  it('два события под окном: тост называет последний, открытие одно', async () => {
    const id = openWindow()
    event('A.salbum')
    event('B.salbum')
    await settle()
    expect(store.toast?.text).toBe('Альбом «B» откроется, когда закроете окно')
    modalStack.remove(id)
    await settle()
    expect(openExternal).toHaveBeenCalledTimes(1)
  })

  it('два события по очереди: открытия идут друг за другом, второе «cancelled» ничего не меняет', async () => {
    openExternal.mockResolvedValueOnce(opened).mockResolvedValueOnce(cancelled)
    event('Свадьба.salbum')
    await settle()
    const seq = store.openSeq
    event('Свадьба.salbum')
    await settle()
    expect(openExternal).toHaveBeenCalledTimes(2)
    expect(store.openSeq).toBe(seq)
    expect(store.view.path).toBe('C:/a/Свадьба.salbum')
  })

  it('пустое имя в событии игнорируется', async () => {
    event('')
    await settle()
    expect(openExternal).not.toHaveBeenCalled()
  })

  it('после отписки событие не слушается', () => {
    stop()
    expect(off).toHaveBeenCalledTimes(1)
  })
})

describe('extOpen: ответ OpenExternal', () => {
  it('открыт: альбом заменён, openSeq вырос', async () => {
    openExternal.mockResolvedValue(opened)
    const seq = store.openSeq
    event('Свадьба.salbum')
    await settle()
    expect(store.openSeq).toBe(seq + 1)
    expect(store.view.path).toBe('C:/a/Свадьба.salbum')
    expect(store.toast).toBeNull()
  })

  it('cancelled (очередь пуста или альбом уже открыт): ничего не меняется, тоста нет', async () => {
    const seq = store.openSeq
    const before = store.view
    event('Свадьба.salbum')
    await settle()
    expect(openExternal).toHaveBeenCalledTimes(1)
    expect(store.openSeq).toBe(seq)
    expect(store.view).toBe(before)
    expect(store.toast).toBeNull()
  })

  it('ошибка: тост словами с заглавной, как у «Открыть»; следующий файл открывается', async () => {
    openExternal.mockRejectedValueOnce(new Error('не альбом: файл повреждён'))
    event('Битый.salbum')
    await settle()
    expect(store.toast).toMatchObject({ text: 'Не альбом: файл повреждён', kind: 'error' })
    openExternal.mockResolvedValueOnce(opened)
    const seq = store.openSeq
    event('Свадьба.salbum')
    await settle()
    expect(store.openSeq).toBe(seq + 1)
  })
})

describe('extOpen: State.pending при init', () => {
  it('файл в очереди с запуска: init просит открыть, OpenExternal зовётся один раз', async () => {
    getState = () => Promise.resolve({ view, photos: [], missing: [], pending: 'Свадьба.salbum' })
    await store.init()
    await settle()
    expect(openExternal).toHaveBeenCalledTimes(1)
  })

  it('pending и событие о том же файле: второй вызов получает cancelled, альбом открыт один раз', async () => {
    getState = () => Promise.resolve({ view, photos: [], missing: [], pending: 'Свадьба.salbum' })
    openExternal.mockResolvedValueOnce(opened).mockResolvedValueOnce(cancelled)
    await store.init()
    await settle()
    const seq = store.openSeq
    event('Свадьба.salbum')
    await settle()
    expect(openExternal).toHaveBeenCalledTimes(2)
    expect(store.openSeq).toBe(seq)
  })

  it('pending при стартовом окне: окно закрывается, альбом открывается', async () => {
    getState = async () => {
      prefs.startOpen = true // prefs.load в init открыл бы окно
      return { view, photos: [], missing: [], pending: 'Свадьба.salbum' }
    }
    const id = openWindow()
    const close = vi.spyOn(prefs, 'closeStart').mockImplementation(() => {
      prefs.startOpen = false
      modalStack.remove(id)
    })
    await store.init()
    await settle()
    expect(close).toHaveBeenCalledTimes(1)
    expect(openExternal).toHaveBeenCalledTimes(1)
  })

  it('без pending init ничего не открывает', async () => {
    await store.init()
    await settle()
    expect(openExternal).not.toHaveBeenCalled()
  })
})
