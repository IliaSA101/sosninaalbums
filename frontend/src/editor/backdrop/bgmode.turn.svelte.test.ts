import { beforeEach, describe, expect, it, vi } from 'vitest'

// Ответ Go подставляет тест; вызовы Api пишем в calls.
const calls: unknown[][] = []
const view = { rev: 2, folders: [], settings: {}, spreads: [] }

vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', () => ({
  TurnBackdrop: async (...a: unknown[]) => {
    calls.push(['TurnBackdrop', ...a])
    return view
  },
  SetBackdropBox: async (...a: unknown[]) => {
    calls.push(['SetBackdropBox', ...a])
    return view
  },
}))

vi.stubGlobal('window', globalThis)

const { bgmode } = await import('./bgmode.svelte')

beforeEach(() => {
  calls.length = 0
  bgmode.exit()
})

describe('bgmode.turn: поворот рамки фона считает Go (070 Г)', () => {
  it.each(['spread', 'left', 'right', 'spine'] as const)('область %s: один вызов TurnBackdrop, без SetBackdropBox', async (scope) => {
    bgmode.enter('s1', scope)
    await bgmode.turn(90)
    expect(calls).toEqual([['TurnBackdrop', 's1', scope, 90, '']])
  })
  it('«0°» (двойной щелчок ⟳) и поле — тот же вызов с углом', async () => {
    bgmode.enter('s1', 'left')
    await bgmode.turn(0)
    bgmode.enter('s1', 'left') // ответ-заглушка без разворотов закрыл режим
    await bgmode.turn(-45)
    expect(calls.map((c) => c[3])).toEqual([0, -45])
  })
  it('вне режима фона — ничего не зовёт', async () => {
    await bgmode.turn(90)
    expect(calls).toEqual([])
  })
})
