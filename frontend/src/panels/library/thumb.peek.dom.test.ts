// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render } from '@testing-library/svelte'
import { flushSync } from 'svelte'
import { setMacForTest } from '../../base/core/platform'

// Ctrl+правый щелчок по плитке открывает крупный просмотр, правый щелчок — меню (128-2).
const m = vi.hoisted(() => ({ can: vi.fn((_f: string) => true) }))
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', () => ({ WhereUsed: vi.fn().mockResolvedValue([]), SetMarks: vi.fn() }))

const { default: Thumb } = await import('./Thumb.svelte')
const { default: ThumbMenu } = await import('./ThumbMenu.svelte')
const { thumbmenu } = await import('./thumbmenustate.svelte')
const { photoui } = await import('../../editor/photo/photoui.svelte')
const { store } = await import('../../base/core/store.svelte')
const { prefs } = await import('../../base/prefs/prefs.svelte')
const { drag } = await import('../../base/drag/drag.svelte')

const photo = (id: string) => ({ id, name: `${id}.jpg`, w: 3, h: 2 })

beforeEach(() => {
  vi.spyOn(prefs, 'can').mockImplementation((f) => m.can(f))
  m.can.mockReturnValue(true)
  store.view = { ...store.view, rev: 1, spreads: [], folders: [] } as never
  store.photos = ['a', 'b', 'c', 'd', 'e'].map(photo) as never
  store.selected.clear()
  thumbmenu.close()
  photoui.preview = null
})
afterEach(() => {
  cleanup()
  vi.restoreAllMocks()
  setMacForTest(null)
  drag.active = false
  photoui.preview = null
})

// Плитка b в окружении меню: щелчок кнопкой button с модификаторами; возвращает событие.
function press(id: string, init: MouseEventInit & { button: number }) {
  const p = store.photos.find((x) => x.id === id)!
  const { container } = render(Thumb, { p })
  render(ThumbMenu)
  const ev = new MouseEvent('contextmenu', { bubbles: true, cancelable: true, clientX: 30, clientY: 20, ...init })
  container.querySelector(`[data-thumb="${id}"]`)!.dispatchEvent(ev)
  flushSync()
  return ev
}

describe('Ctrl+правый щелчок по плитке', () => {
  it('открывает окно из ленты, меню не открывается, событие погашено; плитка выделена одна', () => {
    store.selected.add('a')
    const ev = press('b', { button: 2, ctrlKey: true })
    expect(photoui.preview).toEqual({ ids: ['a', 'b', 'c', 'd', 'e'], at: 1, from: 'library' })
    expect(thumbmenu.menu).toBeNull()
    expect(ev.defaultPrevented).toBe(true)
    expect([...store.selected]).toEqual(['b'])
  })

  it('правый щелчок без Ctrl — меню, окна нет', () => {
    press('b', { button: 2 })
    expect(thumbmenu.menu).not.toBeNull()
    expect(photoui.preview).toBeNull()
  })

  it('по выделенной плитке выделение из четырёх остаётся', () => {
    for (const id of ['a', 'b', 'c', 'd']) store.selected.add(id)
    store.anchor = 'a'
    press('c', { button: 2, ctrlKey: true })
    expect([...store.selected].sort()).toEqual(['a', 'b', 'c', 'd'])
    expect(photoui.previewId).toBe('c')
    expect(store.anchor).toBe('a')
  })

  it('Ctrl+левая кнопка остаётся выделением (щелчок), а не просмотром', async () => {
    const { container } = render(Thumb, { p: store.photos[1] })
    await fireEvent.click(container.querySelector('[data-thumb="b"]')!, { ctrlKey: true, button: 0 })
    expect([...store.selected]).toEqual(['b'])
    expect(photoui.preview).toBeNull()
  })

  it('«Простой» (меню нет): окно есть, а правый щелчок меню не открывает', () => {
    m.can.mockImplementation((f) => f !== 'photoMenu')
    press('b', { button: 2, ctrlKey: true })
    expect(photoui.previewId).toBe('b')
    cleanup()
    photoui.preview = null
    press('c', { button: 2 })
    expect(thumbmenu.menu).toBeNull()
    expect(photoui.preview).toBeNull()
  })

  it('во время переноса молчит', () => {
    drag.active = true
    const ev = press('b', { button: 2, ctrlKey: true })
    expect(photoui.preview).toBeNull()
    expect(store.selected.size).toBe(0)
    expect(ev.defaultPrevented).toBe(true) // браузерное меню всё равно не нужно
  })

  it('уже открытое меню закрывается, окно открывается', () => {
    thumbmenu.menu = { x: 1, y: 1, ids: ['a'] }
    press('b', { button: 2, ctrlKey: true })
    expect(thumbmenu.menu).toBeNull()
    expect(photoui.previewId).toBe('b')
  })
})

describe('Mac', () => {
  beforeEach(() => setMacForTest(true))

  it('⌘+правая кнопка — окно', () => {
    press('b', { button: 2, metaKey: true })
    expect(photoui.previewId).toBe('b')
    expect(thumbmenu.menu).toBeNull()
  })

  it('Ctrl+левая кнопка (так приходит правый щелчок трекпада) — меню, не окно', () => {
    press('b', { button: 0, ctrlKey: true })
    expect(thumbmenu.menu).not.toBeNull()
    expect(photoui.preview).toBeNull()
  })

  it('Ctrl+правая кнопка — тоже меню: просмотр только с ⌘', () => {
    press('b', { button: 2, ctrlKey: true })
    expect(thumbmenu.menu).not.toBeNull()
    expect(photoui.preview).toBeNull()
  })
})
