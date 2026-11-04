// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/svelte'
import { flushSync } from 'svelte'

// Накладка рамочки и окно цвета (109): наведение на цвет подменяет i= в адресе Go у всех выделенных, после паузы.
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', async (orig) => {
  const all = await orig<Record<string, unknown>>()
  return Object.fromEntries(Object.keys(all).map((k) => [k, vi.fn(async () => undefined)]))
})

const { default: BorderOverlay } = await import('./BorderOverlay.svelte')
const { colorLive, INK_DELAY } = await import('../../base/color/colorlive.svelte')
const { store } = await import('../../base/core/store.svelte')
const { placesel } = await import('../places/placesel.svelte')
const { default: InspectorBorder } = await import('./InspectorBorder.svelte')

const border = (url: string) => ({
  kind: 'line',
  photo: '',
  lib: '',
  preset: '',
  name: 'Линия',
  inks: ['#ffffff'],
  width: 1.5,
  scale: 0,
  ins: [1.5, 1.5, 1.5, 1.5],
  url,
  maskUrl: '',
  pic: false,
  open: false,
  known: true,
})
const URL = '/media/gen/frame?k=line&i=ffffff&bw=1.5&w=60&h=80&v=3'

beforeEach(() => vi.useFakeTimers())
afterEach(() => {
  cleanup()
  colorLive.clear()
  vi.useRealTimers()
})

const src = (c: HTMLElement) => c.querySelector('img')?.getAttribute('src') ?? ''

describe('BorderOverlay: показ цвета рамочки', () => {
  it('наведение на цвет рамочки — inkUrl у всех выделенных, у чужих прежний, rev прежний', () => {
    const rev = store.view.rev
    const mine = render(BorderOverlay, { border: border(URL), px: 256, refs: ['p1', 'a'] })
    const mate = render(BorderOverlay, { border: border(URL), px: 256, refs: ['p2', 'b'] })
    const other = render(BorderOverlay, { border: border(URL), px: 256, refs: ['p3', 'c'] })
    expect(src(mine.container)).toContain('i=ffffff')
    // Выделены p1 (по месту) и b (по фото): ключи — как в InspectorBorder (`bd:<ref>`).
    colorLive.set(['bd:p1', 'bd:b'], '#D8A9A1', 0)
    flushSync()
    expect(src(mine.container)).toContain('i=ffffff') // пауза ещё не прошла
    vi.advanceTimersByTime(INK_DELAY)
    flushSync()
    expect(src(mine.container)).toContain('i=d8a9a1')
    expect(src(mate.container)).toContain('i=d8a9a1')
    expect(src(other.container)).toContain('i=ffffff')
    expect(src(mine.container)).toContain('bw=1.5&w=60&h=80&v=3&px=256')
    expect(store.view.rev).toBe(rev)
    colorLive.clear()
    flushSync()
    expect(src(mine.container)).toContain('i=ffffff')
  })
})

describe('InspectorBorder: показ цвета', () => {
  it('наведение на цвет в карточке группы — ключи всех выделенных, краска 0; уход снимает', async () => {
    vi.useRealTimers()
    const place = (id: string, photo: string) => ({
      id,
      photo,
      fit: '',
      spine: false,
      z: 0,
      x: 0,
      y: 0,
      w: 50,
      h: 50,
      rot: 0,
      border: border(URL),
    })
    store.view = {
      ...store.view,
      rev: 7,
      spreads: [
        {
          id: 's1',
          grid: true,
          photos: ['a', 'b'],
          rects: [],
          frames: [],
          texts: [],
          captions: [],
          places: [place('p1', 'a'), place('p2', 'b')],
        },
      ],
    } as never
    store.go(0)
    placesel.select(['p1', 'p2'])
    render(InspectorBorder)
    await fireEvent.click(screen.getByRole('button', { name: 'Цвет рамочки 1' }))
    flushSync()
    await fireEvent.pointerEnter(screen.getAllByRole('button', { name: /#4a4643/ })[0])
    expect(colorLive.color('bd:p1', '')).toBe('#4a4643')
    expect(colorLive.color('bd:p2', '')).toBe('#4a4643')
    expect(colorLive.color('bd:p3', '')).toBe('')
    expect(store.view.rev).toBe(7)
    await fireEvent.pointerLeave(screen.getAllByRole('button', { name: /#4a4643/ })[0])
    expect(colorLive.color('bd:p1', '')).toBe('')
  })
})
