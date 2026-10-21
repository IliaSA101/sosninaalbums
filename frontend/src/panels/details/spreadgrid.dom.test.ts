// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render } from '@testing-library/svelte'
import { flushSync, tick } from 'svelte'

// «Все развороты» (096 сц. 9–13): тот же жест, что в ленте. Геометрия плиток подставлена (happy-dom её не считает).
const move = vi.hoisted(() => vi.fn())
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', () => ({ MoveSpread: move }))
vi.stubGlobal(
  'IntersectionObserver',
  class {
    observe() {}
    unobserve() {}
    disconnect() {}
  },
)

const { default: SpreadGrid } = await import('./SpreadGrid.svelte')
const { store } = await import('../../base/core/store.svelte')
const { drag } = await import('../../base/drag/drag.svelte')

const sp = (id: string) => ({ id, name: '', photos: [], rects: [], w: 400, h: 200, kind: '' })
const view = (rev: number, ids: string[]) => ({ ...store.view, rev, spreads: ids.map(sp), settings: {} }) as never

beforeEach(async () => {
  await new Promise((r) => setTimeout(r, 0))
  store.view = view(1, ['a', 'b', 'c', 'd', 'e'])
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

// Сетка 3 в ряд, плитка 100×80, зазор 10: ряды y 0 и 90; на экране — с (0, 0), видна целиком.
async function mount() {
  const r = render(SpreadGrid)
  await tick()
  const grid = r.container.querySelector<HTMLElement>('.grid')!
  grid.getBoundingClientRect = () => ({ left: 0, top: 0, right: 320, bottom: 170, width: 320, height: 170, x: 0, y: 0 }) as DOMRect
  tiles(r.container).forEach((t, k) => {
    Object.defineProperty(t, 'offsetLeft', { value: (k % 3) * 110 })
    Object.defineProperty(t, 'offsetTop', { value: Math.floor(k / 3) * 90 })
    Object.defineProperty(t, 'offsetWidth', { value: 100 })
    Object.defineProperty(t, 'offsetHeight', { value: 80 })
  })
  return r
}
const tiles = (c: HTMLElement) => Array.from(c.querySelectorAll<HTMLElement>('[data-tile]'))
const shifts = (c: HTMLElement) => tiles(c).map((t) => t.style.transform)
const ptr = (type: string, x: number, y: number) => {
  window.dispatchEvent(new MouseEvent(type, { clientX: x, clientY: y, buttons: type === 'pointerup' ? 0 : 1 }))
  flushSync()
}

describe('«Все развороты»: порядок перетаскиванием (096)', () => {
  it('щелчок без сдвига — переход', async () => {
    const { container } = await mount()
    await fireEvent.pointerDown(tiles(container)[3], { button: 0, clientX: 50, clientY: 130 })
    ptr('pointerup', 50, 130)
    await fireEvent.click(tiles(container)[3])
    expect(store.current).toBe(3)
    expect(move).not.toHaveBeenCalled()
  })

  it('плитка 1 → 4: сдвиг по рядам, одна правка, перенесённый — текущий, щелчок после броска не переходит', async () => {
    const { container } = await mount()
    await fireEvent.pointerDown(tiles(container)[1], { button: 0, clientX: 160, clientY: 40 })
    ptr('pointermove', 170, 40)
    ptr('pointermove', 170, 130) // второй ряд, правее середины плитки 4
    expect(shifts(container)).toEqual([
      '',
      'translate(10px, 90px)',
      'translate(-110px, 0px)',
      'translate(220px, -90px)', // крайняя в ряду — в конец предыдущего
      'translate(-110px, 0px)',
    ])
    ptr('pointerup', 170, 130)
    await fireEvent.click(tiles(container)[1])
    expect(store.current).not.toBe(1)
    await vi.waitFor(() => expect(move).toHaveBeenCalledExactlyOnceWith('b', 4))
    await vi.waitFor(() => expect(store.currentId).toBe('b'))
    expect(store.view.spreads.map((s) => s.id)).toEqual(['a', 'c', 'd', 'e', 'b'])
    await vi.waitFor(() => expect(shifts(container)).toEqual(['', '', '', '', '']))
  })

  it('за пределами сетки дальше 60 px (в ленту) — отмена', async () => {
    const { container } = await mount()
    await fireEvent.pointerDown(tiles(container)[0], { button: 0, clientX: 50, clientY: 40 })
    ptr('pointermove', 170, 130)
    expect(shifts(container)[1]).toBe('translate(-110px, 0px)')
    ptr('pointermove', 170, 400)
    expect(shifts(container)).toEqual(['', '', '', '', ''])
    ptr('pointerup', 170, 400)
    expect(move).not.toHaveBeenCalled()
  })
})
