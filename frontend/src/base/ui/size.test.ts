import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

// Подставной ResizeObserver: запоминает наблюдаемые узлы, колбэк вызывается руками.
class FakeRO {
  static made: FakeRO[] = []
  seen = new Set<Element>()
  constructor(readonly cb: (es: { target: Element }[]) => void) {
    FakeRO.made.push(this)
  }
  observe(n: Element) {
    this.seen.add(n)
  }
  unobserve(n: Element) {
    this.seen.delete(n)
  }
  fire(...nodes: Element[]) {
    this.cb(nodes.map((target) => ({ target })))
  }
}

const node = (w: number, h: number) => ({ clientWidth: w, clientHeight: h }) as unknown as Element

describe('watchSize', () => {
  beforeEach(() => {
    FakeRO.made = []
    vi.resetModules()
    vi.stubGlobal('ResizeObserver', FakeRO)
  })
  afterEach(() => vi.unstubAllGlobals())

  it('один наблюдатель на много узлов, колбэк получает clientWidth/Height', async () => {
    const { watchSize } = await import('./size')
    const a = node(100, 50)
    const b = node(30, 20)
    const ga = vi.fn()
    const gb = vi.fn()
    watchSize(a, ga)
    watchSize(b, gb)
    expect(FakeRO.made).toHaveLength(1)
    FakeRO.made[0].fire(a, b)
    expect(ga).toHaveBeenCalledWith(100, 50)
    expect(gb).toHaveBeenCalledWith(30, 20)
  })

  it('снятие — unobserve, колбэк молчит', async () => {
    const { watchSize } = await import('./size')
    const a = node(10, 10)
    const f = vi.fn()
    const off = watchSize(a, f)
    expect(FakeRO.made[0].seen.has(a)).toBe(true)
    off()
    expect(FakeRO.made[0].seen.has(a)).toBe(false)
    FakeRO.made[0].fire(a)
    expect(f).not.toHaveBeenCalled()
  })

  it('повторное наблюдение того же узла заменяет колбэк', async () => {
    const { watchSize } = await import('./size')
    const a = node(5, 6)
    const f1 = vi.fn()
    const f2 = vi.fn()
    watchSize(a, f1)
    watchSize(a, f2)
    FakeRO.made[0].fire(a)
    expect(f1).not.toHaveBeenCalled()
    expect(f2).toHaveBeenCalledWith(5, 6)
  })
})
