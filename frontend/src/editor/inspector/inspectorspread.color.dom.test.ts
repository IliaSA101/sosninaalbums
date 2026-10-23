// @vitest-environment happy-dom
import { afterEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/svelte'
import { flushSync } from 'svelte'

// «Фон» карточки разворота и окно цвета (109): наведение показывает цвет на нужной странице, не правя альбом.
const setBackground = vi.hoisted(() => vi.fn(async () => undefined))
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', async (orig) => {
  const all = await orig<Record<string, unknown>>()
  return {
    ...Object.fromEntries(Object.keys(all).map((k) => [k, vi.fn(async () => undefined)])),
    MyColors: vi.fn(async () => []),
    SetBackground: setBackground,
  }
})

const { default: InspectorSpread } = await import('./InspectorSpread.svelte')
const { store } = await import('../../base/core/store.svelte')
const { prefs } = await import('../../base/prefs/prefs.svelte')
const { colorLive, pageBg } = await import('../../base/color/colorlive.svelte')

afterEach(() => {
  cleanup()
  colorLive.clear()
  setBackground.mockClear()
  vi.restoreAllMocks()
})

function show(bg: [string, string], bgSet: [string, string]) {
  store.view = {
    ...store.view,
    rev: 5,
    settings: { pageW: 300, pageH: 300 },
    spreads: [{ id: 's1', grid: false, photos: [], places: [], frames: [], rects: [], bg, bgSet, backdrops: [], patterns: [] }],
  } as never
  vi.spyOn(prefs, 'can').mockImplementation((f) => f === 'bgColor')
  render(InspectorSpread)
}

async function hover(label: string, hex: string) {
  await fireEvent.click(screen.getByRole('button', { name: label }))
  flushSync()
  await fireEvent.pointerEnter(screen.getAllByRole('button', { name: new RegExp(hex) })[0])
}

describe('InspectorSpread: показ цвета фона', () => {
  it('наведение на фон страницы меняет только её, rev и Go не затронуты', async () => {
    show(['#ffffff', '#eeeeee'], ['', ''])
    await fireEvent.click(screen.getByRole('button', { name: /По страницам/ }))
    await hover('Цвет фона: правая страница', '#4a4643')
    const bg: [string, string] = ['#ffffff', '#eeeeee']
    expect(pageBg('s1', bg, ['', ''])).toEqual(['#ffffff', '#4a4643'])
    expect(store.view.rev).toBe(5)
    expect(setBackground).not.toHaveBeenCalled()
  })

  it('наведение на фон всего разворота — обе страницы; уход — прежние', async () => {
    show(['#ffffff', '#ffffff'], ['', ''])
    await hover('Цвет фона разворота', '#4a4643')
    const bg: [string, string] = ['#ffffff', '#ffffff']
    expect(pageBg('s1', bg, ['', ''])).toEqual(['#4a4643', '#4a4643'])
    await fireEvent.pointerLeave(screen.getAllByRole('button', { name: /#4a4643/ })[0])
    expect(pageBg('s1', bg, ['', ''])).toEqual(bg)
  })

  it('щелчок — одна правка SetBackground, показ держится до ответа Go', async () => {
    show(['#ffffff', '#ffffff'], ['', ''])
    await hover('Цвет фона разворота', '#4a4643')
    await fireEvent.click(screen.getAllByRole('button', { name: /#4a4643/ })[0])
    expect(setBackground).toHaveBeenCalledExactlyOnceWith('s1', 'spread', '#4a4643')
    expect(colorLive.color('bg:s1:spread', '')).toBe('#4a4643')
    await new Promise((r) => setTimeout(r, 0))
    expect(colorLive.color('bg:s1:spread', '')).toBe('')
  })
})
