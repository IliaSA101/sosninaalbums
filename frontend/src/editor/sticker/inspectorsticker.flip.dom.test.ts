// @vitest-environment happy-dom
import { afterEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/svelte'

// «Зеркало» стикера на общем Seg: щелчок — FlipSticker по своей оси; нажато то, что включено у стикера.
const flip = vi.hoisted(() => vi.fn(async () => undefined))
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', async (orig) => {
  const all = await orig<Record<string, unknown>>()
  return {
    ...Object.fromEntries(Object.keys(all).map((k) => [k, vi.fn(async () => undefined)])),
    MyColors: vi.fn(async () => []),
    FlipSticker: flip,
  }
})

const { default: InspectorSticker } = await import('./InspectorSticker.svelte')
const { store } = await import('../../base/core/store.svelte')
const { placesel } = await import('../places/placesel.svelte')

const stk = (over: object) => ({
  id: 'k1',
  x: 10,
  y: 10,
  w: 40,
  h: 40,
  rot: 0,
  flipH: false,
  flipV: false,
  fade: 0,
  z: 0,
  photo: '',
  name: 'Сердце',
  inks: 1,
  art: { kind: 'heart', inks: ['#c8201e'] },
  url: '/media/gen/art?k=heart&i=c8201e&v=3',
  gen: true,
  stretch: false,
  aspect: 1,
  dpi: 0,
  known: true,
  ...over,
})
function show(over: object) {
  store.view = {
    ...store.view,
    rev: 3,
    settings: { pageW: 300, pageH: 300 },
    spreads: [{ id: 's1', w: 200, h: 100, grid: false, photos: [], places: [], frames: [], rects: [], stickers: [stk(over)] }],
  } as never
  store.go(0)
  placesel.pickSticker('k1', false)
  render(InspectorSticker)
}

afterEach(() => {
  cleanup()
  flip.mockClear()
})

describe('InspectorSticker: зеркало', () => {
  it('щелчок по ↔ и ↕ зовёт FlipSticker с осью h / v', async () => {
    show({})
    await fireEvent.click(screen.getByRole('button', { name: /↔/ }))
    await fireEvent.click(screen.getByRole('button', { name: /↕/ }))
    expect(flip.mock.calls).toEqual([
      ['s1', 'k1', 'h'],
      ['s1', 'k1', 'v'],
    ])
  })

  it('нажаты оси, включённые у стикера; подсказки на месте', () => {
    show({ flipV: true })
    expect(screen.getByRole('button', { name: /↔/ }).getAttribute('aria-pressed')).toBe('false')
    const v = screen.getByRole('button', { name: /↕/ })
    expect(v.getAttribute('aria-pressed')).toBe('true')
    expect(v.getAttribute('title')).toBe('Отразить по вертикали')
  })
})
