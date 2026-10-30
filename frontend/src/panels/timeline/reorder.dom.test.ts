// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render } from '@testing-library/svelte'
import { flushSync, tick } from 'svelte'

// Живая перестановка в ленте (096): переходы состояния жеста. Геометрию happy-dom не считает — слоты групп
// и прямоугольник ленты подставлены; как это выглядит (тень, плавность) — глазами на стенде.
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
const view = (rev: number, ids: string[]) => ({ ...store.view, rev, spreads: ids.map(sp), settings: {} }) as never

beforeEach(async () => {
  await new Promise((r) => setTimeout(r, 0)) // гашение щелчка после прежнего броска
  naming.stop()
  store.view = view(1, ['a', 'b', 'c'])
  store.go(0)
  move.mockImplementation(async (id: string, at: number) => {
    const ids = store.view.spreads.map((s) => s.id).filter((x) => x !== id)
    ids.splice(at, 0, id)
    return view(store.view.rev + 1, ids)
  })
})
afterEach(() => {
  drag.cancel()
  cleanup()
  move.mockReset()
})

// Лента на экране: x 0…1000, y 500…600; группы по 100 px с чертой 14 — x 0, 114, 228 (середины 50, 164, 278).
async function mount() {
  const r = render(Timeline)
  await tick()
  await tick()
  const box = r.container.querySelector<HTMLElement>('.timeline')!
  box.getBoundingClientRect = () => ({ left: 0, top: 500, right: 1000, bottom: 600, width: 1000, height: 100, x: 0, y: 500 }) as DOMRect
  groups(r.container).forEach((g, k) => {
    Object.defineProperty(g, 'offsetLeft', { value: k * 114 })
    Object.defineProperty(g, 'offsetTop', { value: 8 })
    Object.defineProperty(g, 'offsetWidth', { value: 100 })
    Object.defineProperty(g, 'offsetHeight', { value: 80 })
  })
  return r
}
const groups = (c: HTMLElement) => Array.from(c.querySelectorAll<HTMLElement>('[data-drop="mini"]'))
const strip = (c: HTMLElement) => c.querySelector('.timeline')!
const shifts = (c: HTMLElement) => groups(c).map((g) => g.style.transform)
const Y = 540

const ptr = (type: string, x: number, y: number) => {
  window.dispatchEvent(new MouseEvent(type, { clientX: x, clientY: y, buttons: type === 'pointerup' ? 0 : 1 }))
  flushSync()
}
// Нажать на подложку группы k и сдвинуть за порог.
async function grab(c: HTMLElement, k: number, x: number) {
  await fireEvent.pointerDown(groups(c)[k], { button: 0, clientX: x, clientY: Y })
  ptr('pointermove', x + 10, Y)
}

describe('живая перестановка в ленте (096)', () => {
  it('своё место — соседи стоят, ничего не подсвечено; отпускание — ничего', async () => {
    const { container } = await mount()
    await grab(container, 0, 50)
    expect(strip(container).classList.contains('reordering')).toBe(true)
    expect(groups(container)[0].classList.contains('src')).toBe(true)
    expect(shifts(container).slice(1)).toEqual(['', ''])
    expect(container.querySelector('.over')).toBeNull()
    ptr('pointerup', 60, Y)
    expect(move).not.toHaveBeenCalled()
    expect(shifts(container)).toEqual(['', '', ''])
  })

  it('соседи расступаются; отпускание — одна правка, перенесённый — текущий', async () => {
    const { container } = await mount()
    store.go(2) // текущий — c
    await grab(container, 0, 50)
    ptr('pointermove', 290, Y) // за серединой последней — в конец
    expect(shifts(container)).toEqual(['translate(240px, 0px)', 'translate(-114px, 0px)', 'translate(-114px, 0px)'])
    expect(container.querySelector('.join.over, .lead.over, .addcol.gap')).toBeNull() // черты и «+» не цели
    ptr('pointerup', 290, Y)
    expect(drag.clickAllowed()).toBe(false) // щелчок сразу после броска не переходит
    expect(strip(container).classList.contains('landing')).toBe(true)
    await vi.waitFor(() => expect(move).toHaveBeenCalledExactlyOnceWith('a', 2))
    await vi.waitFor(() => expect(store.currentId).toBe('a'))
    expect(store.view.spreads.map((s) => s.id)).toEqual(['b', 'c', 'a'])
    await vi.waitFor(() => expect(shifts(container)).toEqual(['', '', '']))
  })

  it('Esc — всё на местах, правки нет', async () => {
    const { container } = await mount()
    await grab(container, 0, 50)
    ptr('pointermove', 290, Y)
    window.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }))
    flushSync()
    expect(drag.active).toBe(false)
    expect(shifts(container)).toEqual(['', '', ''])
    ptr('pointerup', 290, Y)
    expect(move).not.toHaveBeenCalled()
  })

  it('ушёл за ленту дальше 60 px — назад; вернулся — продолжается; отпускание там — ничего', async () => {
    const { container } = await mount()
    await grab(container, 0, 50)
    ptr('pointermove', 290, Y)
    ptr('pointermove', 290, 400) // 100 px выше ленты — на холст
    expect(shifts(container)).toEqual(['', '', ''])
    expect(groups(container)[0].classList.contains('src')).toBe(false)
    ptr('pointermove', 290, Y)
    expect(shifts(container)[2]).toBe('translate(-114px, 0px)')
    ptr('pointermove', 290, 400)
    ptr('pointerup', 290, 400)
    expect(move).not.toHaveBeenCalled()
    expect(shifts(container)).toEqual(['', '', ''])
  })

  it('миниатюра фото тащит фото, а не разворот', async () => {
    const { container } = await mount()
    document.elementsFromPoint = () => [] // цель фото — по DOM; здесь не нужна
    const chip = container.querySelector<HTMLElement>('[data-drop="chip"]')!
    await fireEvent.pointerDown(chip, { button: 0, clientX: 50, clientY: Y })
    ptr('pointermove', 70, Y)
    expect(drag.payload?.kind).toBe('photos')
    expect(strip(container).classList.contains('reordering')).toBe(false)
  })
})
