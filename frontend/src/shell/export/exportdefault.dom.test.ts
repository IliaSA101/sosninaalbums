// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render } from '@testing-library/svelte'
import type { PrintPlan } from '../../base/core/types'

// Выбор в «Печати» при открытии окна (123 А2): первый ответ Go с Plan.default ставит пункт, дальше решает фотограф.
const api = vi.hoisted(() => ({ PrintPlans: vi.fn(), Export: vi.fn(), MissingNames: vi.fn(async () => ({})) }))
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', () => api)

const { default: ExportDialog } = await import('./ExportDialog.svelte')
const { store } = await import('../../base/core/store.svelte')
const { jobs } = await import('../../base/core/jobs.svelte')

const plan = (o: Partial<PrintPlan>): PrintPlan =>
  ({
    kind: 'fsfera',
    title: 'Для Фотосферы Pro',
    format: '21×30',
    off: '',
    folder: 'Иванов',
    quality: 0,
    block: [],
    warn: [],
    memo: [],
    coverFix: false,
    fixAt: -1,
    default: false,
    px: '',
    ...o,
  }) as PrintPlan
const fsfera = (d = false) => plan({ default: d })
const fabrika = (d = false) => plan({ kind: 'fabrika', title: 'Для Фабрики Фотокниги', quality: 100, default: d })

const tick = () => new Promise((r) => setTimeout(r, 0))
const select = () => document.querySelector('select[aria-label="Печать"]') as HTMLSelectElement
const open = async () => {
  jobs.exportOpen = true
  await tick()
  await tick()
}

beforeEach(() => {
  localStorage.clear()
  api.PrintPlans.mockReset()
  store.view = { rev: 1, folders: [], spreads: [], settings: { pageW: 300, pageH: 300, dpi: 300 }, cover: null } as never
  jobs.exportOpen = false
  jobs.exporting = null
})
afterEach(() => {
  cleanup()
  jobs.exportOpen = false
})

describe('имена файлов в подписях окна', () => {
  const withCover = () => {
    store.view = {
      rev: 1,
      folders: [],
      spreads: [{ id: 's1' }],
      settings: { pageW: 300, pageH: 300, dpi: 300 },
      cover: { sheet: { w: 600, h: 300 } },
    } as never
  }
  const hint = (label: string) =>
    [...document.querySelectorAll('label')].find((l) => l.textContent?.includes(label))?.querySelector('.hint')?.textContent
  it('обычная выгрузка: 00.jpg и 01.jpg', async () => {
    api.PrintPlans.mockResolvedValue([fsfera(false), fabrika(false)])
    withCover()
    render(ExportDialog)
    await open()
    expect(hint('Обложка')).toBe('(00.jpg)')
    expect(hint('Разворотами')).toBe('(01.jpg)')
  })
  it('Фабрика: имена из плана Go — 1-00.jpg и 1-01.jpg', async () => {
    api.PrintPlans.mockResolvedValue([fsfera(false), { ...fabrika(true), coverName: '1-00.jpg' }])
    withCover()
    render(ExportDialog)
    await open()
    expect(select().value).toBe('fabrika')
    expect(hint('Обложка')).toBe('(1-00.jpg)')
    expect(hint('Разворотами')).toBe('(1-01.jpg)')
    await fireEvent.change(select(), { target: { value: '' } }) // назад в обычную: прежние имена
    await tick()
    expect(hint('Обложка')).toBe('(00.jpg)')
    expect(hint('Разворотами')).toBe('(01.jpg)')
  })
})

describe('выбор в «Печати» при открытии', () => {
  it('Go отметил Фабрику — список открывается на ней; запомненное уходит в Go аргументом last', async () => {
    localStorage.setItem('exportOpts', JSON.stringify({ print: 'fsfera' }))
    api.PrintPlans.mockResolvedValue([fsfera(false), fabrika(true)])
    render(ExportDialog)
    await open()
    expect(api.PrintPlans).toHaveBeenCalledWith('fsfera')
    expect(select().value).toBe('fabrika')
  })
  it('Go не отметил ни один пункт — «Обычная выгрузка», хотя раньше была запомнена типография', async () => {
    localStorage.setItem('exportOpts', JSON.stringify({ print: 'fabrika' }))
    api.PrintPlans.mockResolvedValue([fsfera(false), fabrika(false)])
    render(ExportDialog)
    await open()
    expect(select().value).toBe('')
  })
  it('выбор руками запоминается, правка альбома при открытом окне его не сбрасывает', async () => {
    api.PrintPlans.mockResolvedValue([fsfera(true), fabrika(false)])
    render(ExportDialog)
    await open()
    expect(select().value).toBe('fsfera')
    await fireEvent.change(select(), { target: { value: 'fabrika' } })
    expect(JSON.parse(localStorage.getItem('exportOpts') ?? '{}').print).toBe('fabrika')
    store.view = { ...store.view, rev: 2 }
    await tick()
    await tick()
    expect(api.PrintPlans).toHaveBeenCalledTimes(2)
    expect(select().value).toBe('fabrika')
  })
  it('ответ Go приходит не сразу: пока планов нет, раздела нет, потом — пункт Go', async () => {
    let resolve: (p: PrintPlan[]) => void = () => {}
    api.PrintPlans.mockReturnValue(new Promise<PrintPlan[]>((r) => (resolve = r)))
    render(ExportDialog)
    jobs.exportOpen = true
    await tick()
    expect(select()).toBeNull()
    resolve([fsfera(true), fabrika(false)])
    await tick()
    await tick()
    expect(select().value).toBe('fsfera')
  })
  it('повторное открытие — снова пункт от Go, а не прошлый выбор в окне', async () => {
    api.PrintPlans.mockResolvedValue([fsfera(false), fabrika(true)])
    render(ExportDialog)
    await open()
    await fireEvent.change(select(), { target: { value: '' } })
    expect(select().value).toBe('')
    jobs.exportOpen = false
    await tick()
    api.PrintPlans.mockResolvedValue([fsfera(true), fabrika(false)])
    await open()
    expect(select().value).toBe('fsfera')
  })
})
