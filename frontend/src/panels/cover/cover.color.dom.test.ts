// @vitest-environment happy-dom
import { afterEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/svelte'
import { flushSync } from 'svelte'

// Плашка корешка в карточке обложки и окно цвета (109): наведение — показ, щелчок — одна SetCoverSpineBg.
const setSpine = vi.hoisted(() => vi.fn(async () => undefined))
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', async (orig) => {
  const all = await orig<Record<string, unknown>>()
  return {
    ...Object.fromEntries(Object.keys(all).map((k) => [k, vi.fn(async () => undefined)])),
    CoverPresets: vi.fn(async () => []),
    MyColors: vi.fn(async () => []),
    SetCoverSpineBg: setSpine,
  }
})

const { default: InspectorCover } = await import('./InspectorCover.svelte')
const { store } = await import('../../base/core/store.svelte')
const { colorLive } = await import('../../base/color/colorlive.svelte')

afterEach(() => {
  cleanup()
  colorLive.clear()
  setSpine.mockClear()
})

describe('InspectorCover: плашка корешка', () => {
  it('наведение — показ «spine» без правки; щелчок — одна правка, показ снят после ответа', async () => {
    store.view = {
      ...store.view,
      rev: 2,
      settings: { pageW: 300, pageH: 300 },
      spreads: [],
      cover: {
        sheet: { id: 'cover', w: 600, h: 300 },
        spec: { preset: 'custom', wrap: 20, steps: [] },
        preset: 'custom',
        spine: 10,
        n: 5,
        over: false,
        layout: 'both',
        spineBg: '',
        spineText: '',
        zones: [],
        drop: [],
        scopes: [],
        safe: [],
        markup: null,
        pxW: 100,
        pxH: 100,
        fix: '',
      },
    } as never
    render(InspectorCover)
    await fireEvent.click(screen.getByRole('button', { name: 'Цвет плашки корешка' }))
    flushSync()
    await fireEvent.pointerEnter(screen.getAllByRole('button', { name: /#4a4643/ })[0])
    expect(colorLive.color('spine', '')).toBe('#4a4643')
    expect(setSpine).not.toHaveBeenCalled()
    await fireEvent.click(screen.getAllByRole('button', { name: /#4a4643/ })[0])
    expect(setSpine).toHaveBeenCalledExactlyOnceWith('#4a4643')
    await new Promise((r) => setTimeout(r, 0))
    expect(colorLive.cur).toBeNull()
  })
})
