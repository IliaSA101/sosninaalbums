// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/svelte'
import { flushSync } from 'svelte'

// «Настройки альбома» и окно цвета (109, К8): наведение показывает цвет на страницах без своего цвета, выбор
// попадает в черновик формы до «Применить» и ни одной правки альбома не делает.
const setSettings = vi.hoisted(() => vi.fn(async () => undefined))
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', async (orig) => {
  const all = await orig<Record<string, unknown>>()
  return {
    ...Object.fromEntries(Object.keys(all).map((k) => [k, vi.fn(async () => undefined)])),
    PagePresets: vi.fn(async () => []),
    MarginNote: vi.fn(async () => ''),
    MyColors: vi.fn(async () => []),
    SetSettings: setSettings,
  }
})

const { default: SettingsForm } = await import('./SettingsForm.svelte')
const { store } = await import('../../base/core/store.svelte')
const { colorLive, pageBg } = await import('../../base/color/colorlive.svelte')

const settings = { pageW: 300, pageH: 300, dpi: 300, margin: 10, gap: 3, safe: 5, bleed: 3, bg: '' }

beforeEach(() => {
  store.view = { rev: 4, folders: [], spreads: [], settings } as never
})
afterEach(() => {
  cleanup()
  colorLive.clear()
  setSettings.mockClear()
})

describe('SettingsForm: цвет страниц альбома', () => {
  it('наведение — показ «album»: красятся страницы без своего цвета; Go и rev не тронуты', async () => {
    render(SettingsForm)
    await fireEvent.click(screen.getByRole('button', { name: 'Цвет страниц альбома' }))
    flushSync()
    await fireEvent.pointerEnter(screen.getAllByRole('button', { name: /#4a4643/ })[0])
    const bg: [string, string] = ['#ffffff', '#eeeeee']
    expect(pageBg('s1', bg, ['', '#eeeeee'])).toEqual(['#4a4643', '#eeeeee'])
    expect(store.view.rev).toBe(4)
    await fireEvent.pointerLeave(screen.getAllByRole('button', { name: /#4a4643/ })[0])
    expect(pageBg('s1', bg, ['', ''])).toEqual(bg)
    expect(setSettings).not.toHaveBeenCalled()
  })

  it('щелчок — цвет в форме (образец окна становится им), показ снят, SetSettings не звали', async () => {
    render(SettingsForm)
    await fireEvent.click(screen.getByRole('button', { name: 'Цвет страниц альбома' }))
    flushSync()
    await fireEvent.pointerEnter(screen.getAllByRole('button', { name: /#4a4643/ })[0])
    await fireEvent.click(screen.getAllByRole('button', { name: /#4a4643/ })[0])
    await new Promise((r) => setTimeout(r, 0))
    expect(colorLive.cur).toBeNull()
    expect(screen.getByRole('button', { name: 'Цвет страниц альбома' }).style.getPropertyValue('--c')).toBe('#4a4643')
    expect(setSettings).not.toHaveBeenCalled()
    expect(store.view.settings.bg).toBe('')
  })
})
