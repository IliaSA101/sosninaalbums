import { describe, expect, it, vi } from 'vitest'

// Операции над листами (spreadops.svelte.ts): Go подменён, ответ удаления отдаёт тест.
const calls: string[] = []
let rev = 1
let ids = ['a', 'b', 'c', 'd']

vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', () => ({
  DeleteSpread: async (id: string) => {
    calls.push(id)
    ids = ids.filter((x) => x !== id)
    return { rev: ++rev, folders: [], spreads: ids.map((i) => ({ id: i })) }
  },
}))

// Окружение vitest — node: notify() ждёт window.setTimeout.
vi.stubGlobal('window', globalThis)

const { store } = await import('./store.svelte')

describe('SpreadOps.deleteSpread: два быстрых удаления', () => {
  it('убирают два разворота: второе берёт текущий уже после первого', async () => {
    store.apply({ rev, folders: [], spreads: ids.map((id) => ({ id })) } as never)
    store.goId('a')
    await Promise.all([store.deleteSpread(), store.deleteSpread()])
    expect(calls).toEqual(['a', 'b'])
    expect(store.view.spreads.map((s) => s.id)).toEqual(['c', 'd'])
  })
})
