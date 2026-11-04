import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { zoomui } from './zoomui.svelte'
import { PEEK_MS } from './zoomui'

beforeEach(() => vi.useFakeTimers())
afterEach(() => {
  zoomui.hover(false)
  zoomui.close()
  vi.useRealTimers()
})

describe('zoomui (таймер)', () => {
  it('подсказка гаснет через 1500 мс', () => {
    zoomui.pulse()
    expect(zoomui.state).toBe('peek')
    vi.advanceTimersByTime(PEEK_MS - 1)
    expect(zoomui.state).toBe('peek')
    vi.advanceTimersByTime(1)
    expect(zoomui.state).toBe('closed')
  })
  it('новый pulse перезапускает срок', () => {
    zoomui.pulse()
    vi.advanceTimersByTime(1000)
    zoomui.pulse()
    vi.advanceTimersByTime(1000)
    expect(zoomui.state).toBe('peek')
    vi.advanceTimersByTime(500)
    expect(zoomui.state).toBe('closed')
  })
  it('курсор над бегунком держит; ушёл — срок заново', () => {
    zoomui.pulse()
    zoomui.hover(true)
    vi.advanceTimersByTime(10_000)
    expect(zoomui.state).toBe('peek')
    zoomui.hover(false)
    vi.advanceTimersByTime(PEEK_MS)
    expect(zoomui.state).toBe('closed')
  })
  it('pulse при наведении не запускает таймер', () => {
    zoomui.hover(true)
    zoomui.pulse()
    vi.advanceTimersByTime(10_000)
    expect(zoomui.state).toBe('peek')
  })
  it('grab закрепляет: таймер не гасит; pulse в open холостой', () => {
    zoomui.pulse()
    zoomui.grab()
    zoomui.pulse()
    vi.advanceTimersByTime(10_000)
    expect(zoomui.state).toBe('open')
  })
  it('toggle и close', () => {
    zoomui.toggle()
    expect(zoomui.state).toBe('open')
    zoomui.toggle()
    expect(zoomui.state).toBe('closed')
    zoomui.open()
    zoomui.close()
    expect(zoomui.state).toBe('closed')
  })
})
