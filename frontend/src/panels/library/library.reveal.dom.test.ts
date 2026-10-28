// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, render } from '@testing-library/svelte'
import { flushSync } from 'svelte'

// Закрытие крупного просмотра прокручивает виртуальную ленту к показанному кадру (128, решение 0.26-128-17).
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', () => ({ Marks: async () => ({}) }))

const { default: Library } = await import('./Library.svelte')
const { revealTop } = await import('./libwindow')
const { photoui } = await import('../../editor/photo/photoui.svelte')
const { store } = await import('../../base/core/store.svelte')
const { modes } = await import('../../base/prefs/view.svelte')
const { prefs } = await import('../../base/prefs/prefs.svelte')
const { wire } = await import('../../shell/boot/wire')

wire()

const ph = (n: number) => ({ id: `p${n}`, path: `D:/a/p${n}.jpg`, name: `p${n}.jpg`, w: 100, h: 100 })

beforeEach(() => {
  prefs.level = 'standard'
  vi.spyOn(HTMLElement.prototype, 'clientWidth', 'get').mockReturnValue(1000)
  vi.spyOn(HTMLElement.prototype, 'clientHeight', 'get').mockReturnValue(300)
  store.loaded = true
  store.view = { ...store.view, folders: ['D:/a'] } as never
  store.photos = Array.from({ length: 300 }, (_, i) => ph(i)) as never
  store.selected.clear()
  photoui.reveal = null
})
afterEach(() => {
  cleanup()
  vi.restoreAllMocks()
  photoui.preview = null
})

const grid = () => document.querySelector<HTMLElement>('.grid')!

describe('прокрутка ленты после просмотра', () => {
  it('показанный кадр далеко вниз — лента прокручивается к его ряду', () => {
    render(Library)
    flushSync()
    photoui.openLibrary('p250')
    photoui.closePreview()
    flushSync()
    const want = revealTop(250, 0, 300, 1000, modes.thumbSize)
    expect(want).toBeGreaterThan(0)
    expect(grid().scrollTop).toBe(want)
  })

  it('кадр уже виден — лента не двигается', () => {
    render(Library)
    flushSync()
    photoui.openLibrary('p1')
    photoui.closePreview()
    flushSync()
    expect(grid().scrollTop).toBe(0)
  })

  it('кадр скрыт — лента идёт к ближайшему видимому по замороженному списку', () => {
    render(Library)
    flushSync()
    photoui.openLibrary('p200')
    store.photos = store.photos.filter((p) => p.id !== 'p200') as never
    photoui.closePreview()
    flushSync()
    const idx = store.visible.findIndex((p) => p.id === 'p201')
    expect(idx).toBeGreaterThan(0)
    expect(grid().scrollTop).toBe(revealTop(idx, 0, 300, 1000, modes.thumbSize))
  })
})
