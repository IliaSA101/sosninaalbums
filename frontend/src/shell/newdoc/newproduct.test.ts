import { describe, expect, it, vi } from 'vitest'
import { pickedDims, sheetDims, sizeHint } from './newproduct'

// Go подменён: NewVignetteSheet отдаёт состояние с листом, NewProject — обычный альбом.
const calls: unknown[][] = []
const state = (sheet: boolean) => ({ view: { rev: 1, folders: [], spreads: [{ id: 'a' }], settings: { sheet } }, photos: [] })

vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', () => ({
  NewVignetteSheet: async (w: number, h: number) => {
    calls.push(['sheet', w, h])
    return state(true)
  },
  NewProject: async (kind: string) => {
    calls.push(['album', kind])
    return state(false)
  },
}))
vi.mock('../../editor/text/fonts', () => ({ loadFonts: async () => {} }))
vi.stubGlobal('window', globalThis)

const { newDoc } = await import('./newdoc.svelte')
const { store } = await import('../../base/core/store.svelte')

describe('размер листа', () => {
  it('пределы стороны — как у Go (50–1000 мм)', () => {
    expect(sheetDims(204, 304)).toEqual({ w: 204, h: 304 })
    expect(sheetDims(49, 304)).toBeNull()
    expect(sheetDims(204, 1001)).toBeNull()
    expect(sheetDims(null, 300)).toBeNull()
    expect(sheetDims(NaN, 300)).toBeNull()
  })

  it('пресеты 20×30 и 30×40; «свой» — из полей', () => {
    expect(pickedDims(0, null, null)).toEqual({ w: 204, h: 304 })
    expect(pickedDims(1, null, null)).toEqual({ w: 304, h: 404 })
    expect(pickedDims('custom', 250, 350)).toEqual({ w: 250, h: 350 })
    expect(pickedDims('custom', 10, 350)).toBeNull()
    expect(pickedDims(5, 250, 350)).toBeNull()
  })

  it('подсказка — только у «своего» с плохим размером', () => {
    expect(sizeHint(0, null, null)).toBe('')
    expect(sizeHint('custom', 250, 350)).toBe('')
    expect(sizeHint('custom', 10, 350)).toContain('50')
  })
})

describe('newDoc', () => {
  it('альбом создаётся всегда; лист — только с годным размером', () => {
    newDoc.kind = 'album'
    expect(newDoc.ok).toBe(true)
    newDoc.kind = 'sheet'
    newDoc.choose('custom')
    newDoc.w = 20
    expect(newDoc.ok).toBe(false)
    newDoc.w = 250
    newDoc.h = 350
    expect(newDoc.ok).toBe(true)
  })

  it('выбор пресета подставляет его размер в поля', () => {
    newDoc.kind = 'sheet'
    newDoc.choose(1)
    expect([newDoc.w, newDoc.h]).toEqual([304, 404])
  })

  it('«Создать» зовёт NewVignetteSheet с размером и заменяет проект на лист', async () => {
    calls.length = 0
    newDoc.kind = 'sheet'
    newDoc.choose(0)
    expect(await newDoc.create()).toBe(true)
    expect(calls).toEqual([['sheet', 204, 304]])
    expect(store.view.settings.sheet).toBe(true)
  })

  it('без годного размера лист не создаётся', async () => {
    calls.length = 0
    newDoc.kind = 'sheet'
    newDoc.choose('custom')
    newDoc.w = null
    expect(await newDoc.create()).toBe(false)
    expect(calls).toEqual([])
  })

  it('альбом: NewProject("album"), лист не затронут', async () => {
    calls.length = 0
    newDoc.kind = 'album'
    expect(await newDoc.create()).toBe(true)
    expect(calls).toEqual([['album', 'album']])
    expect(store.view.settings.sheet).toBe(false)
  })

  it('двойной щелчок: пока идёт создание, второй вызов ничего не создаёт', async () => {
    calls.length = 0
    newDoc.kind = 'album'
    const [a, b] = await Promise.all([newDoc.create(), newDoc.create()])
    expect([a, b]).toEqual([true, false])
    expect(calls).toEqual([['album', 'album']])
    expect(newDoc.busy).toBe(false)
  })
})
