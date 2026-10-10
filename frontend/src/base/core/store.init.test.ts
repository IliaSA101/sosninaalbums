import { beforeEach, describe, expect, it, vi } from 'vitest'

// Обработчики событий Go, которые store.init() кладёт в EventsOn.
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
}))

const { store } = await import('./store.svelte')

const photos = (n: number) => Array.from({ length: n }, (_, i) => ({ id: 'p' + i }))
const idle = { active: false, walking: false, found: 0, read: 0 }
const view = { spreads: [], folders: [] }

beforeEach(() => {
  for (const k of Object.keys(handlers)) delete handlers[k]
})

describe('store.init: события раньше ответа GetState', () => {
  it('library и scan не затираются устаревшим снимком', async () => {
    const done = store.init()
    handlers.library(photos(36))
    handlers.scan({ ...idle })
    resolveState({ view, photos: [], scan: { ...idle, active: true }, missing: [] })
    await done
    expect(store.photos.length).toBe(36)
    expect(store.scan.active).toBe(false)
    expect(store.loaded).toBe(true)
  })

  it('missing не затирается устаревшим снимком', async () => {
    const done = store.init()
    handlers.missing(['p1', 'p2'])
    resolveState({ view, photos: [], missing: [] })
    await done
    expect(store.missing).toEqual(['p1', 'p2'])
  })

  it('без событий снимок применяется', async () => {
    const done = store.init()
    resolveState({ view, photos: photos(3), scan: { ...idle, active: true }, missing: ['p0'] })
    await done
    expect(store.photos.length).toBe(3)
    expect(store.scan.active).toBe(true)
    expect(store.missing).toEqual(['p0'])
  })
})
