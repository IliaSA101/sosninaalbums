import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

// Тост «Раскладка стала ручной» (158-5): правка закрепила разворот, тост с «Вернуть» привязан к ревизии;
// Undo (кнопка или Ctrl+Z) его гасит. Go подменён: каждая правка отвечает видом нужной ревизии.
const view = (rev: number, grid: boolean) => ({ rev, folders: [], spreads: [{ id: 'a', photos: [] as string[], grid }] })
const undo = vi.hoisted(() => vi.fn())

vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', () => ({ Undo: undo }))
vi.stubGlobal('window', globalThis)

const { store } = await import('../../base/core/store.svelte')
const { gridAct } = await import('./gridact.svelte')

beforeEach(() => {
  store.toast = null
  store.apply(view(1, false) as never)
  undo.mockReset()
  undo.mockImplementation(async () => view(3, false))
})
afterEach(() => {
  store.toast = null
})

describe('placeEdit: тост «Раскладка стала ручной» (158-5)', () => {
  it('placeEdit: «Раскладка стала ручной» гаснет после Undo', async () => {
    expect(await gridAct.placeEdit('a', Promise.resolve(view(2, true)))).toBe(true)
    expect(store.toast?.text).toContain('Раскладка стала ручной')
    expect(store.toast?.action?.label).toBe('Вернуть')
    expect(store.toast?.rev).toBe(2)
    store.apply(view(3, false) as never) // Ctrl+Z: ревизия выросла
    expect(store.toast).toBeNull()
  })

  it('«Вернуть» откатывает правку, и тост гаснет', async () => {
    await gridAct.placeEdit('a', Promise.resolve(view(2, true)))
    await store.toast!.action!.run()
    expect(undo).toHaveBeenCalledTimes(1)
    expect(store.toast).toBeNull()
  })

  it('разворот уже ручной — тоста нет', async () => {
    store.apply(view(2, true) as never)
    await gridAct.placeEdit('a', Promise.resolve(view(3, true)))
    expect(store.toast).toBeNull()
  })
})
