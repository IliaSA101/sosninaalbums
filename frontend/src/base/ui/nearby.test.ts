import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { Nearby } from './nearby.svelte'

// Подставной IntersectionObserver: запоминает параметры и узлы, срабатывает руками.
type Entry = { target: Element; isIntersecting: boolean }
class FakeIO {
  static made: FakeIO[] = []
  seen = new Set<Element>()
  constructor(
    readonly cb: (es: Entry[]) => void,
    readonly opts: IntersectionObserverInit,
  ) {
    FakeIO.made.push(this)
  }
  observe(n: Element) {
    this.seen.add(n)
  }
  unobserve(n: Element) {
    this.seen.delete(n)
  }
  fire(target: Element, isIntersecting: boolean) {
    this.cb([{ target, isIntersecting }])
  }
}
const el = () => ({}) as Element

describe('Nearby', () => {
  beforeEach(() => {
    FakeIO.made = []
    vi.stubGlobal('IntersectionObserver', FakeIO)
  })
  afterEach(() => vi.unstubAllGlobals())

  it('видимый id в has, ушёл — убран; один наблюдатель на список', () => {
    const n = new Nearby(null, '400px 0px')
    const a = el()
    const b = el()
    n.watch(a, 'A')
    n.watch(b, 'B')
    expect(FakeIO.made).toHaveLength(1)
    expect(FakeIO.made[0].opts.rootMargin).toBe('400px 0px')
    expect(n.has('A')).toBe(false)
    FakeIO.made[0].fire(a, true)
    expect(n.has('A')).toBe(true)
    expect(n.has('B')).toBe(false)
    FakeIO.made[0].fire(a, false)
    expect(n.has('A')).toBe(false)
  })

  it('после снятия колбэк молчит и id забыт', () => {
    const n = new Nearby(null, '0px')
    const a = el()
    const off = n.watch(a, 'A')
    FakeIO.made[0].fire(a, true)
    off()
    expect(n.has('A')).toBe(false)
    expect(FakeIO.made[0].seen.has(a)).toBe(false)
    FakeIO.made[0].fire(a, true)
    expect(n.has('A')).toBe(false)
  })

  it('root берётся при первом watch, не в конструкторе', () => {
    const box = el()
    const root = vi.fn(() => box)
    const n = new Nearby(root, '0px 1500px')
    expect(root).not.toHaveBeenCalled()
    n.watch(el(), 'A')
    n.watch(el(), 'B')
    expect(root).toHaveBeenCalledTimes(1)
    expect(FakeIO.made[0].opts.root).toBe(box)
  })

  it('корень ещё не привязан — наблюдение начинается после монтирования', async () => {
    let box: Element | null = null
    const n = new Nearby(() => box, '0px')
    const a = el()
    n.watch(a, 'A')
    expect(FakeIO.made).toHaveLength(0)
    box = el()
    await Promise.resolve()
    expect(FakeIO.made).toHaveLength(1)
    expect(FakeIO.made[0].seen.has(a)).toBe(true)
  })

  it('снятие до появления корня — наблюдать не начинаем', async () => {
    let box: Element | null = null
    const n = new Nearby(() => box, '0px')
    const off = n.watch(el(), 'A')
    off()
    box = el()
    await Promise.resolve()
    await Promise.resolve()
    expect(FakeIO.made).toHaveLength(0)
  })

  it('корень так и не привязан — сдаёмся и показываем всё, цикл конечен', async () => {
    const n = new Nearby(() => null, '0px')
    n.watch(el(), 'A')
    for (let i = 0; i < 10; i++) await Promise.resolve()
    expect(n.has('A')).toBe(true)
    expect(n.has('Z')).toBe(true)
  })

  it('повторный watch того же узла и id не снимает has', () => {
    const n = new Nearby(null, '0px')
    const a = el()
    n.watch(a, 'A')
    FakeIO.made[0].fire(a, true)
    n.watch(a, 'A')
    expect(n.has('A')).toBe(true)
    expect(FakeIO.made).toHaveLength(1)
  })

  it('корень-функция: берётся её результат (скроллер, а не окно)', () => {
    const sc = el()
    const n = new Nearby(() => sc, '400px 0px')
    n.watch(el(), 'A')
    expect(FakeIO.made[0].opts.root).toBe(sc)
    expect(FakeIO.made[0].opts.rootMargin).toBe('400px 0px')
  })

  it('без IntersectionObserver рядом всё', () => {
    vi.unstubAllGlobals()
    vi.stubGlobal('IntersectionObserver', undefined)
    const n = new Nearby(null, '0px')
    n.watch(el(), 'A')()
    expect(n.has('любой')).toBe(true)
  })
})
