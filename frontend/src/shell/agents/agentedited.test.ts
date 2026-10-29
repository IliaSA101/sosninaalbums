import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

// Тост правки агента (149-30): «Клиент · что сделано», «Вернуть» только при той же ревизии, переход к go,
// склейка тостов одного клиента за 5 с. Go подменён: Undo отвечает видом на ревизию меньше.
const ev = vi.hoisted(() => ({ on: new Map<string, (d: unknown) => void>() }))
vi.mock('../../../wailsjs/runtime/runtime', () => ({
  EventsOn: (name: string, cb: (d: unknown) => void) => {
    ev.on.set(name, cb)
    return () => ev.on.delete(name)
  },
}))
const api = vi.hoisted(() => ({ Undo: vi.fn(), AgentsInfo: vi.fn() }))
vi.mock('../../../wailsjs/go/app/App', () => api)
vi.stubGlobal('window', globalThis)

const { agentEdited, resetEdited, GLUE_MS } = await import('./agentedited')
const { agents } = await import('./agents.svelte')
const { store } = await import('../../base/core/store.svelte')

const view = (rev: number, ids = ['s1', 's2', 's3']) => ({ rev, folders: [], spreads: ids.map((id) => ({ id, photos: [] as string[] })) })
const edited = (o: Partial<{ client: string; text: string; rev: number; go: string }> = {}) => ({
  client: 'Claude Code',
  text: 'Собрано разворотов: 24',
  rev: 5,
  go: '',
  ...o,
})
const shownRev = (rev: number) => {
  store.apply(view(rev) as never)
}

beforeEach(() => {
  vi.useFakeTimers()
  vi.setSystemTime(new Date(2026, 9, 29, 12, 0, 0))
  api.Undo.mockReset()
  api.Undo.mockResolvedValue(view(4))
  api.AgentsInfo.mockReset()
  api.AgentsInfo.mockResolvedValue({})
  resetEdited()
  store.toast = null
  shownRev(5)
  store.goId('s1')
})
afterEach(() => {
  vi.useRealTimers()
  store.toast = null
})

describe('тост правки агента', () => {
  it('«Клиент · текст Go» с кнопкой «Вернуть»; тост привязан к ревизии правки', () => {
    agentEdited(edited())
    expect(store.toast?.text).toBe('Claude Code · Собрано разворотов: 24')
    expect(store.toast?.action?.label).toBe('Вернуть')
    expect(store.toast?.rev).toBe(5)
    expect(store.toast?.kind).toBe('info')
  })

  it('«Вернуть» при той же ревизии → Undo', async () => {
    agentEdited(edited())
    await store.toast?.action?.run()
    expect(api.Undo).toHaveBeenCalledTimes(1)
    expect(store.view.rev).toBe(4)
  })

  it('«Вернуть» после чужой правки: Undo не зовём, тост подсказывает «Отменить»', async () => {
    agentEdited(edited())
    const act = store.toast?.action
    store.view = { ...store.view, rev: 6 }
    await act?.run()
    expect(api.Undo).not.toHaveBeenCalled()
    expect(store.toast?.text).toMatch(/Уже были другие изменения — используйте «Отменить»/)
    expect(store.toast?.action).toBeUndefined()
  })

  it('любая другая смена ревизии гасит тост сама', () => {
    agentEdited(edited())
    shownRev(6)
    expect(store.toast).toBeNull()
  })

  it('вид той же ревизии (событие view догоняет agents:edited) тост не гасит', () => {
    shownRev(4)
    agentEdited(edited({ rev: 5 }))
    shownRev(5)
    expect(store.toast?.text).toBe('Claude Code · Собрано разворотов: 24')
  })

  it('ревизия уже дальше правки агента — тост без «Вернуть»: откатило бы не её', () => {
    shownRev(7)
    agentEdited(edited({ rev: 5 }))
    expect(store.toast?.text).toBe('Claude Code · Собрано разворотов: 24')
    expect(store.toast?.action).toBeUndefined()
  })

  it('«Вернуть» ждёт очередь удаления', async () => {
    let free: () => void = () => {}
    const gate = new Promise<void>((r) => (free = r))
    const q = vi.spyOn(store, 'delQueue', 'get').mockReturnValue(gate)
    agentEdited(edited())
    const run = store.toast?.action?.run()
    await Promise.resolve()
    expect(api.Undo).not.toHaveBeenCalled()
    free()
    await run
    expect(api.Undo).toHaveBeenCalledTimes(1)
    q.mockRestore()
  })
})

describe('переход к go', () => {
  it('go — разворот альбома: холст переходит на него', () => {
    agentEdited(edited({ go: 's3' }))
    expect(store.currentId).toBe('s3')
  })

  it('go пуст — остаёмся, где были', () => {
    store.goId('s2')
    agentEdited(edited({ go: '' }))
    expect(store.currentId).toBe('s2')
  })

  it('разворот ещё не дошёл событием view: переход, как только он появился', () => {
    agentEdited(edited({ go: 's4' }))
    expect(store.currentId).toBe('s1')
    store.apply(view(5, ['s1', 's2', 's3', 's4']) as never)
    vi.advanceTimersByTime(200)
    expect(store.currentId).toBe('s4')
  })

  it('разворота так и нет — после ожидания молча остаёмся', () => {
    agentEdited(edited({ go: 'нет-такого' }))
    vi.advanceTimersByTime(5000)
    expect(store.currentId).toBe('s1')
  })
})

// Настоящий порядок Go: view с новой ревизией приходит раньше agents:edited и гасит прежний тост агента.
describe('склейка тостов одного клиента за 5 с', () => {
  it('второй тост того же клиента в окне 5 с продолжает первый (view новой ревизии уже погасил прежний тост)', () => {
    agentEdited(edited())
    vi.advanceTimersByTime(GLUE_MS - 1)
    store.apply(view(6) as never)
    expect(store.toast).toBeNull()
    agentEdited(edited({ text: 'Раскладки изменены: 3', rev: 6 }))
    expect(store.toast?.text).toBe('Claude Code · Собрано разворотов: 24. Раскладки изменены: 3')
    expect(store.toast?.rev).toBe(6)
  })

  it('прежний тост ещё на экране (view не успел) — тоже склеивается', () => {
    agentEdited(edited())
    agentEdited(edited({ text: 'Раскладки изменены: 3', rev: 6 }))
    expect(store.toast?.text).toBe('Claude Code · Собрано разворотов: 24. Раскладки изменены: 3')
  })

  it('«Вернуть» склеенного тоста — один шаг Undo (последняя правка)', async () => {
    agentEdited(edited())
    store.apply(view(6) as never)
    agentEdited(edited({ text: 'Раскладки изменены: 3', rev: 6 }))
    expect(store.toast?.text).toContain('. Раскладки')
    await store.toast?.action?.run()
    expect(api.Undo).toHaveBeenCalledTimes(1)
  })

  it('через 5 с — новый тост, без склейки', () => {
    agentEdited(edited())
    vi.advanceTimersByTime(GLUE_MS)
    store.apply(view(6) as never)
    agentEdited(edited({ text: 'Раскладки изменены: 3', rev: 6 }))
    expect(store.toast?.text).toBe('Claude Code · Раскладки изменены: 3')
  })

  it('другой клиент — не склеивается', () => {
    agentEdited(edited())
    store.apply(view(6) as never)
    agentEdited(edited({ client: 'OpenCode', text: 'Раскладки изменены: 3', rev: 6 }))
    expect(store.toast?.text).toBe('OpenCode · Раскладки изменены: 3')
  })

  it('между ними был другой тост — не склеивается', () => {
    agentEdited(edited())
    store.apply(view(6) as never)
    store.notify('Автосохранено')
    agentEdited(edited({ text: 'Раскладки изменены: 3', rev: 6 }))
    expect(store.toast?.text).toBe('Claude Code · Раскладки изменены: 3')
  })

  it('в склейке не больше трёх последних правок', () => {
    for (let i = 1; i <= 5; i++) {
      store.apply(view(4 + i) as never)
      agentEdited(edited({ text: `Правка ${i}`, rev: 4 + i }))
    }
    expect(store.toast?.text).toBe('Claude Code · Правка 3. Правка 4. Правка 5')
  })
})

describe('событие agents:edited от Go', () => {
  it('значок «ИИ» держит подписку: событие даёт тост и переход без открытого окна', () => {
    const off = agents.watch()
    ev.on.get('agents:edited')?.(edited({ go: 's2' }))
    expect(store.toast?.text).toBe('Claude Code · Собрано разворотов: 24')
    expect(store.currentId).toBe('s2')
    off()
    expect(ev.on.has('agents:edited')).toBe(false)
  })
})
