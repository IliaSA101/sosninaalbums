import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { HINT_MS, hintDue, wheelHint } from './wheelhint.svelte'

const base = { shown: false, wheeled: false, photos: true, since: 1000 }

describe('hintDue', () => {
  it('после 700 мс', () => {
    expect(hintDue(base, 1000 + HINT_MS - 1)).toBe(false)
    expect(hintDue(base, 1000 + HINT_MS)).toBe(true)
  })
  it('уже показывали, колесо крутили, тащат разворот — нет', () => {
    expect(hintDue({ ...base, shown: true }, 5000)).toBe(false)
    expect(hintDue({ ...base, wheeled: true }, 5000)).toBe(false)
    expect(hintDue({ ...base, photos: false }, 5000)).toBe(false)
  })
})

describe('wheelHint', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    wheelHint.fresh()
  })
  afterEach(() => {
    wheelHint.fresh()
    vi.useRealTimers()
  })

  it('после 700 мс', () => {
    wheelHint.begin(true, 0)
    vi.advanceTimersByTime(HINT_MS - 1)
    expect(wheelHint.show).toBe(false)
    wheelHint.check(HINT_MS)
    expect(wheelHint.show).toBe(true)
    wheelHint.end()
    expect(wheelHint.show).toBe(false)
  })

  it('один раз за сеанс', () => {
    wheelHint.begin(true, 0)
    wheelHint.check(HINT_MS)
    expect(wheelHint.show).toBe(true)
    wheelHint.end()
    wheelHint.begin(true, 10_000)
    wheelHint.check(20_000)
    expect(wheelHint.show).toBe(false)
  })

  it('колесо раньше — не показываем', () => {
    wheelHint.begin(true, 0)
    wheelHint.wheel()
    wheelHint.check(HINT_MS)
    expect(wheelHint.show).toBe(false)
    wheelHint.end()
    // и в следующих переносах тоже
    wheelHint.begin(true, 10_000)
    wheelHint.check(20_000)
    expect(wheelHint.show).toBe(false)
  })

  it('разворот — без подсказки', () => {
    wheelHint.begin(false, 0)
    wheelHint.check(HINT_MS * 3)
    expect(wheelHint.show).toBe(false)
  })

  it('перенос кончился раньше 700 мс — таймер не показывает строку', () => {
    wheelHint.begin(true, performance.now())
    wheelHint.end()
    vi.advanceTimersByTime(HINT_MS * 2)
    expect(wheelHint.show).toBe(false)
  })
})
