// @vitest-environment happy-dom
import { afterEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/svelte'

// «Цвет | Ч/б» на общем Seg: щелчок по другому пункту — один SetGray через gate, по нажатому — ничего.
const api = vi.hoisted(() => ({ SetGray: vi.fn() }))
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', async (orig) => {
  const all = await orig<Record<string, unknown>>()
  return { ...Object.fromEntries(Object.keys(all).map((k) => [k, vi.fn(async () => undefined)])), ...api }
})

const { store } = await import('../../base/core/store.svelte')
const { default: AdjustRows } = await import('./AdjustRows.svelte')

const ok = async () => ({ ...store.view, rev: store.view.rev + 1 })
function setup(gray: boolean) {
  store.view = {
    ...store.view,
    rev: 5,
    spreads: [{ id: 's1', grid: false, photos: ['a'], rects: [], places: [], texts: [], captions: [], frames: [{ gray }] }],
  } as never
  store.go(0)
  render(AdjustRows, { id: 'a' })
}
const btn = (n: string) => screen.getByRole('button', { name: n })

afterEach(() => {
  cleanup()
  vi.clearAllMocks()
})

describe('AdjustRows: Цвет | Ч/б', () => {
  it('цветное фото: «Ч/б» зовёт SetGray([id]), «Цвет» нажат и ничего не зовёт', async () => {
    api.SetGray.mockImplementation(ok)
    setup(false)
    expect(btn('Цвет').getAttribute('aria-pressed')).toBe('true')
    await fireEvent.click(btn('Цвет'))
    expect(api.SetGray).not.toHaveBeenCalled()
    await fireEvent.click(btn('Ч/б'))
    expect(api.SetGray).toHaveBeenCalledExactlyOnceWith(['a'])
  })

  it('чёрно-белое фото: «Цвет» зовёт SetGray, повторный щелчок по нажатому «Ч/б» — нет', async () => {
    api.SetGray.mockImplementation(ok)
    setup(true)
    expect(btn('Ч/б').getAttribute('aria-pressed')).toBe('true')
    await fireEvent.click(btn('Ч/б'))
    expect(api.SetGray).not.toHaveBeenCalled()
    await fireEvent.click(btn('Цвет'))
    expect(api.SetGray).toHaveBeenCalledExactlyOnceWith(['a'])
  })

  it('пока SetGray не вернулся, второй щелчок игнорируется (gate)', async () => {
    let done!: () => void
    api.SetGray.mockImplementation(() => new Promise<void>((r) => (done = r)).then(ok))
    setup(false)
    await fireEvent.click(btn('Ч/б'))
    await fireEvent.click(btn('Ч/б'))
    expect(api.SetGray).toHaveBeenCalledTimes(1)
    done()
  })
})
