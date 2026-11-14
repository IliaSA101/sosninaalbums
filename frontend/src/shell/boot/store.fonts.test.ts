import { describe, expect, it, vi } from 'vitest'

// Шрифты (082): событие `fonts` заменяет список и зовёт хук с признаком «свежее»; GetState — без него.
const handlers: Record<string, (...a: unknown[]) => void> = {}
vi.mock('../../../wailsjs/runtime/runtime', () => ({
  EventsOn: (name: string, f: (...a: unknown[]) => void) => {
    handlers[name] = f
    return () => {}
  },
}))
const MY = {
  id: 'my:0123456789abcdef',
  name: 'Lato',
  css: 'SA my 0123456789abcdef',
  group: 'my',
  faces: [{ file: '0123456789abcdef.ttf', bold: false, italic: false }],
}
const dropFonts = vi.hoisted(() => vi.fn(async (faces: string[]) => faces))
vi.mock('../../../wailsjs/go/app/App', () => ({
  DropFonts: dropFonts,
  Marks: async () => ({}),
  GetState: async () => ({ view: { rev: 1, marksRev: 1, folders: [], spreads: [] }, photos: [], fonts: [MY] }),
}))
vi.stubGlobal('window', globalThis)

const { store } = await import('../../base/core/store.svelte')
const { wire } = await import('./wire')

describe('событие fonts', () => {
  it('GetState — список без признака свежести; событие заменяет список и зовёт хук с fresh', async () => {
    const calls: [string[], boolean | undefined][] = []
    store.plug({ ...store.hooks, fontsLoaded: (f, fresh) => void calls.push([f.map((x) => x.id), fresh]) })
    await store.init()
    expect(store.fonts.map((f) => f.id)).toEqual([MY.id])
    expect(calls).toEqual([[[MY.id], undefined]])

    handlers.fonts([{ ...MY, id: 'my:fedcba9876543210' }, MY])
    expect(store.fonts.map((f) => f.id)).toEqual(['my:fedcba9876543210', MY.id])
    expect(calls.at(-1)).toEqual([['my:fedcba9876543210', MY.id], true])
  })
  it('null от Go (пустой список) — пустой список, не падение', () => {
    handlers.fonts(null)
    expect(store.fonts).toEqual([])
  })
})

describe('State.freshFonts через wire() (082-14)', () => {
  const file = 'fedcba9876543210.ttf'
  // Окно программы: файл не открывается — проверка должна откатить его через DropFonts.
  const stubFace = (bad: boolean) =>
    vi.stubGlobal(
      'FontFace',
      class {
        async load() {
          if (bad) throw new Error('OTS')
          return this
        }
      },
    )

  it('открытие через replaceState: отказанное окном начертание уходит в DropFonts', async () => {
    wire()
    stubFace(true)
    dropFonts.mockClear()
    const view = { rev: 1, marksRev: 1, folders: [], spreads: [] }
    await store.replaceState(Promise.resolve({ view, photos: [], info: 'Добавлены шрифты из папки альбома', freshFonts: [file] }))
    await vi.waitFor(() => expect(dropFonts).toHaveBeenCalledWith([file]))
    expect(store.toast).toMatchObject({ kind: 'info' })
    vi.unstubAllGlobals()
    vi.stubGlobal('window', globalThis)
  })
  it('все начертания открылись — Go не зовётся', async () => {
    wire()
    stubFace(false)
    dropFonts.mockClear()
    store.hooks.freshFonts(['0000000000000001.otf'])
    await new Promise((r) => setTimeout(r, 0))
    expect(dropFonts).not.toHaveBeenCalled()
    vi.unstubAllGlobals()
    vi.stubGlobal('window', globalThis)
  })
  it('ошибка DropFonts не падает наружу', async () => {
    wire()
    stubFace(true)
    dropFonts.mockRejectedValueOnce(new Error('нет метода'))
    store.hooks.freshFonts(['0000000000000002.ttf'])
    await new Promise((r) => setTimeout(r, 0))
    vi.unstubAllGlobals()
    vi.stubGlobal('window', globalThis)
  })
})
