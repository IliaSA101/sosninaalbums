// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, render } from '@testing-library/svelte'
import { flushSync } from 'svelte'

// Вопрос поверх крупного просмотра («Файл новее», 128): ложится выше по z, Esc и стрелки достаются ему, а не просмотру.
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', async (orig) => ({
  ...(await orig<typeof import('../../../wailsjs/go/app/App')>()),
  OpenInNewer: vi.fn(),
  OpenLink: vi.fn(),
}))

const { default: PhotoPreview } = await import('./PhotoPreview.svelte')
const { default: NewerAskDialog } = await import('../../shell/ask/NewerAskDialog.svelte')
const { newerAsk } = await import('../../shell/ask/newerask.svelte')
const { photoui } = await import('./photoui.svelte')
const { store } = await import('../../base/core/store.svelte')
const { modalStack, Z_MODAL } = await import('../../base/modal/modalstack.svelte')
const { useCatalog, dropCatalog } = await import('../../base/keys/keymap.fixture')

const photo = (id: string) => ({ id, name: `${id}.jpg`, w: 3, h: 2 })
const z = (el: Element | null) => Number((el as HTMLElement).style.zIndex)
const backs = () => [...document.querySelectorAll<HTMLElement>('.back')]
const key = (k: string) => {
  const e = new KeyboardEvent('keydown', { key: k, code: k, bubbles: true, cancelable: true })
  document.body.dispatchEvent(e)
  flushSync()
  return e
}

beforeEach(() => {
  useCatalog()
  store.photos = ['a', 'b', 'c'].map(photo) as never
  store.zone = 'library'
  render(PhotoPreview)
  render(NewerAskDialog)
  photoui.openLibrary('b')
  flushSync()
})
afterEach(() => {
  newerAsk.close()
  cleanup()
  photoui.preview = null
  dropCatalog()
})

const info = { name: 'Свадьба.salbum', draft: false, need: '0.29', mine: '0.28', have: '' }

describe('вопрос поверх просмотра', () => {
  it('просмотр один — на нижнем слое стека', () => {
    expect(z(backs()[0])).toBe(Z_MODAL)
    expect(modalStack.count()).toBe(1)
  })

  it('вопрос выше просмотра по z, хотя в DOM стоит после него и раньше открытия', () => {
    newerAsk.show(info)
    flushSync()
    const [pv, ask] = backs()
    expect(z(ask)).toBeGreaterThan(z(pv))
  })

  it('Esc закрывает только вопрос, просмотр остаётся и снова получает Esc', () => {
    newerAsk.show(info)
    flushSync()
    key('Escape')
    expect(newerAsk.cur).toBeNull()
    expect(photoui.previewId).toBe('b')
    key('Escape')
    expect(photoui.previewId).toBeNull()
  })

  it('стрелки под вопросом не листают просмотр, после закрытия листают', () => {
    newerAsk.show(info)
    flushSync()
    key('ArrowRight')
    expect(photoui.previewId).toBe('b')
    newerAsk.close()
    flushSync()
    key('ArrowRight')
    expect(photoui.previewId).toBe('c')
  })
})
