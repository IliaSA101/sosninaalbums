// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/svelte'
import { flushSync } from 'svelte'

// 132-13: кнопка «Заполнить места» в полосе ленты — число «отсюда», видимость по пустым местам всего альбома.
const fills: string[] = []
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', () => ({
  Marks: async () => ({}),
  FillPlaces: async (_ids: string[], at: string) => {
    fills.push(at)
    return { view: {}, left: 0 }
  },
}))

const { default: LibraryBar } = await import('./LibraryBar.svelte')
const { store } = await import('../../base/core/store.svelte')
const { wire } = await import('../../shell/boot/wire')

wire()

const sp = (id: string, free = 0) => ({ id, photos: [], places: Array.from({ length: free }, () => ({})) })
const album = (extra: object) =>
  ({
    rev: 1,
    marksRev: 1,
    folders: ['D:/a'],
    spreads: [sp('s1'), sp('s2'), sp('s3', 1)],
    cover: { sheet: sp('cover') },
    emptyPlaces: 5,
    ...extra,
  }) as never
const btn = () => document.querySelector('[data-guide="fill-empty"]') as HTMLButtonElement | null

beforeEach(() => {
  fills.length = 0
  store.toast = null
  store.loaded = true
  store.photos = [{ id: 'a', path: 'D:/a/a.jpg', name: 'a.jpg', w: 100, h: 100 }] as never
})
afterEach(cleanup)

describe('кнопка «Заполнить места» в полосе ленты', () => {
  it('число — отсюда: на обложке coverFill, на развороте fillFrom[i]; подсказка с обложки другая', async () => {
    store.apply(album({ coverFill: 5, fillFrom: [3, 3, 1] }))
    render(LibraryBar)
    store.goCover()
    flushSync()
    expect(btn()!.textContent!.trim()).toBe('Заполнить места: 5')
    expect(btn()!.title).toContain('начиная с обложки: лицевая, задняя, затем развороты')
    store.go(2)
    flushSync()
    expect(btn()!.textContent!.trim()).toBe('Заполнить места: 1')
    expect(btn()!.title).toContain('начиная с этого разворота')
    expect(btn()!.getAttribute('aria-label')).toBe('Заполнить пустые места: 1')
  })

  it('отсюда 0, места есть: кнопка видна, щелчок — тост «где» и без вызова Go', async () => {
    store.apply(album({ coverFill: 1, fillFrom: [1, 0, 0], spreads: [sp('s1', 1), sp('s2'), sp('s3')] }))
    render(LibraryBar)
    store.go(2)
    flushSync()
    expect(btn()!.textContent!.trim()).toBe('Заполнить места: 0')
    await fireEvent.click(btn()!)
    expect(fills).toEqual([])
    expect(store.toast?.text).toBe('Пустые места — в развороте 1')
  })

  it('пустых мест в альбоме нет — кнопки нет; полей fillFrom нет (старый вид) — число 0, без падения', () => {
    store.apply(album({ emptyPlaces: 0, coverFill: 0, fillFrom: [0, 0, 0] }))
    render(LibraryBar)
    expect(btn()).toBeNull()
    store.apply(album({ rev: 2, fillFrom: undefined, coverFill: undefined }))
    flushSync()
    expect(btn()!.textContent!.trim()).toBe('Заполнить места: 0')
    expect(screen.getByRole('button', { name: 'Заполнить пустые места: 0' })).toBe(btn())
  })

  it('короткий fillFrom (индекс за концом) — 0, не падает', () => {
    store.apply(album({ coverFill: 1, fillFrom: [1] }))
    render(LibraryBar)
    store.go(2)
    flushSync()
    expect(btn()!.textContent!.trim()).toBe('Заполнить места: 0')
  })
})

// 132-14: ширина числа резервируется по разрядам emptyPlaces — листание «12» → «0» не двигает группы полосы.
describe('узкая полоса: состав групп и ширина кнопки не зависят от числа «отсюда»', () => {
  // Размеров в happy-dom нет: модель раскладки. Группа — 100 px, кнопка «Заполнить» — 17 знаков × 7 px + число;
  // число — max(min-width в ch, его знаки) × 8 px. Наблюдатель отдаёт размер при observe и по fire().
  const live = new Set<() => void>()
  class Obs {
    constructor(private cb: (e: unknown[]) => void) {}
    observe() {
      const f = () => this.cb([])
      live.add(f)
      f()
    }
    unobserve() {}
    disconnect() {}
  }
  let barW = 0
  const numW = (el: Element) => {
    const num = el.querySelector('.num') as HTMLElement | null
    if (!num) return (el.textContent ?? '').trim().split(': ')[1].length * 8
    const min = Number(/([\d.]+)ch/.exec(num.getAttribute('style') ?? '')?.[1] ?? 0)
    return Math.max(min, (num.textContent ?? '').length) * 8
  }
  const widthOf = (el: Element) => (el.querySelector('[data-guide="fill-empty"]') ? 17 * 7 + numW(el) : 100)
  const settle = () => {
    for (let i = 0; i < 3; i++) {
      live.forEach((f) => f())
      flushSync()
    }
  }
  const groups = () => [...document.querySelectorAll('[data-bar="library"] .g')]
  const snapshot = () => ({
    n: groups().length,
    fill: btn() ? Math.ceil(widthOf(btn()!.parentElement!)) : null,
    more: !!document.querySelector('[data-bar="library"] [aria-haspopup]'),
  })
  // Альбом: emptyPlaces 12; с обложки 12, с разворота 1 — 10, с разворота 3 — 0.
  const wide = () => album({ emptyPlaces: 12, coverFill: 12, fillFrom: [10, 10, 0], spreads: [sp('s1', 10), sp('s2'), sp('s3')] })

  beforeEach(() => {
    live.clear()
    vi.stubGlobal('ResizeObserver', Obs)
    vi.spyOn(HTMLElement.prototype, 'clientWidth', 'get').mockImplementation(() => barW)
    vi.spyOn(Element.prototype, 'getBoundingClientRect').mockImplementation(function (this: Element) {
      return { width: this.classList.contains('g') ? widthOf(this) : 0, height: 30 } as DOMRect
    })
  })
  afterEach(() => {
    vi.unstubAllGlobals()
    vi.restoreAllMocks()
  })

  // Прогон листания: обложка → разворот 1 → разворот 3 (число 12 / 10 / 0).
  function browse(width: number) {
    barW = width
    store.apply(wide())
    render(LibraryBar)
    store.goCover()
    settle()
    const seen = [snapshot()]
    for (const i of [0, 2]) {
      store.go(i)
      settle()
      seen.push(snapshot())
    }
    return seen
  }
  // Ширина полосы под N групп: поля 24 + растяжка 16 + группы + зазоры 8, из них кнопка шириной fill.
  const need = (n: number, fill: number) => 24 + 16 + (n - 1) * 100 + fill + 8 * (n - 1)

  it('полоса, где кнопка с одной цифрой влезла бы, а с двумя — нет: состав и ширина те же при листании', () => {
    barW = 3000
    store.apply(wide())
    render(LibraryBar)
    store.goCover()
    settle()
    const n = groups().length
    cleanup()
    const seen = browse(need(n, 17 * 7 + 8))
    expect(seen[1]).toEqual(seen[0])
    expect(seen[2]).toEqual(seen[0])
  })

  it('полоса впритык под резерв: кнопка в строке, ширина одна на всех числах', () => {
    barW = 3000
    store.apply(wide())
    render(LibraryBar)
    store.goCover()
    settle()
    const n = groups().length
    cleanup()
    const seen = browse(need(n, 17 * 7 + 16))
    expect(seen.map((x) => x.n)).toEqual([n, n, n])
    expect(seen.map((x) => x.fill)).toEqual([135, 135, 135])
  })
})
