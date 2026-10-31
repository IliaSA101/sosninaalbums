// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/svelte'

// Окно «Готово» типографии (123): кнопка сайта зовёт App.OpenPrintSite(kind) — адрес Go берёт из профиля.
const api = vi.hoisted(() => ({ OpenPrintSite: vi.fn(async () => undefined) }))
const ev = vi.hoisted(() => ({ on: new Map<string, (...a: unknown[]) => void>() }))
vi.mock('../../../wailsjs/runtime/runtime', () => ({
  EventsOn: (n: string, f: (...a: unknown[]) => void) => {
    ev.on.set(n, f)
    return () => {}
  },
}))
vi.mock('../../../wailsjs/go/app/App', async (orig) => {
  const all = await orig<Record<string, unknown>>()
  return { ...Object.fromEntries(Object.keys(all).map((k) => [k, vi.fn(async () => undefined)])), ...api }
})

const { default: PrintDone } = await import('./PrintDone.svelte')
const { jobs } = await import('../../base/core/jobs.svelte')
jobs.listen()
const done = (memo: string[] | null = ['Загрузите папку']) => ev.on.get('export:done')?.('D:\\x', '', '', memo, false)

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
  jobs.exportPending = null
  jobs.exporting = null
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

// Выгрузка по просьбе агента (152): план приходит в ответе AgentAnswerExport и ждёт в exportPending; в окно «Готово» — в export:done.
describe('окно «Готово» после выгрузки агента: план из agentExport', () => {
  it('started + план → ход-заглушка и exportPending; после export:done-памятки окно «Готово для Фабрики» с кнопкой сайта', async () => {
    jobs.exportMemo = null
    jobs.agentExport({ started: true, print: plan() }, jobs.exportMark)
    expect(jobs.exporting).toEqual({ done: 0, total: 0 })
    expect(jobs.exportPending).toEqual(plan())
    expect(jobs.exportPrint).toBeNull()
    done()
    expect(jobs.exportPending).toBeNull()
    render(PrintDone)
    expect(screen.getByText('Готово для Фабрики')).toBeTruthy()
    await fireEvent.click(screen.getByText('Открыть Фабрику Фотокниги'))
    expect(api.OpenPrintSite).toHaveBeenCalledWith('fabrika')
  })
  it('окно «Готово» открыто, агент запускает новую выгрузку — окно не меняется до export:done новой; после — показывает новую', () => {
    jobs.exportPrint = plan({ kind: 'fsfera', done: 'Готово для Фотосферы', siteButton: 'Открыть Фотосферу' })
    render(PrintDone)
    expect(screen.getByText('Готово для Фотосферы')).toBeTruthy()
    jobs.agentExport({ started: true, print: plan() }, jobs.exportMark)
    expect(screen.getByText('Готово для Фотосферы')).toBeTruthy()
    expect(screen.getByText('Открыть Фотосферу')).toBeTruthy()
    expect(screen.queryByText('Готово для Фабрики')).toBeNull()
    done(['Новая памятка'])
    return vi.waitFor(() => {
      expect(screen.getByText('Готово для Фабрики')).toBeTruthy()
      expect(screen.getByText('Новая памятка')).toBeTruthy()
      expect(screen.queryByText('Открыть Фотосферу')).toBeNull()
    })
  })
  it('новая выгрузка закончилась ошибкой — открытое окно «Готово» прежнее, план в ожидании сброшен', () => {
    jobs.exportPrint = plan({ kind: 'fsfera' })
    jobs.agentExport({ started: true, print: plan() }, jobs.exportMark)
    ev.on.get('export:done')?.('D:\\x', 'нет места', '', null, false)
    expect(jobs.exportPrint).toEqual(plan({ kind: 'fsfera' }))
    expect(jobs.exportPending).toBeNull()
  })
  it('обычная выгрузка (плана нет) — после export:done прежний план не остаётся', () => {
    jobs.exportPrint = plan({ kind: 'fsfera' })
    jobs.agentExport({ started: true }, jobs.exportMark)
    expect(jobs.exportPrint).toEqual(plan({ kind: 'fsfera' }))
    done()
    expect(jobs.exportPrint).toBeNull()
  })
  it('started=false (человек остановил) — ни хода, ни плана не трогаем', () => {
    jobs.exportPrint = plan()
    jobs.agentExport({ started: false }, jobs.exportMark)
    expect(jobs.exporting).toBeNull()
    expect(jobs.exportPending).toBeNull()
    expect(jobs.exportPrint).not.toBeNull()
  })
  it('export:done обогнал ответ (mark устарел) — заглушку хода не ставим, план сразу в окно', () => {
    const mark = jobs.exportMark - 1
    jobs.agentExport({ started: true, print: plan() }, mark)
    expect(jobs.exporting).toBeNull()
    expect(jobs.exportPending).toBeNull()
    expect(jobs.exportPrint).toEqual(plan())
  })
})
