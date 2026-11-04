import { afterEach, describe, expect, it, vi } from 'vitest'
import { PinchTrack, pinching } from './pinch'

const at = (scale: number) => ({ scale, clientX: 10, clientY: 20 })

// Жесты живут в общем реестре страховки колеса: каждый тест чистит свои.
const made: PinchTrack[] = []
const track = () => {
  const t = new PinchTrack()
  made.push(t)
  return t
}
afterEach(() => {
  vi.useRealTimers()
  for (const t of made.splice(0)) t.cancel()
})

describe('PinchTrack (134-1)', () => {
  it('произведение множителей жеста равно итоговому scale', () => {
    const t = track()
    t.start(at(1))
    let prod = 1
    for (const s of [1.1, 1.25, 1.6, 1.9, 2.4]) prod *= t.step(at(s))
    prod *= t.end(at(3))
    expect(prod).toBeCloseTo(3, 12)
  })

  it('база — scale начала жеста, а не 1', () => {
    const t = track()
    t.start(at(1.2))
    expect(t.step(at(1.5))).toBeCloseTo(1.25, 12)
  })

  it('сведение: множители меньше 1, произведение — итог', () => {
    const t = track()
    t.start(at(1))
    const a = t.step(at(0.8))
    const b = t.step(at(0.5))
    expect(a).toBeLessThan(1)
    expect(a * b).toBeCloseTo(0.5, 12)
  })

  it('повторный gestureend (WebKit#233137) — 1', () => {
    const t = track()
    t.start(at(1))
    expect(t.end(at(2))).toBe(2)
    expect(t.active).toBe(false)
    expect(t.end(at(2))).toBe(1)
  })

  it('scale 0, NaN, ∞ и шаг без gesturestart — 1, прежний scale не теряется', () => {
    const t = track()
    expect(t.step(at(2))).toBe(1) // жеста нет
    t.start(at(1))
    for (const bad of [0, -1, NaN, Infinity]) expect(t.step(at(bad))).toBe(1)
    expect(t.step(at(1.5))).toBe(1.5)
  })

  it('мусорный scale в gesturestart — база 1', () => {
    const t = track()
    t.start(at(NaN))
    expect(t.active).toBe(true)
    expect(t.step(at(1.5))).toBe(1.5)
  })

  it('один шаг зажат в [0,5; 2]', () => {
    const t = track()
    t.start(at(1))
    expect(t.step(at(9))).toBe(2)
    expect(t.step(at(0.01))).toBe(0.5)
  })

  it('cancel сбрасывает жест', () => {
    const t = track()
    t.start(at(1))
    t.cancel()
    expect(t.active).toBe(false)
    expect(t.step(at(2))).toBe(1)
  })
})

describe('pinching', () => {
  it('true между gesturestart и gestureend, по двум жестам — пока идёт хоть один', () => {
    const a = track()
    const b = track()
    expect(pinching()).toBe(false)
    a.start(at(1))
    b.start(at(1))
    expect(pinching()).toBe(true)
    a.end(at(1))
    expect(pinching()).toBe(true)
    b.end(at(1))
    expect(pinching()).toBe(false)
  })

  it('потерянный gestureend не глушит колесо навсегда', () => {
    vi.useFakeTimers()
    vi.setSystemTime(0)
    vi.spyOn(performance, 'now').mockImplementation(() => Date.now())
    const t = track()
    t.start(at(1))
    expect(pinching()).toBe(true)
    vi.setSystemTime(5000)
    expect(pinching()).toBe(false)
    // запись снята, но жест не сброшен: следующее событие вернёт его в реестр
    expect(t.active).toBe(true)
    expect(t.step(at(1.5))).toBe(1.5)
    expect(pinching()).toBe(true)
    vi.restoreAllMocks()
  })
})
