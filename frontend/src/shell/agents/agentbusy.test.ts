// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushSync } from 'svelte'

// «Человек занят» (149-16, 149-32): что считается занятостью и как о ней знает Go — AgentsBusy(true) сразу и раз в 10 с,
// AgentsBusy(false) в конце. Окна «ИИ-агенты», F1 и просмотра занятостью не считаются.
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
const api = vi.hoisted(() => ({ AgentsBusy: vi.fn(), AgentsInfo: vi.fn(), RelinkFolder: vi.fn() }))
vi.mock('../../../wailsjs/go/app/App', () => api)

const { BUSY_PING_MS, BusyPing, userBusy } = await import('./agentbusy')
const { drag } = await import('../../base/drag/drag.svelte')
const { canvas } = await import('../../editor/canvas/canvas.svelte')
const { edit } = await import('../../editor/text/edit.svelte')
const { store } = await import('../../base/core/store.svelte')
const { jobs } = await import('../../base/core/jobs.svelte')
const { modes } = await import('../../base/prefs/view.svelte')
const { settingsDraft } = await import('../../panels/details/settingsbusy.svelte')
const { relinkFolder } = await import('../../panels/library/relink')
const { modalStack } = await import('../../base/modal/modalstack.svelte')
const { agents } = await import('./agents.svelte')

const calls = () => api.AgentsBusy.mock.calls.map((c) => c[0])

beforeEach(() => {
  vi.useFakeTimers()
  api.AgentsBusy.mockReset()
  api.AgentsBusy.mockResolvedValue(undefined)
  api.AgentsInfo.mockReset()
  api.AgentsInfo.mockResolvedValue({})
})
afterEach(() => {
  drag.active = false
  canvas.busy = false
  edit.target = null
  store.busy = false
  store.autoDraft = null
  jobs.exporting = null
  jobs.collecting = null
  jobs.opening = null
  modes.details = false
  modes.sections.settings = false
  settingsDraft.dirty = false
  settingsDraft.focus = false
  agents.open = false
  vi.useRealTimers()
})

describe('userBusy: что считается занятостью', () => {
  it('ничего не происходит — не занят', () => {
    expect(userBusy()).toBe(false)
  })

  const cases: [string, () => void][] = [
    ['перенос', () => (drag.active = true)],
    ['жест на холсте (кадр, места, текст)', () => (canvas.busy = true)],
    ['набор текста', () => (edit.target = { kind: 'text', spread: 's1', id: 't1' })],
    ['идущая задача Go (store.busy)', () => (store.busy = true)],
    ['выгрузка', () => (jobs.exporting = { done: 1, total: 5 })],
    ['сбор в папку', () => (jobs.collecting = { doneFiles: 1, totalFiles: 5, doneBytes: 1, totalBytes: 5 })],
    ['открытие альбома', () => (jobs.opening = { done: 1, total: 5 })],
    ['окно «Автосборка»', () => (store.autoDraft = { from: 'all', ids: [] } as never)],
    ['«Настройки альбома»: непринятые правки формы', () => (settingsDraft.dirty = true)],
    ['«Настройки альбома»: фокус в форме', () => (settingsDraft.focus = true)],
  ]
  it.each(cases)('занят: %s', (_n, on) => {
    on()
    expect(userBusy()).toBe(true)
  })

  // Флаги раскрытия лежат в localStorage: раскрытый раз раздел не должен давать агенту вечный отказ.
  it('«Настройки альбома» раскрыты при открытой панели, правок и фокуса нет — не занят', () => {
    modes.details = true
    modes.sections.settings = true
    expect(userBusy()).toBe(false)
  })

  it('перепривязка папки: занят, пока идёт диалог и поиск, потом нет', async () => {
    let finish: (r: unknown) => void = () => {}
    api.RelinkFolder.mockReturnValue(new Promise((r) => (finish = r)))
    const run = relinkFolder()
    expect(userBusy()).toBe(true)
    finish({ found: 0, view: null })
    await run
    expect(userBusy()).toBe(false)
  })

  it('перепривязка упала ошибкой — флаг всё равно снят', async () => {
    api.RelinkFolder.mockRejectedValue(new Error('нет диска'))
    await relinkFolder()
    expect(userBusy()).toBe(false)
    store.toast = null
  })

  it('«ИИ-агенты» открыты — не занят', () => {
    agents.open = true
    expect(userBusy()).toBe(false)
  })

  it('любое окно в modalstack (F1, просмотр, справочники) — не занят: берём окна по именам, не весь стек', () => {
    const id = modalStack.push()
    expect(modalStack.any()).toBe(true)
    expect(userBusy()).toBe(false)
    modalStack.remove(id)
  })
})

describe('BusyPing: сигнал Go', () => {
  it('занят → true сразу и раз в 10 с; конец → false один раз', () => {
    const p = new BusyPing()
    p.set(true)
    expect(calls()).toEqual([true])
    vi.advanceTimersByTime(BUSY_PING_MS - 1)
    expect(calls()).toEqual([true])
    vi.advanceTimersByTime(1)
    expect(calls()).toEqual([true, true])
    vi.advanceTimersByTime(BUSY_PING_MS * 2)
    expect(calls()).toEqual([true, true, true, true])
    p.set(false)
    expect(calls().at(-1)).toBe(false)
    vi.advanceTimersByTime(BUSY_PING_MS * 3)
    expect(calls().filter((x) => x === false)).toHaveLength(1)
    expect(calls()).toHaveLength(5)
  })

  it('то же значение подряд ничего не шлёт; «не занят» с самого начала — тишина', () => {
    const p = new BusyPing()
    p.set(false)
    p.set(true)
    p.set(true)
    expect(calls()).toEqual([true])
    p.stop()
    p.stop()
    expect(calls()).toEqual([true, false])
  })

  it('Go не ответил (ошибка) — не падаем, повтор идёт', async () => {
    api.AgentsBusy.mockRejectedValue(new Error('нет связи'))
    const p = new BusyPing()
    p.set(true)
    vi.advanceTimersByTime(BUSY_PING_MS)
    await Promise.resolve()
    expect(calls()).toEqual([true, true])
    p.stop()
  })
})

describe('значок «ИИ» (agents.watch) сообщает о занятости', () => {
  let off: () => void = () => {}
  beforeEach(() => {
    off = agents.watch()
  })
  afterEach(() => off())

  it('жест начался → true, повтор раз в 10 с, жест кончился → false', () => {
    flushSync()
    expect(calls()).toEqual([])
    canvas.busy = true
    flushSync()
    expect(calls()).toEqual([true])
    vi.advanceTimersByTime(BUSY_PING_MS)
    expect(calls()).toEqual([true, true])
    canvas.busy = false
    flushSync()
    expect(calls()).toEqual([true, true, false])
    vi.advanceTimersByTime(BUSY_PING_MS * 2)
    expect(calls()).toHaveLength(3)
  })

  it('открыли «Автосборку», потом «ИИ-агенты» — занят остаётся ровно по «Автосборке»', () => {
    store.autoDraft = { from: 'all', ids: [] } as never
    flushSync()
    agents.open = true
    flushSync()
    expect(calls()).toEqual([true])
    store.autoDraft = null
    flushSync()
    expect(calls()).toEqual([true, false])
  })

  it('подписка снята посреди занятости — Go получает false', () => {
    drag.active = true
    flushSync()
    off()
    expect(calls()).toEqual([true, false])
  })

  it('форма «Настройки альбома»: правка → true, принята → false', () => {
    settingsDraft.dirty = true
    flushSync()
    expect(calls()).toEqual([true])
    settingsDraft.dirty = false
    flushSync()
    expect(calls()).toEqual([true, false])
  })
})
