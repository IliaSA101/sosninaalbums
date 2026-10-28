import { beforeEach, describe, expect, it, vi } from 'vitest'

const api = vi.hoisted(() => ({ PrintProfiles: vi.fn(), CoverPresets: vi.fn() }))
const events = vi.hoisted(() => ({ handlers: {} as Record<string, () => void> }))
vi.mock('../../../wailsjs/runtime/runtime', () => ({
  EventsOn: (name: string, f: () => void) => {
    events.handlers[name] = f
    return () => delete events.handlers[name]
  },
}))
vi.mock('../../../wailsjs/go/app/App', () => api)

const { printList } = await import('./printlist.svelte')
const { coverAct } = await import('../../panels/cover/coveract.svelte')

const info = (id: string, name: string, source: string) => ({ id, name, source, version: 1, hasExport: true, formats: [] })
const flush = async () => {
  for (let i = 0; i < 8; i++) await Promise.resolve()
}

beforeEach(() => {
  api.PrintProfiles.mockReset().mockResolvedValue([info('fabrika', 'Фабрика', 'builtin'), info('own-1', 'Альфа', 'own')])
  api.CoverPresets.mockReset().mockResolvedValue([])
  printList.infos = []
  printList.rev = 0
  printList.saves = 0
  coverAct.presets = []
})

describe('printList', () => {
  it('watch читает список и подписывается на print:changed; снятие отписывает', async () => {
    const off = printList.watch()
    await flush()
    expect(printList.infos.map((i) => i.id)).toEqual(['fabrika', 'own-1'])
    expect(events.handlers['print:changed']).toBeTypeOf('function')
    off()
    expect(events.handlers['print:changed']).toBeUndefined()
  })

  it('print:changed: rev растёт, список и пресеты обложки читаются заново', async () => {
    const off = printList.watch()
    await flush()
    api.PrintProfiles.mockResolvedValue([info('own-2', 'Бета', 'own')])
    api.CoverPresets.mockClear()
    events.handlers['print:changed']()
    await flush()
    expect(printList.rev).toBe(1)
    expect(printList.infos.map((i) => i.id)).toEqual(['own-2'])
    expect(api.CoverPresets).toHaveBeenCalledTimes(1)
    off()
  })

  it('label и byName: своя — с пометкой', async () => {
    await printList.load()
    expect(printList.label('Альфа')).toBe('Альфа (своя)')
    expect(printList.label('Фабрика')).toBe('Фабрика')
    expect(printList.byName('Альфа')?.id).toBe('own-1')
    expect(printList.byName('Нет')).toBeUndefined()
  })

  it('ошибка чтения не ломает: список остаётся прежним', async () => {
    await printList.load()
    api.PrintProfiles.mockRejectedValue('сбой')
    await printList.load()
    expect(printList.infos).toHaveLength(2)
  })

  it('устаревший ответ списка отбрасывается', async () => {
    let first!: (v: unknown) => void
    api.PrintProfiles.mockReturnValueOnce(new Promise((r) => (first = r)))
    const a = printList.load()
    api.PrintProfiles.mockResolvedValueOnce([info('own-9', 'Новая', 'own')])
    await printList.load()
    first([info('old', 'Старая', 'own')])
    await a
    expect(printList.infos.map((i) => i.id)).toEqual(['own-9'])
  })
})

describe('coverAct.loadPresets', () => {
  it('force читает заново; запрос во время запроса — повтор после него', async () => {
    api.CoverPresets.mockResolvedValue([{ id: 'a' }])
    await coverAct.loadPresets()
    await coverAct.loadPresets()
    expect(api.CoverPresets).toHaveBeenCalledTimes(1)
    let first!: (v: unknown) => void
    api.CoverPresets.mockReturnValueOnce(new Promise((r) => (first = r)))
    const p = coverAct.reloadPresets()
    void coverAct.reloadPresets()
    api.CoverPresets.mockResolvedValue([{ id: 'b' }])
    first([{ id: 'a2' }])
    await p
    expect(api.CoverPresets).toHaveBeenCalledTimes(3)
    expect(coverAct.presets.map((x) => x.id)).toEqual(['b'])
  })
})
