// @vitest-environment happy-dom
import { afterEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/svelte'
import { flushSync } from 'svelte'

// «Откуда имена» на общем Seg: «Список» ↔ «Имена файлов», повторный щелчок по нажатому ничего не меняет.
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', async (orig) => {
  const all = await orig<Record<string, unknown>>()
  return { ...Object.fromEntries(Object.keys(all).map((k) => [k, vi.fn(async () => undefined)])) }
})

const { default: NamesDialog } = await import('./NamesDialog.svelte')
const { namesUi } = await import('./names.svelte')
const { store } = await import('../../base/core/store.svelte')

afterEach(() => {
  cleanup()
  namesUi.open = false
  namesUi.from = 'list'
})

function show() {
  store.view = {
    ...store.view,
    settings: { pageW: 300, pageH: 300, bg: '' },
    spreads: [{ id: 's1', name: '', grid: false, photos: [], places: [], frames: [], rects: [] }],
  } as never
  namesUi.spreadId = 's1'
  namesUi.from = 'list'
  namesUi.open = true
  render(NamesDialog)
  flushSync()
}

describe('NamesDialog: откуда имена', () => {
  it('щелчок переключает источник, повторный по нажатому — нет', async () => {
    show()
    const list = screen.getByRole('button', { name: 'Список' })
    const files = screen.getByRole('button', { name: 'Имена файлов' })
    expect(list.getAttribute('aria-pressed')).toBe('true')
    await fireEvent.click(files)
    expect(namesUi.from).toBe('files')
    expect(files.getAttribute('aria-pressed')).toBe('true')
    expect(list.getAttribute('aria-pressed')).toBe('false')
    await fireEvent.click(files)
    expect(namesUi.from).toBe('files')
    await fireEvent.click(list)
    expect(namesUi.from).toBe('list')
  })
})
