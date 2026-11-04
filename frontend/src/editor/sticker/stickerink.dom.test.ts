// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/svelte'
import { flushSync } from 'svelte'

// Стикер-генератор и окно цвета (109): наведение на цвет подменяет i= в адресе Go после паузы, щелчок — одна правка.
const setInks = vi.hoisted(() => vi.fn(async () => undefined))
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', async (orig) => {
  const all = await orig<Record<string, unknown>>()
  return {
    ...Object.fromEntries(Object.keys(all).map((k) => [k, vi.fn(async () => undefined)])),
    MyColors: vi.fn(async () => []),
    SetStickerInks: setInks,
  }
})

const { default: StickerLayer } = await import('./StickerLayer.svelte')
const { default: InspectorSticker } = await import('./InspectorSticker.svelte')
const { colorLive, INK_DELAY } = await import('../../base/color/colorlive.svelte')
const { store } = await import('../../base/core/store.svelte')
const { placesel } = await import('../places/placesel.svelte')

const URL = '/media/gen/art?k=heart&i=c8201e&v=3'
const stk = (id: string) => ({
  id,
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
  url: URL,
  gen: true,
  stretch: false,
  aspect: 1,
  dpi: 0,
  known: true,
})
const sheet = () =>
  ({ id: 's1', w: 200, h: 100, grid: false, photos: [], places: [], frames: [], rects: [], stickers: [stk('k1'), stk('k2')] }) as never
const srcs = (c: HTMLElement) => Array.from(c.querySelectorAll('.stk img')).map((i) => i.getAttribute('src') ?? '')

beforeEach(() => vi.useFakeTimers())
afterEach(() => {
  cleanup()
  colorLive.clear()
  setInks.mockClear()
  vi.useRealTimers()
})

describe('StickerLayer: показ краски', () => {
  it('подменяется только у стикера с ключом, после паузы; без i= — как был', () => {
    const r = render(StickerLayer, { spread: sheet(), width: 400 })
    colorLive.set(['st:k1'], '#112233', 0)
    flushSync()
    expect(srcs(r.container)[0]).toContain('i=c8201e')
    vi.advanceTimersByTime(INK_DELAY)
    flushSync()
    const [a, b] = srcs(r.container)
    expect(a).toContain('i=112233')
    expect(b).toContain('i=c8201e')
    colorLive.clear()
    flushSync()
    expect(srcs(r.container)[0]).toContain('i=c8201e')
  })
})

describe('InspectorSticker: цвет стикера', () => {
  it('наведение — показ без правки; щелчок — одна SetStickerInks, показ держится до ответа', async () => {
    vi.useRealTimers()
    store.view = { ...store.view, rev: 3, settings: { pageW: 300, pageH: 300 }, spreads: [sheet()] } as never
    store.go(0)
    placesel.pickSticker('k1', false)
    render(InspectorSticker)
    await fireEvent.click(screen.getByRole('button', { name: 'Цвет стикера 1' }))
    flushSync()
    await fireEvent.pointerEnter(screen.getAllByRole('button', { name: /#4a4643/ })[0])
    expect(colorLive.color('st:k1', '')).toBe('#4a4643')
    expect(colorLive.color('st:k2', '')).toBe('')
    expect(setInks).not.toHaveBeenCalled()
    await fireEvent.click(screen.getAllByRole('button', { name: /#4a4643/ })[0])
    expect(setInks).toHaveBeenCalledExactlyOnceWith('s1', 'k1', ['#4a4643'], '')
    await new Promise((r) => setTimeout(r, 0))
    expect(colorLive.cur).toBeNull()
  })
})
