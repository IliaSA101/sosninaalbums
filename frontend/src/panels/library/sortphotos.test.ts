import { describe, expect, it, vi } from 'vitest'

// Порядок ленты (075-14): чистая сортировка и её связка со store (лента, Shift/Ctrl+A, автосборка берут store.visible).
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', () => ({ Marks: async () => ({}) }))
vi.mock('../../editor/text/fonts', () => ({ loadFonts: async () => {} }))
vi.stubGlobal('window', globalThis)

const { sortPhotos } = await import('./sortphotos')
const { store } = await import('../../base/core/store.svelte')
const { modes } = await import('../../base/prefs/view.svelte')
const { wire } = await import('../../shell/boot/wire')
const { libfilter } = await import('../marks/libfilter.svelte')

const ph = (id: string, shot?: number) => ({ id, path: `D:/a/${id}.jpg`, name: id, w: 1, h: 1, shot })
const ids = (l: { id: string }[]) => l.map((p) => p.id)

describe('sortPhotos', () => {
  it('по имени — тот же массив, без копии', () => {
    const l = [ph('b', 5), ph('a', 1)]
    expect(sortPhotos(l, 'name')).toBe(l)
  })

  it('по дате съёмки: раньше — выше', () => {
    expect(ids(sortPhotos([ph('a', 30), ph('b', 10), ph('c', 20)], 'shot'))).toEqual(['b', 'c', 'a'])
  })

  it('одинаковая дата: порядок имён Go сохраняется (устойчиво)', () => {
    expect(ids(sortPhotos([ph('a', 7), ph('b', 3), ph('c', 7), ph('d', 3)], 'shot'))).toEqual(['b', 'd', 'a', 'c'])
  })

  it('фото без ключа (0 или нет поля) идёт первым, между собой — по имени', () => {
    expect(ids(sortPhotos([ph('a', 5), ph('b'), ph('c', 0)], 'shot'))).toEqual(['b', 'c', 'a'])
  })

  it('пусто и один элемент не ломаются', () => {
    expect(sortPhotos([], 'shot')).toEqual([])
    const one = [ph('a', 1)]
    expect(sortPhotos(one, 'shot')).toBe(one)
  })

  it('не меняет исходный массив', () => {
    const l = [ph('a', 2), ph('b', 1)]
    sortPhotos(l, 'shot')
    expect(ids(l)).toEqual(['a', 'b'])
  })
})

describe('порядок ленты в store', () => {
  wire()
  const setup = () => {
    store.filter = 'all'
    store.selected.clear()
    store.photos = [ph('a', 30), ph('b', 10), ph('c', 20)] as never
    store.apply({ rev: 1, marksRev: 1, folders: [], spreads: [] } as never)
  }

  it('смена порядка перестраивает ленту, выделенные и «Не в альбоме» идут в порядке на экране', () => {
    setup()
    modes.setLibSort('name')
    expect(ids(store.visible)).toEqual(['a', 'b', 'c'])
    store.selected.add('a')
    store.selected.add('c')
    modes.setLibSort('shot')
    expect(ids(store.visible)).toEqual(['b', 'c', 'a'])
    expect(store.selectedIds).toEqual(['c', 'a'])
    expect(ids(store.free)).toEqual(['b', 'c', 'a'])
    modes.setLibSort('name')
    expect(ids(store.visible)).toEqual(['a', 'b', 'c'])
  })

  it('«Сбросить» фильтра порядок не трогает', () => {
    setup()
    modes.setLibSort('shot')
    libfilter.reset()
    expect(modes.libSort).toBe('shot')
    expect(libfilter.active).toBe(false)
    modes.setLibSort('name')
  })

  it('пустая лента при любом порядке пуста', () => {
    setup()
    store.photos = []
    modes.setLibSort('shot')
    expect(store.visible).toEqual([])
    modes.setLibSort('name')
  })
})
