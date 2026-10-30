// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/svelte'

// Окно «Готово» типографии (123): кнопка сайта зовёт App.OpenPrintSite(kind) — адрес Go берёт из профиля.
const api = vi.hoisted(() => ({ OpenPrintSite: vi.fn(async () => undefined) }))
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', async (orig) => {
  const all = await orig<Record<string, unknown>>()
  return { ...Object.fromEntries(Object.keys(all).map((k) => [k, vi.fn(async () => undefined)])), ...api }
})

const { default: PrintDone } = await import('./PrintDone.svelte')
const { jobs } = await import('../../base/core/jobs.svelte')

const plan = (o: Record<string, unknown> = {}) =>
  ({ kind: 'fabrika', done: 'Готово для Фабрики', site: 'Заказ на сайте', siteButton: 'Открыть Фабрику Фотокниги', ...o }) as never

beforeEach(() => {
  api.OpenPrintSite.mockClear()
  jobs.exportMemo = ['Загрузите папку']
})
afterEach(() => {
  cleanup()
  jobs.exportMemo = null
  jobs.exportPrint = null
})

describe('кнопка сайта в окне «Готово»', () => {
  it('клик — OpenPrintSite с id типографии плана, окно остаётся открытым', async () => {
    jobs.exportPrint = plan()
    render(PrintDone)
    await fireEvent.click(screen.getByText('Открыть Фабрику Фотокниги'))
    expect(api.OpenPrintSite).toHaveBeenCalledWith('fabrika')
    expect(jobs.exportMemo).not.toBeNull()
  })
  it('другая типография — её id', async () => {
    jobs.exportPrint = plan({ kind: 'fsfera', siteButton: 'Открыть Фотосферу' })
    render(PrintDone)
    await fireEvent.click(screen.getByText('Открыть Фотосферу'))
    expect(api.OpenPrintSite).toHaveBeenCalledWith('fsfera')
  })
  it('у типографии без сайта (siteButton пуст) кнопки нет', () => {
    jobs.exportPrint = plan({ siteButton: '', site: '' })
    render(PrintDone)
    expect(screen.queryByText(/Открыть Фабрику/)).toBeNull()
    expect(screen.getByText('Открыть папку')).toBeTruthy()
  })
})
