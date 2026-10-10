import { beforeEach, describe, expect, it, vi } from 'vitest'

// Go и store подменены: проверяем только порядок «набранное → ожидание цепочки».
const calls: string[] = []
vi.mock('../../base/core/store.svelte', () => ({
  store: {
    run: vi.fn(async (p: Promise<unknown>) => (await p, true)),
    spread: undefined,
    view: { spreads: [] },
    selectedCell: null,
    fonts: [],
  },
  Api: {
    SetText: vi.fn(async (id: string, text: string) => {
      calls.push(`${id}:${text}`)
    }),
    SetCaption: vi.fn(),
  },
}))

import { edit, SEND_DELAY } from './edit.svelte'

beforeEach(() => {
  vi.stubGlobal('window', globalThis)
  calls.length = 0
  edit.sel = { kind: 'text', spread: 's1', id: 't1' }
  edit.target = null
  edit.closing = false
})

describe('edit.idle', () => {
  it('досылает набранное сразу, не дожидаясь паузы', async () => {
    edit.startEdit()
    edit.input('привет')
    expect(calls).toEqual([])
    await edit.idle()
    expect(calls).toEqual(['t1:привет'])
    // Таймер погашен: через SEND_DELAY вторая отправка не придёт.
    await new Promise((r) => setTimeout(r, SEND_DELAY + 50))
    expect(calls).toEqual(['t1:привет'])
  })

  it('без набранного ничего не шлёт', async () => {
    edit.startEdit()
    await edit.idle()
    expect(calls).toEqual([])
  })
})
