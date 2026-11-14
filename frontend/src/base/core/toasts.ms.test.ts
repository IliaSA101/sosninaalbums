import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', () => ({}))
vi.stubGlobal('window', globalThis)

const { Toasts } = await import('./toasts.svelte')

beforeEach(() => vi.useFakeTimers())
afterEach(() => vi.useRealTimers())

describe('Toasts.notify: время жизни', () => {
  it('прежние тосты не изменились: 4 с без кнопки, 8 с с кнопкой', () => {
    const t = new Toasts()
    t.notify('просто')
    vi.advanceTimersByTime(3999)
    expect(t.toast).not.toBeNull()
    vi.advanceTimersByTime(1)
    expect(t.toast).toBeNull()
    t.notify('с кнопкой', 'info', { label: 'Вернуть', run: () => {} })
    vi.advanceTimersByTime(7999)
    expect(t.toast).not.toBeNull()
    vi.advanceTimersByTime(1)
    expect(t.toast).toBeNull()
  })

  it('итог подгона по лицам: 20 с, с кнопкой и без неё', () => {
    const t = new Toasts()
    t.notify('итог', 'info', { label: 'Показать', run: () => {} }, 20_000)
    vi.advanceTimersByTime(19_999)
    expect(t.toast?.text).toBe('итог')
    vi.advanceTimersByTime(1)
    expect(t.toast).toBeNull()
    t.notify('итог без кнопки', 'info', undefined, 20_000)
    vi.advanceTimersByTime(19_999)
    expect(t.toast).not.toBeNull()
    vi.advanceTimersByTime(1)
    expect(t.toast).toBeNull()
  })

  it('новый тост перезапускает таймер старого', () => {
    const t = new Toasts()
    t.notify('первый', 'info', undefined, 20_000)
    vi.advanceTimersByTime(15_000)
    t.notify('второй')
    vi.advanceTimersByTime(4000)
    expect(t.toast).toBeNull()
  })
})
