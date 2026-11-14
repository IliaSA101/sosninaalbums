import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

// Событие open:progress и окно «Открываем альбом» (073): ход живёт в jobs, окно только рисует его.
const handlers: Record<string, (...a: unknown[]) => void> = {}
let resolveState: (st: unknown) => void = () => {}

vi.mock('../../../wailsjs/runtime/runtime', () => ({
  EventsOn: (name: string, fn: (...a: unknown[]) => void) => {
    handlers[name] = fn
    return () => {}
  },
}))
vi.mock('../../../wailsjs/go/app/App', () => ({
  GetState: () => new Promise((r) => (resolveState = r)),
  MissingNames: async () => ({}),
}))

vi.stubGlobal('window', globalThis)
const { store } = await import('./store.svelte')
const { jobs, OPEN_SHOW_AFTER } = await import('./jobs.svelte')

const view = { spreads: [], folders: [] }

async function ready() {
  const done = store.init()
  resolveState({ view, photos: [], missing: [] })
  await done
  jobs.endOpening()
}
// Прокрутить таймеры и сказать, появилось ли окно.
async function shownAfter(ms: number) {
  await vi.advanceTimersByTimeAsync(ms)
  return jobs.openShown
}

beforeEach(() => {
  for (const k of Object.keys(handlers)) delete handlers[k]
  vi.useFakeTimers()
})
afterEach(() => {
  jobs.endOpening()
  vi.useRealTimers()
})

describe('open:progress — окно «Открываем альбом» (073)', () => {
  it('первое событие ведёт счётчик, окно появляется после паузы, не сразу', async () => {
    await ready()
    handlers['open:progress'](10, 200)
    expect(jobs.opening).toEqual({ done: 10, total: 200 })
    expect(await shownAfter(OPEN_SHOW_AFTER - 1)).toBe(false)
    expect(await shownAfter(1)).toBe(true)
    handlers['open:progress'](120, 200)
    expect(jobs.opening).toEqual({ done: 120, total: 200 })
    expect(jobs.openShown).toBe(true)
  })
  it('(total, total) до паузы гасит таймер: короткий прогрев окна не показывает', async () => {
    await ready()
    handlers['open:progress'](10, 200)
    handlers['open:progress'](200, 200)
    expect(jobs.opening).toBeNull()
    expect(await shownAfter(OPEN_SHOW_AFTER * 2)).toBe(false)
  })
  it('(total, total) при открытом окне — окно гаснет', async () => {
    await ready()
    handlers['open:progress'](10, 200)
    await shownAfter(OPEN_SHOW_AFTER)
    handlers['open:progress'](200, 200)
    expect(jobs.opening).toBeNull()
    expect(jobs.openShown).toBe(false)
  })
  it('событие с total 0 окно не открывает', async () => {
    await ready()
    handlers['open:progress'](0, 0)
    expect(jobs.opening).toBeNull()
  })
  it('смена альбома (replaceState) гасит окно, даже без (total, total)', async () => {
    await ready()
    handlers['open:progress'](10, 200)
    await shownAfter(OPEN_SHOW_AFTER)
    await store.replaceState(Promise.resolve({ view, photos: [] }))
    expect(jobs.opening).toBeNull()
    expect(jobs.openShown).toBe(false)
  })
  it('отмена диалога «Открыть» и ошибка открытия тоже гасят окно', async () => {
    await ready()
    handlers['open:progress'](10, 200)
    await store.replaceState(Promise.resolve({ cancelled: true }))
    expect(jobs.opening).toBeNull()
    handlers['open:progress'](10, 200)
    await shownAfter(OPEN_SHOW_AFTER)
    await store.replaceState(Promise.reject('нет доступа'))
    expect(jobs.opening).toBeNull()
    expect(jobs.openShown).toBe(false)
  })
  it('новое открытие после прежнего начинается с паузы заново', async () => {
    await ready()
    handlers['open:progress'](10, 200)
    await shownAfter(OPEN_SHOW_AFTER)
    handlers['open:progress'](200, 200)
    handlers['open:progress'](5, 50)
    expect(jobs.openShown).toBe(false)
    expect(await shownAfter(OPEN_SHOW_AFTER)).toBe(true)
  })
  it('старт программы: init после GetState гасит окно', async () => {
    const done = store.init()
    handlers['open:progress'](10, 200)
    await vi.advanceTimersByTimeAsync(OPEN_SHOW_AFTER)
    expect(jobs.openShown).toBe(true)
    resolveState({ view, photos: [], missing: [] })
    await done
    expect(jobs.openShown).toBe(false)
    expect(jobs.opening).toBeNull()
  })
})
