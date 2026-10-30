// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/svelte'
import { flushSync } from 'svelte'
import type { AgentAsk } from './agents'

// Вопросы выгрузки и открытия (152): виды export и open. Остальное поведение окна (600 мс, Esc, askdone) — в agentask.dom.test.ts. Go подменён.
const ev = vi.hoisted(() => ({ on: new Map<string, (d: unknown) => void>() }))
vi.mock('../../../wailsjs/runtime/runtime', () => ({
  EventsOn: (name: string, cb: (d: unknown) => void) => {
    ev.on.set(name, cb)
    return () => ev.on.delete(name)
  },
}))
const api = vi.hoisted(() => ({
  AnswerAgent: vi.fn(),
  AgentAnswerExport: vi.fn(),
  AgentAnswerOpen: vi.fn(),
  AgentsInfo: vi.fn(),
  AgentsBusy: vi.fn(),
}))
vi.mock('../../../wailsjs/go/app/App', () => api)

const { default: AgentAskDialog } = await import('./AgentAskDialog.svelte')
const { agentAsk, ARM_MS } = await import('./agentask.svelte')
const { store } = await import('../../base/core/store.svelte')
const { jobs } = await import('../../base/core/jobs.svelte')
const { edit } = await import('../../editor/text/edit.svelte')

const ask = (o: Partial<AgentAsk> = {}): AgentAsk => ({
  id: 'a1',
  kind: 'export',
  title: '',
  lines: [],
  hint: '',
  ok: '',
  path: '',
  link: false,
  alt: '',
  tool: '',
  why: '',
  ...o,
})
const state = (rev = 7) => ({ view: { rev, folders: [], spreads: [], settings: {} }, photos: [] })
const emit = (name: string, d: unknown) => ev.on.get(name)?.(d)
const dialog = () => screen.queryByRole('dialog')
const cancelBtn = () => screen.getByRole('button', { name: 'Отмена' })
const okBtn = () => screen.getByRole('button', { name: 'Выбрать папку…' })
function shown(a: AgentAsk) {
  render(AgentAskDialog)
  emit('agents:ask', a)
  flushSync()
}
const arm = () => {
  vi.advanceTimersByTime(ARM_MS)
  flushSync()
}

beforeEach(() => {
  vi.useFakeTimers()
  api.AnswerAgent.mockReset()
  api.AnswerAgent.mockResolvedValue({ rev: 8, folders: [], spreads: [], settings: {} })
  api.AgentsBusy.mockReset()
  api.AgentsBusy.mockResolvedValue(undefined)
})
afterEach(() => {
  cleanup()
  agentAsk.close()
  store.toast = null
  vi.useRealTimers()
})

// Выгрузка и открытие по просьбе агента (152): виды export и open. Tool пуст — «Не спрашивать» нет; строки приходят из Go.
describe('AgentAskDialog: выгрузка и открытие (152)', () => {
  const DIR = 'D:\\Фото\\Выгрузки\\Петровы'
  const exp = (o: Partial<AgentAsk> = {}) =>
    ask({
      id: 'e1',
      kind: 'export',
      title: 'Выгрузить альбом в папку «Петровы»?',
      lines: [
        'Claude Code просит выгрузить альбом «Свадьба Петровых» в JPEG:',
        'Развороты 1–24 и обложка · 300 dpi · качество 95 · 25 файлов',
        'Папка будет создана',
      ],
      hint: '',
      ok: 'Выгрузить',
      path: DIR,
      alt: 'Выбрать другую…',
      tool: '',
      files: 25,
      create: true,
      ...o,
    })
  const opn = (o: Partial<AgentAsk> = {}) =>
    ask({
      id: 'o1',
      kind: 'open',
      title: 'Открыть альбом «Свадьба Ивановы»?',
      lines: ['Claude Code просит открыть альбом.', 'Открытый «Альбом Петровых» сохранится в свой файл.'],
      hint: '',
      ok: 'Открыть',
      path: 'D:\\Альбомы\\Свадьба Ивановы.salbum',
      alt: 'Выбрать другой…',
      tool: '',
      ...o,
    })
  const btns = () => [...screen.getByRole('dialog').querySelectorAll('.actions button')].map((b) => b.textContent)
  const started = (print: unknown = null) => ({ started: true, files: 25, print })

  beforeEach(() => {
    api.AgentAnswerExport.mockReset()
    api.AgentAnswerExport.mockResolvedValue(started())
    api.AgentAnswerOpen.mockReset()
    api.AgentAnswerOpen.mockResolvedValue(state(11))
    jobs.exporting = null
    jobs.exportPrint = null
    jobs.exportPending = null
  })
  afterEach(() => {
    jobs.exporting = null
    jobs.exportPrint = null
    jobs.exportPending = null
  })

  it('export: путь, строки Go и «Папка будет создана», «Отмена · Выбрать другую… · Выгрузить», ссылки «Не спрашивать» нет', () => {
    shown(exp())
    const d = screen.getByRole('dialog')
    expect(d.getAttribute('aria-label')).toBe('Выгрузить альбом в папку «Петровы»?')
    expect(d.querySelector('[data-path]')?.textContent).toBe(DIR)
    expect(d.querySelector('[data-path]')?.classList.contains('new')).toBe(true)
    expect(screen.getByText('Папка будет создана')).toBeTruthy()
    expect(screen.getByText(/25 файлов/)).toBeTruthy()
    expect(btns()).toEqual(['Отмена', 'Выбрать другую…', 'Выгрузить'])
    expect(screen.queryByText(/Не спрашивать/)).toBeNull()
  })

  it('export: папка уже есть — путь без пунктира', () => {
    shown(exp({ create: false }))
    expect(screen.getByRole('dialog').querySelector('[data-path]')?.classList.contains('new')).toBe(false)
  })

  it('export: «Выгрузить» → AgentAnswerExport(id, false); ход-заглушка и план типографии ставит jobs.agentExport', async () => {
    const plan = { kind: 'fabrika', done: 'Готово для Фабрики' }
    api.AgentAnswerExport.mockResolvedValue(started(plan))
    shown(exp())
    arm()
    await fireEvent.click(screen.getByRole('button', { name: 'Выгрузить' }))
    await vi.waitFor(() => expect(dialog()).toBeNull())
    expect(api.AgentAnswerExport).toHaveBeenCalledExactlyOnceWith('e1', false)
    expect(api.AnswerAgent).not.toHaveBeenCalled()
    expect(jobs.exporting).toEqual({ done: 0, total: 0 })
    expect(jobs.exportPending).toEqual(plan)
    expect(jobs.exportPrint).toBeNull()
  })

  it('export: «Выбрать другую…» → AgentAnswerExport(id, true)', async () => {
    shown(exp())
    arm()
    await fireEvent.click(screen.getByRole('button', { name: 'Выбрать другую…' }))
    await vi.waitFor(() => expect(dialog()).toBeNull())
    expect(api.AgentAnswerExport).toHaveBeenCalledExactlyOnceWith('e1', true)
  })

  it('export: человек остановил (started=false) — окно закрыто, хода и плана нет, тоста нет', async () => {
    api.AgentAnswerExport.mockResolvedValue({ started: false, files: 0 })
    shown(exp())
    arm()
    await fireEvent.click(screen.getByRole('button', { name: 'Выгрузить' }))
    await vi.waitFor(() => expect(dialog()).toBeNull())
    expect(jobs.exporting).toBeNull()
    expect(jobs.exportPrint).toBeNull()
    expect(store.toast).toBeNull()
  })

  it('export: выгрузка закончилась до ответа Go (export:done обогнал) — ход не рисуем, план остаётся для окна «Готово»', async () => {
    jobs.listen()
    const plan = { kind: 'fabrika', done: 'Готово для Фабрики' }
    api.AgentAnswerExport.mockImplementation(async () => {
      const done = ev.on.get('export:done') as (...a: unknown[]) => void
      done('D:\\x', '', '', ['Загрузите папку'], false)
      return started(plan)
    })
    shown(exp())
    arm()
    await fireEvent.click(screen.getByRole('button', { name: 'Выгрузить' }))
    await vi.waitFor(() => expect(dialog()).toBeNull())
    expect(jobs.exporting).toBeNull()
    expect(jobs.exportPrint).toEqual(plan)
    expect(jobs.exportMemo).toEqual(['Загрузите папку'])
    jobs.exportMemo = null
  })

  it('export: ошибка Go после щелчка — красный тост, окно закрыто', async () => {
    api.AgentAnswerExport.mockRejectedValue(new Error('не удалось создать папку'))
    shown(exp())
    arm()
    await fireEvent.click(screen.getByRole('button', { name: 'Выгрузить' }))
    await vi.waitFor(() => expect(dialog()).toBeNull())
    expect(store.toast?.text).toBe('Не удалось создать папку')
    expect(store.toast?.kind).toBe('error')
  })

  it('export: подсказка негодна (why=bad), пути нет — строка причины и «Отмена · Выбрать папку…»; OK без второго аргумента other', async () => {
    shown(exp({ path: '', why: 'bad', ok: 'Выбрать папку…', alt: '', create: false, files: 25 }))
    expect(screen.getByText(/не берёт без вас/)).toBeTruthy()
    expect(btns()).toEqual(['Отмена', 'Выбрать папку…'])
    arm()
    await fireEvent.click(okBtn())
    await vi.waitFor(() => expect(dialog()).toBeNull())
    expect(api.AgentAnswerExport).toHaveBeenCalledExactlyOnceWith('e1', false)
  })

  it('export: «Отмена» — отказ AnswerAgent(id, false), выгрузка не запускается', async () => {
    shown(exp())
    arm()
    await fireEvent.click(cancelBtn())
    await vi.waitFor(() => expect(dialog()).toBeNull())
    expect(api.AnswerAgent).toHaveBeenCalledExactlyOnceWith('e1', false)
    expect(api.AgentAnswerExport).not.toHaveBeenCalled()
  })

  it('export: первые 600 мс «Выгрузить» не срабатывает', async () => {
    shown(exp())
    await fireEvent.click(screen.getByRole('button', { name: 'Выгрузить' }))
    expect(api.AgentAnswerExport).not.toHaveBeenCalled()
  })

  it('open: «Отмена · Выбрать другой… · Открыть»; «Открыть» → AgentAnswerOpen(id, false) внутри замены проекта', async () => {
    const idle = vi.spyOn(edit, 'idle').mockResolvedValue(undefined)
    shown(opn())
    expect(btns()).toEqual(['Отмена', 'Выбрать другой…', 'Открыть'])
    expect(screen.queryByText(/Не спрашивать/)).toBeNull()
    arm()
    await fireEvent.click(screen.getByRole('button', { name: 'Открыть' }))
    await vi.waitFor(() => expect(dialog()).toBeNull())
    expect(idle).toHaveBeenCalled()
    expect(api.AgentAnswerOpen).toHaveBeenCalledExactlyOnceWith('o1', false)
    expect(store.view.rev).toBe(11)
    idle.mockRestore()
  })

  it('open: «Выбрать другой…» → AgentAnswerOpen(id, true)', async () => {
    shown(opn())
    arm()
    await fireEvent.click(screen.getByRole('button', { name: 'Выбрать другой…' }))
    await vi.waitFor(() => expect(dialog()).toBeNull())
    expect(api.AgentAnswerOpen).toHaveBeenCalledExactlyOnceWith('o1', true)
  })

  it('open без ключа: пути нет, «Отмена · Выбрать файл…» → AgentAnswerOpen(id, false)', async () => {
    shown(opn({ path: '', ok: 'Выбрать файл…', alt: '' }))
    expect(btns()).toEqual(['Отмена', 'Выбрать файл…'])
    arm()
    await fireEvent.click(screen.getByRole('button', { name: 'Выбрать файл…' }))
    await vi.waitFor(() => expect(dialog()).toBeNull())
    expect(api.AgentAnswerOpen).toHaveBeenCalledExactlyOnceWith('o1', false)
  })

  it('open: Go ответил cancelled (окно ОС закрыто) — вид и выделение целы, окно закрыто', async () => {
    api.AgentAnswerOpen.mockResolvedValue({ cancelled: true, view: { rev: 99, folders: [], spreads: [], settings: {} }, photos: [] })
    const rev = store.view.rev
    shown(opn({ path: '', ok: 'Выбрать файл…', alt: '' }))
    arm()
    await fireEvent.click(screen.getByRole('button', { name: 'Выбрать файл…' }))
    await vi.waitFor(() => expect(dialog()).toBeNull())
    expect(store.view.rev).toBe(rev)
    expect(store.toast).toBeNull()
  })

  it('open: ошибка (ErrNewer, битый файл) — красный тост, окно закрыто', async () => {
    api.AgentAnswerOpen.mockRejectedValue(new Error('файл повреждён'))
    shown(opn())
    arm()
    await fireEvent.click(screen.getByRole('button', { name: 'Открыть' }))
    await vi.waitFor(() => expect(dialog()).toBeNull())
    expect(store.toast?.kind).toBe('error')
  })
})
