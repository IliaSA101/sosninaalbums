import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushSync } from 'svelte'
import type { Photo } from '../../base/core/types'

vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', () => ({ CheckFolders: vi.fn(async () => {}) }))
vi.stubGlobal('window', globalThis)

const { store } = await import('../../base/core/store.svelte')
const { lost } = await import('./lost.svelte')
const { autocheck } = await import('../folders/autocheck')

const ph = (id: string) => ({ id, name: `${id}.jpg`, w: 3, h: 2 }) as unknown as Photo

describe('lost', () => {
  let soon: ReturnType<typeof vi.spyOn>
  beforeEach(() => {
    // Ошибку засчитываем только плиткам текущей ленты: r1–r5 нужны тестам повтора.
    store.photos = ['a', 'b', 'r1', 'r2', 'r3', 'r4', 'r5'].map(ph)
    flushSync()
    lost.clear()
    soon = vi.spyOn(autocheck, 'soon').mockImplementation(() => {})
  })
  afterEach(() => soon.mockRestore())

  it('ошибка загрузки — ID в lost и запрос проверки', () => {
    expect(lost.has('a')).toBe(false)
    lost.mark('a')
    expect(lost.has('a')).toBe(true)
    expect(lost.has('b')).toBe(false)
    expect(soon).toHaveBeenCalledTimes(1)
  })

  it('ошибка плитки не из ленты не запрашивает сверку', () => {
    lost.mark('zzz')
    expect(lost.has('zzz')).toBe(false)
    expect(soon).not.toHaveBeenCalled()
  })

  it('новый список ленты очищает lost', () => {
    lost.mark('a')
    lost.mark('b')
    store.photos = [ph('b')]
    flushSync()
    expect(lost.has('a')).toBe(false)
    expect(lost.has('b')).toBe(false)
  })

  it('после сверки плитка, которой нет в store.missing, получает один повтор: lost сброшен, метка адреса растёт', () => {
    store.missing = []
    lost.mark('r1')
    expect(lost.retryOf('r1')).toBe(0)
    lost.retry()
    expect(lost.has('r1')).toBe(false)
    expect(lost.retryOf('r1')).toBe(1)
    expect(lost.retryOf('r2')).toBe(0)
  })

  it('повтор снова с ошибкой: заглушка остаётся, новой сверки не просим, второго повтора нет', () => {
    lost.mark('r2')
    lost.retry()
    soon.mockClear()
    lost.mark('r2')
    expect(lost.has('r2')).toBe(true)
    expect(soon).not.toHaveBeenCalled()
    lost.retry()
    expect(lost.has('r2')).toBe(true)
    expect(lost.retryOf('r2')).toBe(1)
  })

  it('пропавшее по версии Go (store.missing) не перезагружается', () => {
    store.missing = ['r3']
    lost.mark('r3')
    lost.retry()
    expect(lost.has('r3')).toBe(true)
    expect(lost.retryOf('r3')).toBe(0)
    store.missing = []
  })

  it('новый список ленты снова разрешает повтор; метка адреса сохраняется', () => {
    lost.mark('r4')
    lost.retry()
    store.photos = [ph('r4'), ph('b')]
    flushSync()
    soon.mockClear()
    lost.mark('r4')
    expect(soon).toHaveBeenCalledTimes(1)
    lost.retry()
    expect(lost.retryOf('r4')).toBe(2)
  })

  it('конец сверки автопроверки зовёт retry (подписка при загрузке модуля)', async () => {
    lost.mark('r5')
    vi.useFakeTimers()
    try {
      soon.mockRestore()
      autocheck.soon()
      vi.advanceTimersByTime(300)
    } finally {
      vi.useRealTimers()
    }
    await new Promise((r) => setTimeout(r, 0))
    expect(lost.has('r5')).toBe(false)
    expect(lost.retryOf('r5')).toBeGreaterThan(0)
  })

  it('причина «не читается» исчезла (117): заглушка снята, адрес миниатюры новый, сверку снова можно просить', () => {
    store.unread = { r1: 'файл оборван' }
    flushSync()
    lost.mark('r1')
    lost.retry() // повтор уже потрачен
    lost.mark('r1')
    expect(lost.has('r1')).toBe(true)
    const before = lost.retryOf('r1')
    store.unread = {}
    flushSync()
    expect(lost.has('r1')).toBe(false)
    expect(lost.retryOf('r1')).toBe(before + 1)
    soon.mockClear()
    lost.mark('r1')
    expect(soon).toHaveBeenCalledTimes(1)
  })

  it('причина осталась или появилась — адрес не меняется', () => {
    const [t2, t3] = [lost.retryOf('r2'), lost.retryOf('r3')]
    store.unread = { r2: 'пустой файл' }
    flushSync()
    store.unread = { r2: 'пустой файл', r3: 'оборван' }
    flushSync()
    expect(lost.retryOf('r2')).toBe(t2)
    expect(lost.retryOf('r3')).toBe(t3)
    store.unread = {}
    flushSync()
  })

  it('повторная ошибка той же плитки не плодит записей', () => {
    lost.mark('a')
    lost.mark('a')
    expect(lost.has('a')).toBe(true)
    lost.clear()
    expect(lost.has('a')).toBe(false)
  })
})
