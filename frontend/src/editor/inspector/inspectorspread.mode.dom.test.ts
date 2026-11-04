// @vitest-environment happy-dom
import { afterEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/svelte'

// «Авто · Ручная» на общем Seg (card): щелчок по нажатому ничего не зовёт, по другому — setGrid(true) / backToAuto.
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', async (orig) => {
  const all = await orig<Record<string, unknown>>()
  return { ...Object.fromEntries(Object.keys(all).map((k) => [k, vi.fn(async () => undefined)])), MyColors: vi.fn(async () => []) }
})

const { default: InspectorSpread } = await import('./InspectorSpread.svelte')
const { store } = await import('../../base/core/store.svelte')
const { prefs } = await import('../../base/prefs/prefs.svelte')
const { gridAct } = await import('../../panels/layouts/gridact.svelte')
const { cardContext } = await import('../../base/prefs/cardgroups.svelte')

function show(grid: boolean) {
  store.view = {
    ...store.view,
    rev: 5,
    settings: { pageW: 300, pageH: 300, bg: '' },
    spreads: [
      {
        id: 's1',
        name: '',
        grid,
        photos: ['p1'],
        places: [],
        frames: [],
        rects: [],
        bg: ['#ffffff', '#ffffff'],
        bgSet: ['', ''],
        backdrops: [],
        patterns: [],
        captions: [],
      },
    ],
  } as never
  store.go(0)
  vi.spyOn(prefs, 'can').mockImplementation(() => true)
  render(InspectorSpread, { context: new Map([cardContext(() => true)]) })
}
const btn = (n: string) => screen.getByRole('button', { name: n })

afterEach(() => {
  cleanup()
  vi.restoreAllMocks()
})

describe('InspectorSpread: Авто · Ручная', () => {
  it('«Авто»: щелчок по нажатому не зовёт backToAuto, «Ручная» зовёт setGrid(true)', async () => {
    const back = vi.spyOn(gridAct, 'backToAuto').mockResolvedValue()
    const set = vi.spyOn(gridAct, 'setGrid').mockResolvedValue()
    show(false)
    expect(btn('Авто').getAttribute('aria-pressed')).toBe('true')
    await fireEvent.click(btn('Авто'))
    expect(back).not.toHaveBeenCalled()
    await fireEvent.click(btn('Ручная'))
    expect(set).toHaveBeenCalledExactlyOnceWith(true)
  })

  it('«Ручная»: щелчок по нажатой не зовёт setGrid, «Авто» зовёт backToAuto(id)', async () => {
    const back = vi.spyOn(gridAct, 'backToAuto').mockResolvedValue()
    const set = vi.spyOn(gridAct, 'setGrid').mockResolvedValue()
    show(true)
    expect(btn('Ручная').getAttribute('aria-pressed')).toBe('true')
    await fireEvent.click(btn('Ручная'))
    expect(set).not.toHaveBeenCalled()
    await fireEvent.click(btn('Авто'))
    expect(back).toHaveBeenCalledExactlyOnceWith('s1')
  })
})
