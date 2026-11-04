import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

// agents:show (149): агент просит показать лист — холст переходит, альбом не меняется, тоста нет.
const ev = vi.hoisted(() => ({ on: new Map<string, (d: unknown) => void>() }))
vi.mock('../../../wailsjs/runtime/runtime', () => ({
  EventsOn: (name: string, cb: (d: unknown) => void) => {
    ev.on.set(name, cb)
    return () => ev.on.delete(name)
  },
}))
const api = vi.hoisted(() => ({ AgentsInfo: vi.fn() }))
vi.mock('../../../wailsjs/go/app/App', () => api)
vi.stubGlobal('window', globalThis)

const { agentShow } = await import('./agentedited')
const { agents } = await import('./agents.svelte')
const { store } = await import('../../base/core/store.svelte')

const ids = ['s1', 's2', 's3', 's4', 's5', 's6', 's7', 's8']
const view = (rev: number, more: string[] = []) => ({
  rev,
  folders: [],
  spreads: [...ids, ...more].map((id) => ({ id, photos: [] as string[] })),
})

beforeEach(() => {
  vi.useFakeTimers()
  api.AgentsInfo.mockReset()
  api.AgentsInfo.mockResolvedValue({})
  store.toast = null
  store.apply(view(3) as never)
  store.goId('s1')
})
afterEach(() => {
  vi.useRealTimers()
  store.toast = null
})

describe('agents:show', () => {
  it('переходит к листу, альбом (rev) не меняется, тоста нет', () => {
    agentShow({ spread: 's8' })
    expect(store.currentId).toBe('s8')
    expect(store.view.rev).toBe(3)
    expect(store.toast).toBeNull()
  })

  it('событие Go через значок «ИИ» доходит до холста', () => {
    const off = agents.watch()
    ev.on.get('agents:show')?.({ spread: 's5' })
    expect(store.currentId).toBe('s5')
    off()
    expect(ev.on.has('agents:show')).toBe(false)
  })

  it('обложка (spread: "cover"): холст переходит на неё, текущий разворот помнится; без обложки в виде — остаёмся', () => {
    store.goId('s3')
    agentShow({ spread: 'cover' })
    expect(store.onCover).toBe(false)
    expect(store.currentId).toBe('s3')
    store.apply({ ...view(4), cover: { sheet: { id: 'cover', photos: [] } } } as never)
    agentShow({ spread: 'cover' })
    expect(store.onCover).toBe(true)
    expect(store.currentId).toBe('s3')
    expect(store.view.rev).toBe(4)
    expect(store.toast).toBeNull()
    agentShow({ spread: 's5' })
    expect(store.onCover).toBe(false)
    expect(store.currentId).toBe('s5')
  })

  it('тот же лист, на котором стоим, — остаёмся на нём', () => {
    agentShow({ spread: 's1' })
    expect(store.currentId).toBe('s1')
  })

  it('листа ещё нет в виде: переход, когда view его принесёт', () => {
    agentShow({ spread: 's9' })
    expect(store.currentId).toBe('s1')
    store.apply(view(4, ['s9']) as never)
    vi.advanceTimersByTime(200)
    expect(store.currentId).toBe('s9')
  })

  it('пустой id и несуществующий лист — молча остаёмся', () => {
    agentShow({ spread: '' })
    agentShow({ spread: 'нет-такого' })
    vi.advanceTimersByTime(5000)
    expect(store.currentId).toBe('s1')
    expect(store.toast).toBeNull()
  })
})
