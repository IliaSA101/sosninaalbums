// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/svelte'
import { flushSync } from 'svelte'
import type { AgentAsk } from './agents'

// Вопрос агента (149): 600 мс `aria-disabled`, фокус на «Отмена», Enter и пробел — отказ, подсказка в <bdi>,
// «Выбрать папку…» (create) → edit.idle() → AgentAnswerCreate, agents:askdone закрывает окно. Go подменён.
const ev = vi.hoisted(() => ({ on: new Map<string, (d: unknown) => void>() }))
vi.mock('../../../wailsjs/runtime/runtime', () => ({
  EventsOn: (name: string, cb: (d: unknown) => void) => {
    ev.on.set(name, cb)
    return () => ev.on.delete(name)
  },
}))
const api = vi.hoisted(() => ({
  AnswerAgent: vi.fn(),
  AgentAnswerCreate: vi.fn(),
  AgentsBusy: vi.fn(),
}))
vi.mock('../../../wailsjs/go/app/App', () => api)

const { default: AgentAskDialog } = await import('./AgentAskDialog.svelte')
const { agentAsk, ARM_MS } = await import('./agentask.svelte')
const { store } = await import('../../base/core/store.svelte')
const { edit } = await import('../../editor/text/edit.svelte')
const { modalStack } = await import('../../base/modal/modalstack.svelte')

const ask = (o: Partial<AgentAsk> = {}): AgentAsk => ({
  id: 'a1',
  kind: 'create',
  title: 'Создать альбом по просьбе агента?',
  lines: ['Claude Code просит создать новый альбом.', 'Открытый «Альбом Петровых» сохранится в свой файл.'],
  hint: 'Петровы',
  ok: 'Выбрать папку…',
  ...o,
})
const state = (rev = 7) => ({ view: { rev, folders: [], spreads: [], settings: {} }, photos: [] })
const emit = (name: string, d: unknown) => ev.on.get(name)?.(d)
const dialog = () => screen.queryByRole('dialog')
const cancelBtn = () => screen.getByRole('button', { name: 'Отмена' })
const okBtn = () => screen.getByRole('button', { name: 'Выбрать папку…' })

// Показать вопрос через событие Go.
function shown(a = ask()) {
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
  api.AgentAnswerCreate.mockReset()
  api.AgentAnswerCreate.mockResolvedValue(state())
  api.AgentsBusy.mockReset()
  api.AgentsBusy.mockResolvedValue(undefined)
})
afterEach(() => {
  cleanup()
  agentAsk.close()
  store.toast = null
  vi.useRealTimers()
})

describe('AgentAskDialog: вид', () => {
  it('нет вопроса — окна в DOM нет', () => {
    render(AgentAskDialog)
    expect(dialog()).toBeNull()
  })

  it('agents:ask → окно с заголовком и абзацами Go; «Отмена» слева, «Выбрать папку…» справа', () => {
    shown()
    expect(dialog()?.getAttribute('aria-label')).toBe('Создать альбом по просьбе агента?')
    expect(screen.getByText('Claude Code просит создать новый альбом.')).toBeTruthy()
    expect(screen.getByText('Открытый «Альбом Петровых» сохранится в свой файл.')).toBeTruthy()
    const btns = [...screen.getByRole('dialog').querySelectorAll('.actions button')]
    expect(btns.map((b) => b.textContent)).toEqual(['Отмена', 'Выбрать папку…'])
    expect(modalStack.any()).toBe(true)
  })

  it('подсказка агента — в <bdi>, текстом; своих слов агента в окне нет, кроме имени папки', () => {
    shown(ask({ hint: '<b>Петровы</b>' }))
    const b = screen.getByRole('dialog').querySelector('bdi')
    expect(b?.textContent).toBe('<b>Петровы</b>')
    expect(b?.querySelector('b')).toBeNull()
    expect(b?.parentElement?.textContent).toBe('Агент предлагает папку «<b>Петровы</b>».')
  })

  it('подсказки нет — строки про папку нет', () => {
    shown(ask({ hint: '' }))
    expect(screen.queryByText(/Агент предлагает папку/)).toBeNull()
    expect(screen.getByRole('dialog').querySelector('bdi')).toBeNull()
  })

  it('lines = null от Go не ломает окно', () => {
    shown(ask({ lines: null as unknown as string[], hint: 'Петровы' }))
    expect(screen.getByText(/Агент предлагает папку/)).toBeTruthy()
  })
})

describe('AgentAskDialog: первые 600 мс', () => {
  it('кнопки aria-disabled (не disabled), фокус на «Отмена»', () => {
    shown()
    for (const b of [cancelBtn(), okBtn()]) {
      expect(b.getAttribute('aria-disabled')).toBe('true')
      expect((b as HTMLButtonElement).disabled).toBe(false)
    }
    expect(document.activeElement).toBe(cancelBtn())
  })

  it('щелчки и Enter в это время игнорируются: Go не зовут, окно на месте', async () => {
    shown()
    await fireEvent.click(cancelBtn())
    await fireEvent.click(okBtn())
    await fireEvent.keyDown(document.activeElement as Element, { key: 'Enter' })
    await fireEvent.keyDown(document.activeElement as Element, { key: ' ' })
    expect(api.AnswerAgent).not.toHaveBeenCalled()
    expect(api.AgentAnswerCreate).not.toHaveBeenCalled()
    expect(dialog()).not.toBeNull()
  })

  it('через 600 мс кнопки живые', () => {
    shown()
    vi.advanceTimersByTime(ARM_MS - 1)
    flushSync()
    expect(okBtn().getAttribute('aria-disabled')).toBe('true')
    vi.advanceTimersByTime(1)
    flushSync()
    expect(cancelBtn().getAttribute('aria-disabled')).toBe('false')
    expect(okBtn().getAttribute('aria-disabled')).toBe('false')
  })

  it('новый вопрос заводит отсчёт заново', () => {
    shown()
    arm()
    emit('agents:ask', ask({ id: 'a2' }))
    flushSync()
    expect(okBtn().getAttribute('aria-disabled')).toBe('true')
  })

  it('Esc — отказ и в первые 600 мс', async () => {
    shown()
    await fireEvent.keyDown(document.activeElement as Element, { key: 'Escape' })
    expect(api.AnswerAgent).toHaveBeenCalledExactlyOnceWith('a1', false)
    expect(dialog()).toBeNull()
  })

  it('✕ и щелчок по фону в первые 600 мс ничего не шлют, после — отказ', async () => {
    shown()
    await fireEvent.click(screen.getByRole('button', { name: 'Закрыть' }))
    await fireEvent.pointerDown(screen.getByRole('dialog').parentElement as Element)
    expect(api.AnswerAgent).not.toHaveBeenCalled()
    expect(dialog()).not.toBeNull()
    arm()
    await fireEvent.click(screen.getByRole('button', { name: 'Закрыть' }))
    expect(api.AnswerAgent).toHaveBeenCalledExactlyOnceWith('a1', false)
    expect(dialog()).toBeNull()
  })

  it('щелчок по фону после 600 мс — отказ', async () => {
    shown()
    arm()
    await fireEvent.pointerDown(screen.getByRole('dialog').parentElement as Element)
    expect(api.AnswerAgent).toHaveBeenCalledExactlyOnceWith('a1', false)
  })

  it('пробел на «Отмена» до 600 мс — ничего не шлёт', async () => {
    shown()
    cancelBtn().focus()
    await fireEvent.keyDown(cancelBtn(), { key: ' ' })
    expect(api.AnswerAgent).not.toHaveBeenCalled()
    expect(dialog()).not.toBeNull()
  })
})

describe('AgentAskDialog: ответы', () => {
  it('«Отмена» → AnswerAgent(id, false), окно закрыто, альбом не тронут', async () => {
    shown()
    arm()
    await fireEvent.click(cancelBtn())
    expect(api.AnswerAgent).toHaveBeenCalledExactlyOnceWith('a1', false)
    expect(api.AgentAnswerCreate).not.toHaveBeenCalled()
    expect(dialog()).toBeNull()
  })

  it('Enter → отказ (даже когда фокус на «Выбрать папку…»)', async () => {
    shown()
    arm()
    okBtn().focus()
    await fireEvent.keyDown(okBtn(), { key: 'Enter' })
    expect(api.AnswerAgent).toHaveBeenCalledExactlyOnceWith('a1', false)
    expect(api.AgentAnswerCreate).not.toHaveBeenCalled()
  })

  it('пробел на «Отмена» и на самом окне → отказ', async () => {
    shown()
    arm()
    await fireEvent.keyDown(cancelBtn(), { key: ' ' })
    expect(api.AnswerAgent).toHaveBeenCalledExactlyOnceWith('a1', false)
    cleanup()
    agentAsk.close()
    api.AnswerAgent.mockClear()
    shown()
    arm()
    await fireEvent.keyDown(screen.getByRole('dialog'), { key: ' ' })
    expect(api.AnswerAgent).toHaveBeenCalledExactlyOnceWith('a1', false)
  })

  it('пробел на «Выбрать папку…» отказом не становится (Tab + пробел — обычное нажатие кнопки)', async () => {
    shown()
    arm()
    okBtn().focus()
    const e = new KeyboardEvent('keydown', { key: ' ', bubbles: true, cancelable: true })
    okBtn().dispatchEvent(e)
    expect(e.defaultPrevented).toBe(false)
    expect(api.AnswerAgent).not.toHaveBeenCalled()
  })

  it('create: «Выбрать папку…» → edit.idle() → AgentAnswerCreate, затем окно закрыто, альбом заменён', async () => {
    const order: string[] = []
    const idle = vi.spyOn(edit, 'idle').mockImplementation(async () => void order.push('idle'))
    api.AgentAnswerCreate.mockImplementation(async () => {
      order.push('create')
      return state(7)
    })
    shown()
    arm()
    await fireEvent.click(okBtn())
    await vi.waitFor(() => expect(dialog()).toBeNull())
    expect(order).toEqual(['idle', 'create'])
    expect(api.AgentAnswerCreate).toHaveBeenCalledExactlyOnceWith('a1')
    expect(api.AnswerAgent).not.toHaveBeenCalled()
    expect(store.view.rev).toBe(7)
    idle.mockRestore()
  })

  it('create: Go ответил cancelled (диалог закрыт, папка не подошла, флажок выключен) — разворот и выделение целы, окно закрыто', async () => {
    store.apply({
      rev: 3,
      folders: [],
      spreads: [
        { id: 's1', photos: [] },
        { id: 's2', photos: [] },
        { id: 's3', photos: [] },
      ],
      settings: {},
    } as never)
    store.goId('s3')
    store.selected.add('p1')
    api.AgentAnswerCreate.mockResolvedValue({ cancelled: true, view: { rev: 9, folders: [], spreads: [], settings: {} }, photos: [] })
    shown()
    arm()
    await fireEvent.click(okBtn())
    await vi.waitFor(() => expect(dialog()).toBeNull())
    expect(api.AgentAnswerCreate).toHaveBeenCalledExactlyOnceWith('a1')
    expect(store.currentId).toBe('s3')
    expect(store.view.rev).toBe(3)
    expect([...store.selected]).toEqual(['p1'])
    expect(store.toast).toBeNull()
    store.selected.clear()
  })

  it('add: «Выбрать папку…» → AnswerAgent(id, true)', async () => {
    shown(ask({ kind: 'add', title: 'Добавить папку по просьбе агента?' }))
    arm()
    await fireEvent.click(okBtn())
    await vi.waitFor(() => expect(dialog()).toBeNull())
    expect(api.AnswerAgent).toHaveBeenCalledExactlyOnceWith('a1', true)
    expect(api.AgentAnswerCreate).not.toHaveBeenCalled()
  })

  it('пока открыт выбор папки ОС: повторный щелчок и «Отмена» в Go не уходят', async () => {
    let finish: (s: unknown) => void = () => {}
    api.AgentAnswerCreate.mockReturnValue(new Promise((r) => (finish = r)))
    shown()
    arm()
    await fireEvent.click(okBtn())
    await fireEvent.click(okBtn())
    await fireEvent.click(cancelBtn())
    await fireEvent.keyDown(document.activeElement as Element, { key: 'Enter' })
    expect(api.AgentAnswerCreate).toHaveBeenCalledTimes(1)
    expect(api.AnswerAgent).not.toHaveBeenCalled()
    expect(dialog()).not.toBeNull()
    finish(state())
    await vi.waitFor(() => expect(dialog()).toBeNull())
  })

  it('Go ответил ошибкой — тост со словами ошибки, окно закрыто', async () => {
    api.AgentAnswerCreate.mockRejectedValue(new Error('вопрос уже закрыт'))
    shown()
    arm()
    await fireEvent.click(okBtn())
    await vi.waitFor(() => expect(dialog()).toBeNull())
    expect(store.toast?.kind).toBe('error')
    expect(store.toast?.text).toBe('Вопрос уже закрыт')
  })

  it('отказ, который Go уже не принял (срок вышел), — тишина', async () => {
    api.AnswerAgent.mockRejectedValue(new Error('вопрос закрыт'))
    shown()
    arm()
    await fireEvent.click(cancelBtn())
    await vi.waitFor(() => expect(api.AnswerAgent).toHaveBeenCalled())
    expect(dialog()).toBeNull()
    expect(store.toast).toBeNull()
  })
})

describe('AgentAskDialog: agents:askdone', () => {
  it('закрывает окно своего вопроса; «answered» без тоста', () => {
    shown()
    emit('agents:askdone', { id: 'a1', why: 'answered' })
    flushSync()
    expect(dialog()).toBeNull()
    expect(store.toast).toBeNull()
  })

  it('timeout — тост «Агент не дождался ответа»', () => {
    shown()
    emit('agents:askdone', { id: 'a1', why: 'timeout' })
    flushSync()
    expect(dialog()).toBeNull()
    expect(store.toast?.text).toBe('Агент не дождался ответа')
  })

  it.each(['declined', 'gone', 'closed', 'off'])('%s закрывает окно молча', (why) => {
    shown()
    emit('agents:askdone', { id: 'a1', why })
    flushSync()
    expect(dialog()).toBeNull()
    expect(store.toast).toBeNull()
  })

  it('чужой id окно не трогает', () => {
    shown()
    emit('agents:askdone', { id: 'другой', why: 'timeout' })
    flushSync()
    expect(dialog()).not.toBeNull()
    expect(store.toast).toBeNull()
  })

  it('закрыто во время выбора папки (выход) — окно уходит', () => {
    api.AgentAnswerCreate.mockReturnValue(new Promise(() => {}))
    shown()
    arm()
    okBtn().click()
    emit('agents:askdone', { id: 'a1', why: 'closed' })
    flushSync()
    expect(dialog()).toBeNull()
  })
})
