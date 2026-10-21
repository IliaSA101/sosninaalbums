import { beforeEach, describe, expect, it, vi } from 'vitest'

// Перенос разворота (096 сц. 5): одна правка MoveSpread, перенесённый — текущий; dropOn сообщает, прошла ли правка.
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', () => ({ MoveSpread: vi.fn() }))
vi.stubGlobal('window', globalThis)

const Api = await import('../../../wailsjs/go/app/App')
const { store } = await import('../core/store.svelte')
const { dropOn } = await import('./dropact')

const MoveSpread = vi.mocked(Api.MoveSpread)
const sp = (id: string) => ({ id, photos: [], grid: false })
const view = (rev: number, ids: string[]) => ({ rev, marksRev: 0, folders: [], spreads: ids.map(sp), settings: { sheet: false } }) as never

beforeEach(() => {
  vi.restoreAllMocks()
  MoveSpread.mockReset()
  store.apply(view(1, ['a', 'b', 'c', 'd']))
  store.go(0)
})

describe('dropOn: перенос разворота', () => {
  it('перенесённый — текущий', async () => {
    MoveSpread.mockResolvedValueOnce(view(2, ['b', 'c', 'a', 'd']))
    const go = vi.spyOn(store, 'go')
    const ok = await dropOn({ kind: 'spread', id: 'a', index: 0 }, { kind: 'gap', at: 3, to: 2 })
    expect(ok).toBe(true)
    expect(MoveSpread).toHaveBeenCalledOnce()
    expect(MoveSpread).toHaveBeenCalledWith('a', 2)
    expect(go).toHaveBeenCalledWith(2)
    expect(store.current).toBe(2)
    expect(store.currentId).toBe('a')
  })

  it('перенесённый — текущий, даже если до этого текущим был другой', async () => {
    store.go(3) // текущий — d
    MoveSpread.mockResolvedValueOnce(view(2, ['b', 'a', 'c', 'd']))
    expect(await dropOn({ kind: 'spread', id: 'a', index: 0 }, { kind: 'gap', at: 2, to: 1 })).toBe(true)
    expect(store.currentId).toBe('a')
    expect(store.current).toBe(1)
  })

  it('ошибка — без перехода', async () => {
    store.go(3)
    MoveSpread.mockRejectedValueOnce(new Error('нельзя'))
    const go = vi.spyOn(store, 'go')
    const ok = await dropOn({ kind: 'spread', id: 'a', index: 0 }, { kind: 'gap', at: 3, to: 2 })
    expect(ok).toBe(false)
    expect(go).not.toHaveBeenCalled()
    expect(store.currentId).toBe('d')
    store.toast = null
  })

  it('своё место — ни правки, ни перехода', async () => {
    const go = vi.spyOn(store, 'go')
    expect(await dropOn({ kind: 'spread', id: 'b', index: 1 }, { kind: 'gap', at: 1, to: 1 })).toBe(false)
    expect(MoveSpread).not.toHaveBeenCalled()
    expect(go).not.toHaveBeenCalled()
  })
})
