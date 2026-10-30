// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render } from '@testing-library/svelte'
import { flushSync } from 'svelte'

// Промежуток между листами на холсте (103 §3): Go подменён — NewSpread возвращает вид с новым разворотом.
const newSpread = vi.hoisted(() => vi.fn())
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', () => ({ NewSpread: newSpread }))

const { default: SpreadSlots } = await import('./SpreadSlots.svelte')
const { store } = await import('../../base/core/store.svelte')
const { drag } = await import('../../base/drag/drag.svelte')
const { gapKey } = await import('../../panels/timeline/cutkey.svelte')
const { row } = await import('./spreadrow.svelte')
const { pitch, slotCenters } = await import('./spreadrow')

const sp = (id: string) => ({ id, name: '', photos: [], rects: [], w: 400, h: 200, kind: '' })
const setView = (ids: string[], rev: number, cover = false) => {
  store.view = { ...store.view, rev, spreads: ids.map(sp), settings: {}, cover: cover ? { sheet: sp('cover') } : undefined } as never
}
// Один px на мм: листы 400×200, зазор 24 между краями.
const base = { w: 400, h: 200, mid: 200 }
const fit = { left: 500, top: 50, width: 400, height: 200 }
// n — число листов ряда; row.geo сообщает ряду размеры, как это делает Stage (иначе max = 0 и рисуется один слот).
const props = (n: number, zoomed = false) => {
  const centers = slotCenters(Array(n).fill(400))
  row.geo(pitch(400), n, zoomed, centers)
  return { ...common(n, zoomed), centers }
}
const common = (n: number, zoomed: boolean) => ({
  settings: store.view.settings,
  k: 1,
  base,
  spreadW: 400,
  w: 1400,
  fit,
  place: fit,
  zoomed,
  onhover: () => {},
})
const gaps = (c: HTMLElement) => Array.from(c.querySelectorAll<HTMLButtonElement>('button.gap'))
const ctrl = (type: 'keydown' | 'keyup', held: boolean) => {
  window.dispatchEvent(new KeyboardEvent(type, { key: 'Control', ctrlKey: held }))
  flushSync()
}

beforeEach(() => {
  gapKey.held = false
  row.moving = false
  setView(['a', 'b', 'c'], 1)
  store.currentId = 'a'
  store.current = 0
  store.onCover = false
  newSpread.mockImplementation(async (at: number) => {
    const ids = store.view.spreads.map((x) => x.id)
    ids.splice(at, 0, 'n')
    setView(ids, store.view.rev + 1)
    return store.view
  })
})
afterEach(() => {
  cleanup()
  newSpread.mockReset()
  drag.active = false
})

describe('промежуток между листами: Ctrl+щелчок вставляет разворот (103 §3)', () => {
  it('промежутки стоят между соседними листами; перед слотом «новый» зоны нет', () => {
    const { container } = render(SpreadSlots, props(3))
    expect(gaps(container).map((g) => g.dataset.at)).toEqual(['1', '2'])
  })

  it('без Ctrl зона не ловит мышь, щелчок ничего не вставляет', async () => {
    const { container } = render(SpreadSlots, props(3))
    const g = gaps(container)[0]
    expect(g.classList.contains('armed')).toBe(false)
    expect(g.getAttribute('aria-hidden')).toBe('true')
    await fireEvent.click(g, { ctrlKey: false })
    expect(newSpread).not.toHaveBeenCalled()
  })

  it('Ctrl зажат — зона вооружена; отпущен — снова нет', () => {
    const { container } = render(SpreadSlots, props(3))
    ctrl('keydown', true)
    expect(gaps(container)[0].classList.contains('armed')).toBe(true)
    ctrl('keyup', false)
    expect(gaps(container)[0].classList.contains('armed')).toBe(false)
  })

  it('Ctrl+щелчок — NewSpread(at, []) между этими листами', async () => {
    const { container } = render(SpreadSlots, props(3))
    ctrl('keydown', true)
    await fireEvent.click(gaps(container)[1], { ctrlKey: true })
    expect(newSpread).toHaveBeenCalledTimes(1)
    expect(newSpread).toHaveBeenCalledWith(2, [])
  })

  it('Ctrl зажат, но щелчок без Ctrl (отпустили на лету) — мимо', async () => {
    const { container } = render(SpreadSlots, props(3))
    ctrl('keydown', true)
    await fireEvent.click(gaps(container)[0], { ctrlKey: false })
    expect(newSpread).not.toHaveBeenCalled()
  })

  it('после обложки — at 0 (в начало)', async () => {
    setView(['a', 'b'], 1, true)
    const { container } = render(SpreadSlots, props(3))
    expect(gaps(container).map((g) => g.dataset.at)).toEqual(['0', '1'])
    ctrl('keydown', true)
    await fireEvent.click(gaps(container)[0], { ctrlKey: true })
    expect(newSpread).toHaveBeenCalledWith(0, [])
  })

  it('в зуме зон нет', () => {
    const { container } = render(SpreadSlots, props(3, true))
    expect(gaps(container)).toHaveLength(0)
  })

  it('пока ряд едет — зон нет', () => {
    row.moving = true
    const { container } = render(SpreadSlots, props(3))
    expect(gaps(container)).toHaveLength(0)
  })

  it('один разворот без обложки — пары нет, зон нет', () => {
    setView(['a'], 1)
    const { container } = render(SpreadSlots, props(1))
    expect(gaps(container)).toHaveLength(0)
  })

  it('перенос в ходу — зона молчит, даже с Ctrl', () => {
    const { container } = render(SpreadSlots, props(3))
    ctrl('keydown', true)
    drag.active = true
    flushSync()
    expect(gaps(container)[0].classList.contains('armed')).toBe(false)
  })

  it('пустой альбом — слушателей Ctrl нет, зон нет', () => {
    setView([], 1)
    const { container } = render(SpreadSlots, props(0))
    expect(gaps(container)).toHaveLength(0)
    ctrl('keydown', true)
    expect(gapKey.held).toBe(false)
  })
})
