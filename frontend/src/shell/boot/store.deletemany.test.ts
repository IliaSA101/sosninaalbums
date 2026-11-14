import { describe, expect, it, vi } from 'vitest'

// Go подменён: DeleteSpreads убирает известные id, Undo возвращает всё.
const calls: string[][] = []
const ids = ['a', 'b', 'c', 'd', 'e']
const viewWithout = (gone: string[], rev: number) => ({
  rev,
  folders: [],
  spreads: ids.filter((i) => !gone.includes(i)).map((id) => ({ id })),
})

vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', () => ({
  DeleteSpreads: async (list: string[]) => {
    calls.push(list)
    return viewWithout(list, 2)
  },
  Undo: async () => viewWithout([], 3),
}))
vi.mock('../../editor/text/fonts', () => ({ loadFonts: async () => {} }))
vi.stubGlobal('window', globalThis)

const { store } = await import('../../base/core/store.svelte')
const { wire } = await import('./wire')
const { spreadsel } = await import('../../panels/timeline/spreadsel.svelte')

wire()

describe('store.deleteSpreads', () => {
  it('один запрос в порядке альбома, выделение снято, тост «Вернуть» возвращает и переходит к первому', async () => {
    calls.length = 0
    store.apply(viewWithout([], 1) as never)
    store.goId('e')
    spreadsel.toggle('d')
    spreadsel.toggle('b')
    await store.deleteSpreads(['d', 'b', 'zz'])
    // Неизвестный id не уходит в Go; порядок — как в альбоме.
    expect(calls).toEqual([['b', 'd']])
    expect(spreadsel.size).toBe(0)
    expect(store.toast?.text).toBe('Удалено разворотов: 2')
    expect(store.toast?.action?.label).toBe('Вернуть')
    await store.toast?.action?.run()
    expect(store.view.spreads).toHaveLength(5)
    expect(store.currentId).toBe('b')
  })

  it('холостая правка (rev тот же) — тоста нет', async () => {
    calls.length = 0
    store.toast = null
    store.apply(viewWithout([], 2) as never) // Go вернёт rev 2: ничего не изменилось
    await store.deleteSpreads(['a'])
    expect(calls).toEqual([['a']])
    expect(store.toast).toBeNull()
  })

  it('ни одного известного id — Go не вызывается', async () => {
    calls.length = 0
    store.apply(viewWithout([], 1) as never)
    await store.deleteSpreads(['zz'])
    expect(calls).toEqual([])
  })

  it('apply выкидывает из выделения исчезнувшие развороты', () => {
    store.apply(viewWithout([], 1) as never)
    spreadsel.clear()
    spreadsel.toggle('a')
    spreadsel.toggle('c')
    store.apply(viewWithout(['c'], 2) as never)
    expect(spreadsel.ids).toEqual(['a'])
  })
})
