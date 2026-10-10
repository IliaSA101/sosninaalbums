import { describe, expect, it, vi } from 'vitest'

// Отложенные ответы Go: тест отпускает их по одному.
const calls: string[] = []
const pending: Array<{ resolve: (v: unknown) => void; reject: (e: unknown) => void }> = []

vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', () => ({
  DeleteSpread: (id: string) =>
    new Promise((resolve, reject) => {
      calls.push(id)
      pending.push({ resolve, reject })
    }),
}))

// Окружение vitest — node: notify() ждёт window.setTimeout.
vi.stubGlobal('window', globalThis)

const { store } = await import('./store.svelte')

const ids = ['a', 'b', 'c', 'd']
const viewWithout = (gone: string[], rev: number) => ({
  rev,
  folders: [],
  spreads: ids.filter((i) => !gone.includes(i)).map((id) => ({ id })),
})
const tick = () => new Promise((r) => setTimeout(r, 0))

describe('store.deleteSpread: очередь', () => {
  it('три быстрых щелчка — три удаления подряд, id берётся после ответа предыдущего', async () => {
    calls.length = 0
    pending.length = 0
    store.apply(viewWithout([], 1) as never)
    store.goId('a')
    const all = [store.deleteSpread(), store.deleteSpread(), store.deleteSpread()]
    await tick()
    // До ответа Go ушёл только первый.
    expect(calls).toEqual(['a'])
    pending[0].resolve(viewWithout(['a'], 2))
    await tick()
    expect(calls).toEqual(['a', 'b'])
    pending[1].resolve(viewWithout(['a', 'b'], 3))
    await tick()
    expect(calls).toEqual(['a', 'b', 'c'])
    pending[2].resolve(viewWithout(['a', 'b', 'c'], 4))
    await Promise.all(all)
    expect(store.view.spreads.map((s) => s.id)).toEqual(['d'])
  })

  it('ошибка одного удаления не рушит очередь', async () => {
    calls.length = 0
    pending.length = 0
    store.apply(viewWithout([], 1) as never)
    store.goId('a')
    const all = [store.deleteSpread(), store.deleteSpread()]
    await tick()
    pending[0].reject(new Error('сбой'))
    await tick()
    // Первое не прошло, второе берёт тот же текущий разворот и выполняется.
    expect(calls).toEqual(['a', 'a'])
    pending[1].resolve(viewWithout(['a'], 2))
    await Promise.all(all)
    expect(store.view.spreads.map((s) => s.id)).toEqual(['b', 'c', 'd'])
    expect(store.toast?.kind).toBe('info')
  })
})
