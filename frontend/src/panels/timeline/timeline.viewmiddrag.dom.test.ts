// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render } from '@testing-library/svelte'
import { flushSync, tick } from 'svelte'

// 149 Т5 шаг 1, лента: агент прислал `view`, пока человек тащит разворот (живая перестановка 096). Перенос не рвётся,
// ничего не залипает (класс reordering, смещения, призрак), правка либо идёт по актуальному порядку, либо отменяется.
// Геометрию happy-dom не считает — слоты подставлены, как в reorder.dom.test.ts.
const move = vi.hoisted(() => vi.fn())
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', () => ({ MoveSpread: move }))

class SeenIO {
  constructor(private cb: IntersectionObserverCallback) {}
  observe(target: Element) {
    this.cb([{ target, isIntersecting: true } as IntersectionObserverEntry], this as never)
  }
  unobserve() {}
  disconnect() {}
}
vi.stubGlobal('IntersectionObserver', SeenIO)

const { default: Timeline } = await import('./Timeline.svelte')
const { store } = await import('../../base/core/store.svelte')
const { drag } = await import('../../base/drag/drag.svelte')
const { naming } = await import('./naming.svelte')

const rect = { x: 0, y: 0, w: 100, h: 100 }
const sp = (id: string) => ({ id, name: '', photos: [`${id}1`], rects: [rect], w: 400, h: 200, kind: '' })
const view = (rev: number, ids: string[]) => ({ ...store.view, seq: 0, rev, spreads: ids.map(sp), settings: {} }) as never
const Y = 540

beforeEach(async () => {
  await new Promise((r) => setTimeout(r, 0)) // гашение щелчка после прежнего броска
  naming.stop()
  store.toast = null
  store.view = view(1, ['a', 'b', 'c'])
  store.go(0)
})
afterEach(() => {
  drag.cancel()
  cleanup()
  move.mockReset()
})

const groups = (c: HTMLElement) => Array.from(c.querySelectorAll<HTMLElement>('[data-drop="mini"]'))
const strip = (c: HTMLElement) => c.querySelector('.timeline')!
const shifts = (c: HTMLElement) => groups(c).map((g) => g.style.transform)
// Лента x 0…1000, y 500…600; группы по 100 px с чертой 14 (середины 50, 164, 278): слоты первых n групп.
function place(c: HTMLElement) {
  strip(c).getBoundingClientRect = () =>
    ({ left: 0, top: 500, right: 1000, bottom: 600, width: 1000, height: 100, x: 0, y: 500 }) as DOMRect
  groups(c).forEach((g, k) => {
    Object.defineProperty(g, 'offsetLeft', { configurable: true, value: k * 114 })
    Object.defineProperty(g, 'offsetTop', { configurable: true, value: 8 })
    Object.defineProperty(g, 'offsetWidth', { configurable: true, value: 100 })
    Object.defineProperty(g, 'offsetHeight', { configurable: true, value: 80 })
  })
}
async function mount() {
  const r = render(Timeline)
  await tick()
  await tick()
  place(r.container)
  return r
}
const ptr = (type: string, x: number, y = Y) => {
  window.dispatchEvent(new MouseEvent(type, { clientX: x, clientY: y, buttons: type === 'pointerup' ? 0 : 1 }))
  flushSync()
}
async function grab(c: HTMLElement, k: number, x: number) {
  await fireEvent.pointerDown(groups(c)[k], { button: 0, clientX: x, clientY: Y })
  ptr('pointermove', x + 10)
}
// Всё осело: ни перестановки, ни смещений, ни перехода приземления.
const calm = (c: HTMLElement) => {
  expect(drag.active).toBe(false)
  expect(strip(c).classList.contains('reordering')).toBe(false)
  expect(strip(c).classList.contains('landing')).toBe(false)
  expect(shifts(c).every((s) => s === '')).toBe(true)
}

describe('view посреди переноса разворота в ленте (149 Т5)', () => {
  it('тот же порядок, другой rev: перенос жив, отпускание — одна правка по актуальному порядку', async () => {
    const { container } = await mount()
    await grab(container, 0, 50)
    ptr('pointermove', 290) // за серединой последней — в конец
    store.apply(view(2, ['a', 'b', 'c']))
    await tick()
    expect(drag.active).toBe(true)
    expect(strip(container).classList.contains('reordering')).toBe(true)
    move.mockImplementation(async () => view(3, ['b', 'c', 'a']))
    ptr('pointerup', 290)
    await vi.waitFor(() => expect(move).toHaveBeenCalledExactlyOnceWith('a', 2))
    await vi.waitFor(() => expect(store.view.rev).toBe(3))
    expect(store.view.spreads.map((s) => s.id)).toEqual(['b', 'c', 'a'])
    await vi.waitFor(() => calm(container))
    expect(store.toast).toBeNull()
  })

  it('агент удалил поднятый разворот: отпускание — ошибка Go честно показана, лента оседает, разворотов не двоится', async () => {
    const { container } = await mount()
    await grab(container, 0, 50)
    ptr('pointermove', 290)
    store.apply(view(2, ['b', 'c']))
    await tick()
    expect(drag.active).toBe(true)
    move.mockRejectedValue(new Error('разворот не найден'))
    ptr('pointerup', 290)
    await vi.waitFor(() => expect(store.toast?.kind).toBe('error'))
    await vi.waitFor(() => calm(container))
    expect(store.view.spreads.map((s) => s.id)).toEqual(['b', 'c'])
  })

  it('агент удалил разворот перед поднятым: перенос не падает, после отпускания лента оседает', async () => {
    const { container } = await mount()
    await grab(container, 2, 278) // тащим c
    ptr('pointermove', 60)
    store.apply(view(2, ['b', 'c'])) // a исчез: c стал вторым
    await tick()
    place(container)
    expect(drag.active).toBe(true)
    move.mockImplementation(async (id: string, at: number) => {
      const ids = store.view.spreads.map((s) => s.id).filter((x) => x !== id)
      ids.splice(at, 0, id)
      return view(3, ids)
    })
    ptr('pointerup', 60)
    await vi.waitFor(() => calm(container))
    // как бы ни решили, разворотов ровно два и каждый один раз
    expect([...store.view.spreads.map((s) => s.id)].sort()).toEqual(['b', 'c'])
    expect(store.toast).toBeNull()
  })

  it('Esc после view: всё на местах, Go не зовётся', async () => {
    const { container } = await mount()
    await grab(container, 0, 50)
    ptr('pointermove', 290)
    store.apply(view(2, ['a', 'b', 'c', 'd']))
    await tick()
    window.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }))
    flushSync()
    await vi.waitFor(() => calm(container)) // возврат на места — 150 мс перехода
    ptr('pointerup', 290)
    expect(move).not.toHaveBeenCalled()
  })
})
