import { beforeEach, describe, expect, it, vi } from 'vitest'

// Ворота копии (098) в store: «Добавить» и двойной щелчок по плитке ленты (addToCurrent) и новый разворот из ленты
// идут через hooks.gate; Go получает то, что вернули ворота; «Отмена» — ни вызова Go, ни снятия выделения.
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', () => ({ AddPhotos: vi.fn(), NewSpread: vi.fn(), ForgetFileNames: vi.fn() }))
vi.stubGlobal('window', globalThis)

const Api = await import('../../../wailsjs/go/app/App')
const { store } = await import('./store.svelte')
const { noHooks } = await import('./hooks')

const AddPhotos = vi.mocked(Api.AddPhotos)
const NewSpread = vi.mocked(Api.NewSpread)
const gate = vi.fn()
const dropGrid = vi.fn()
const sp = (id: string, photos: string[], grid = false) => ({ id, photos, grid })
const view = (rev: number, ...spreads: object[]) => ({ rev, marksRev: 0, folders: [], spreads, settings: { sheet: false } }) as never

beforeEach(() => {
  vi.clearAllMocks()
  gate.mockImplementation(async (ids: string[]) => ids)
  dropGrid.mockResolvedValue(false)
  store.plug({ ...noHooks, gate, dropGrid })
  store.selected.clear()
  store.toast = null
  store.apply(view(1, sp('s1', ['f1']), sp('s2', ['f2'])))
  store.go(1)
})

describe('addToCurrent: «Добавить» фото из ленты', () => {
  it('свободное фото — ворота пропускают как есть', async () => {
    AddPhotos.mockResolvedValueOnce(view(2, sp('s1', ['f1']), sp('s2', ['f2', 'f9'])))
    await store.addToCurrent(['f9'])
    expect(gate).toHaveBeenCalledExactlyOnceWith(['f9'])
    expect(AddPhotos).toHaveBeenCalledExactlyOnceWith('s2', ['f9'], -1)
  })

  it('«Поставить копию»: Go получает запрос копии, тост «перенесено» не нужен (с места ничего не уходит)', async () => {
    gate.mockResolvedValueOnce(['f1-new'])
    AddPhotos.mockResolvedValueOnce(view(2, sp('s1', ['f1']), sp('s2', ['f2', 'f1-2'])))
    await store.addToCurrent(['f1'])
    expect(AddPhotos).toHaveBeenCalledExactlyOnceWith('s2', ['f1-new'], -1)
    expect(store.toast).toBeNull()
  })

  it('«Перенести сюда»: Go получает запись места, тост «Фото перенесено из …»', async () => {
    gate.mockResolvedValueOnce(['f1'])
    AddPhotos.mockResolvedValueOnce(view(2, sp('s1', []), sp('s2', ['f2', 'f1'])))
    await store.addToCurrent(['f1'])
    expect(store.toast?.text).toBe('Фото перенесено из разворота 1')
  })

  it('«Отмена»: Go не зовём, выделение в ленте остаётся', async () => {
    gate.mockResolvedValueOnce(null)
    store.selected.add('f1')
    await store.addToCurrent(['f1'])
    expect(AddPhotos).not.toHaveBeenCalled()
    expect(store.selected.has('f1')).toBe(true)
    expect(store.toast).toBeNull()
  })

  it('ручной разворот: ворота стоят в dropGrid (хук верха), здесь второй раз не спрашиваем', async () => {
    store.apply(view(2, sp('s1', ['f1']), sp('s2', ['f2'], true)))
    dropGrid.mockResolvedValueOnce(true)
    await store.addToCurrent(['f1'])
    expect(dropGrid).toHaveBeenCalledExactlyOnceWith('s2', ['f1'])
    expect(gate).not.toHaveBeenCalled()
  })

  it('пустой список — ничего', async () => {
    await store.addToCurrent([])
    expect(gate).not.toHaveBeenCalled()
  })
})

describe('newSpread из ленты', () => {
  it('Go получает ответ ворот; «+ Новый разворот» без фото ворот не зовёт', async () => {
    gate.mockResolvedValueOnce(['f1-new'])
    NewSpread.mockResolvedValueOnce(view(2, sp('s1', ['f1']), sp('s2', ['f2']), sp('s3', ['f1-2'])))
    await store.newSpread(2, ['f1'])
    expect(NewSpread).toHaveBeenCalledExactlyOnceWith(2, ['f1-new'])
    NewSpread.mockResolvedValueOnce(view(3, sp('s1', ['f1']), sp('s2', ['f2']), sp('s3', ['f1-2']), sp('s4', [])))
    gate.mockClear()
    await store.addEmptySpread(3)
    expect(gate).not.toHaveBeenCalled()
  })

  it('«Отмена» — разворота нет, текущий прежний', async () => {
    gate.mockResolvedValueOnce(null)
    await store.newSpread(2, ['f1'])
    expect(NewSpread).not.toHaveBeenCalled()
    expect(store.view.spreads).toHaveLength(2)
    expect(store.current).toBe(1)
  })
})
