// @vitest-environment happy-dom
import { afterEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/svelte'
import { flushSync } from 'svelte'

// Цвет текста в карточке и окно цвета (109): наведение — показ у выбранного блока, щелчок — одна SetTextStyle.
const setStyle = vi.hoisted(() => vi.fn(async () => undefined))
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', async (orig) => {
  const all = await orig<Record<string, unknown>>()
  return {
    ...Object.fromEntries(Object.keys(all).map((k) => [k, vi.fn(async () => undefined)])),
    MyColors: vi.fn(async () => []),
    SetTextStyle: setStyle,
  }
})

const { default: InspectorText } = await import('./InspectorText.svelte')
const { edit } = await import('./edit.svelte')
const { store } = await import('../../base/core/store.svelte')
const { colorLive } = await import('../../base/color/colorlive.svelte')

afterEach(() => {
  cleanup()
  colorLive.clear()
  setStyle.mockClear()
  edit.sel = null
})

describe('InspectorText: цвет текста', () => {
  it('наведение — показ у блока t1 (t2 не тронут); щелчок — один SetTextStyle', async () => {
    const style = { font: 'montserrat', size: 12, color: '#000000', align: 'left' }
    const text = (id: string) => ({ id, x: 0, y: 0, w: 50, h: 10, style, lines: [], z: 0, rot: 0 })
    store.view = {
      ...store.view,
      rev: 6,
      settings: { pageW: 300, pageH: 300 },
      spreads: [{ id: 's1', grid: false, photos: [], places: [], frames: [], rects: [], texts: [text('t1'), text('t2')], captions: [] }],
    } as never
    store.go(0)
    edit.sel = { kind: 'text', spread: 's1', id: 't1' }
    render(InspectorText)
    await fireEvent.click(screen.getByRole('button', { name: 'Цвет текста' }))
    flushSync()
    await fireEvent.pointerEnter(screen.getAllByRole('button', { name: /#4a4643/ })[0])
    expect(colorLive.color('text:t1', '')).toBe('#4a4643')
    expect(colorLive.color('text:t2', '')).toBe('')
    expect(setStyle).not.toHaveBeenCalled()
    await fireEvent.click(screen.getAllByRole('button', { name: /#4a4643/ })[0])
    expect(setStyle).toHaveBeenCalledExactlyOnceWith('t1', expect.objectContaining({ color: '#4a4643' }))
    await new Promise((r) => setTimeout(r, 0))
    expect(colorLive.cur).toBeNull()
  })
})
