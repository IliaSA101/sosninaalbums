// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, render, screen } from '@testing-library/svelte'
import { flushSync } from 'svelte'

// Холодный старт на альбоме новее программы (0.25-n2ui): «Альбом из новой версии» не прячется под стартовым окном,
// тост лежит выше любого окна. Go подменён.
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', () => ({ Recent: async () => [], OpenLink: async () => {} }))

const { default: StartDialog } = await import('../dialogs/StartDialog.svelte')
const { default: NewerAskDialog } = await import('./NewerAskDialog.svelte')
const { default: Overlay } = await import('../../base/drag/Overlay.svelte')
const { default: Modal } = await import('../../base/modal/Modal.svelte')
const { newerAsk } = await import('./newerask.svelte')
const { store } = await import('../../base/core/store.svelte')
const { prefs } = await import('../../base/prefs/prefs.svelte')
const { MODAL_DEPTH_MAX, Z_MODAL, Z_TOAST } = await import('../../base/modal/modalstack.svelte')

const info = { name: 'Alpha.salbum', draft: false, need: '0.26', mine: '0.25', have: '0.25.1' }
const z = (el: Element | null) => Number((el as HTMLElement).style.zIndex)
const backOf = (title: string) => screen.getByRole('dialog', { name: title }).parentElement

beforeEach(() => {
  store.toast = null
  store.loaded = true
  store.recent = []
})
afterEach(() => {
  cleanup()
  newerAsk.close()
  prefs.startOpen = false
  prefs.first = false
  store.toast = null
})

describe('окна холодного старта', () => {
  it('первый запуск: «новее» монтируется вместе со стартовым и лежит выше него', () => {
    prefs.startOpen = true
    prefs.first = true
    render(StartDialog)
    render(NewerAskDialog)
    newerAsk.show(info)
    flushSync()
    expect(z(backOf('Альбом из новой версии'))).toBeGreaterThan(z(backOf('Sosnina Albums')))
  })

  it('стартовое окно закрыто, «новее» открыто: в DOM только оно', () => {
    prefs.startOpen = false
    render(StartDialog)
    render(NewerAskDialog)
    newerAsk.show(info)
    flushSync()
    expect(screen.queryByRole('dialog', { name: 'Sosnina Albums' })).toBeNull()
    expect(screen.queryByRole('dialog', { name: 'Альбом из новой версии' })).not.toBeNull()
  })
})

describe('тост выше окон', () => {
  it('слой тоста выше самого верхнего возможного окна', () => {
    expect(Z_TOAST).toBeGreaterThan(Z_MODAL + MODAL_DEPTH_MAX)
  })

  it('тост при открытом стартовом окне: z-index выше окна', () => {
    prefs.startOpen = true
    render(StartDialog)
    render(Overlay)
    store.notify('Программа обновлена')
    flushSync()
    const toast = screen.getByText('Программа обновлена').parentElement
    expect(z(toast)).toBe(Z_TOAST)
    expect(z(toast)).toBeGreaterThan(z(backOf('Sosnina Albums')))
  })

  it('тост выше окна на предельной глубине стека', () => {
    const noop = () => {}
    const empty = (() => {}) as never
    for (let i = 0; i < MODAL_DEPTH_MAX + 2; i++) render(Modal, { title: 'W' + i, onclose: noop, children: empty })
    render(Overlay)
    store.notify('Вышла версия')
    flushSync()
    const top = Math.max(...screen.getAllByRole('dialog').map((d) => z(d.parentElement)))
    expect(top).toBe(Z_MODAL + MODAL_DEPTH_MAX)
    expect(z(screen.getByText('Вышла версия').parentElement)).toBeGreaterThan(top)
  })
})
