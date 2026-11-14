// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render } from '@testing-library/svelte'

// Объём выгрузки считает Go (App.ExportSize): окно шлёт выбор и показывает ответ; запоздавший ответ новый не перетирает.
const api = vi.hoisted(() => ({
  PrintPlans: vi.fn(async () => []),
  ExportSize: vi.fn(),
  MissingNames: vi.fn(async () => ({})),
}))
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', () => api)

const { default: ExportDialog } = await import('./ExportDialog.svelte')
const { store } = await import('../../base/core/store.svelte')
const { jobs } = await import('../../base/core/jobs.svelte')

type Out = { summary: string; per: { quality: number; text: string }[] }
const out = (tag: string): Out => ({
  summary: `${tag} на разворот`,
  per: [100, 95, 80].map((quality) => ({ quality, text: `${tag}-${quality}` })),
})
const tick = () => new Promise((r) => setTimeout(r, 0))
const open = async () => {
  jobs.exportOpen = true
  await tick()
  await tick()
}
const sizeText = () => document.querySelector('p.hint.size')?.textContent
const chips = () => [...document.querySelectorAll('[aria-label="Качество"] small')].map((e) => e.textContent)
const lastCall = () => api.ExportSize.mock.calls.at(-1)?.[0]

beforeEach(() => {
  localStorage.clear()
  api.ExportSize.mockReset()
  api.ExportSize.mockImplementation(async () => out('A'))
  store.view = {
    rev: 1,
    folders: [],
    spreads: [{ id: 's1' }, { id: 's2' }],
    settings: { pageW: 300, pageH: 300, dpi: 300 },
    cover: null,
  } as never
  jobs.exportOpen = false
  jobs.exporting = null
})
afterEach(() => {
  cleanup()
  jobs.exportOpen = false
})

describe('объём выгрузки из Go', () => {
  it('итог и подписи качества — из ответа; в запросе выбор окна', async () => {
    render(ExportDialog)
    await open()
    expect(sizeText()).toBe('A на разворот.')
    expect(chips()).toEqual(['A-100', 'A-95', 'A-80'])
    expect(lastCall()).toEqual({
      scope: 'all',
      dpi: 300,
      quality: 95,
      pages: false,
      units: [],
      badRange: false,
      cover: false,
      only: false,
      print: false,
    })
  })
  it('смена качества шлёт новый запрос', async () => {
    render(ExportDialog)
    await open()
    const n = api.ExportSize.mock.calls.length
    await fireEvent.click(document.querySelectorAll('[aria-label="Качество"] button')[2])
    await tick()
    expect(api.ExportSize.mock.calls.length).toBe(n + 1)
    expect(lastCall().quality).toBe(80)
  })
  it('диапазон: цели уходят страницами 0/1/2, неверный ввод — badRange', async () => {
    render(ExportDialog)
    await open()
    await fireEvent.click(document.querySelectorAll('input[name="ex-scope"]')[2])
    const field = document.querySelector('input.ranges') as HTMLInputElement
    await fireEvent.input(field, { target: { value: '1, 2п' } })
    await tick()
    expect(lastCall()).toMatchObject({ scope: 'range', units: [0, 2], badRange: false })
    await fireEvent.input(field, { target: { value: 'ы' } })
    await tick()
    expect(lastCall()).toMatchObject({ scope: 'range', units: [], badRange: true })
  })
  it('запоздавший ответ не перетирает новый', async () => {
    const waits: ((o: Out) => void)[] = []
    api.ExportSize.mockImplementation(() => new Promise<Out>((r) => waits.push(r)))
    render(ExportDialog)
    await open()
    await fireEvent.click(document.querySelectorAll('[aria-label="Качество"] button')[0])
    await tick()
    expect(waits.length).toBeGreaterThanOrEqual(2)
    waits.at(-1)?.(out('NEW'))
    await tick()
    waits[0](out('OLD'))
    await tick()
    expect(sizeText()).toBe('NEW на разворот.')
    expect(chips()[0]).toBe('NEW-100')
  })
  it('пока ответа нет — прежний текст остаётся; сбой Go его не стирает', async () => {
    render(ExportDialog)
    await open()
    api.ExportSize.mockImplementation(() => new Promise<Out>(() => {}))
    await fireEvent.click(document.querySelectorAll('[aria-label="Качество"] button')[2])
    await tick()
    expect(sizeText()).toBe('A на разворот.')
    api.ExportSize.mockImplementation(async () => {
      throw new Error('нет связи')
    })
    await fireEvent.click(document.querySelectorAll('[aria-label="Качество"] button')[1])
    await tick()
    expect(sizeText()).toBe('A на разворот.')
  })
  it('правка альбома при открытом окне перезапрашивает объём', async () => {
    render(ExportDialog)
    await open()
    const n = api.ExportSize.mock.calls.length
    store.view = { ...store.view, rev: 2 }
    await tick()
    expect(api.ExportSize.mock.calls.length).toBe(n + 1)
  })
  it('ответ после закрытия окна не применяется; при новом открытии старых цифр нет', async () => {
    const waits: ((o: Out) => void)[] = []
    api.ExportSize.mockImplementation(() => new Promise<Out>((r) => waits.push(r)))
    render(ExportDialog)
    await open()
    waits[0](out('OLD'))
    await tick()
    expect(sizeText()).toBe('OLD на разворот.')
    // запрос в полёте, окно закрыли: ответ отсекается
    store.view = { ...store.view, rev: 2 }
    await tick()
    jobs.exportOpen = false
    await tick()
    waits.at(-1)?.(out('LATE'))
    await tick()
    // повторное открытие: пока нового ответа нет, ни OLD, ни LATE не видны
    jobs.exportOpen = true
    await tick()
    await tick()
    expect(sizeText() ?? '').not.toMatch(/OLD|LATE/)
    expect(chips().join('')).not.toMatch(/OLD|LATE/)
    waits.at(-1)?.(out('NEW'))
    await tick()
    expect(sizeText()).toBe('NEW на разворот.')
  })
})
