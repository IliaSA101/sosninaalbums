// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, render } from '@testing-library/svelte'
import { flushSync } from 'svelte'

// Курсор-лупа над лентой, пока зажат Ctrl (⌘): подсказка жеста крупного просмотра (128-9, 0.26-128-16).
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', () => ({ Marks: async () => ({}) }))

const { default: Library } = await import('./Library.svelte')
const { peekKey, cutKey } = await import('../timeline/cutkey.svelte')
const { store } = await import('../../base/core/store.svelte')
const { prefs } = await import('../../base/prefs/prefs.svelte')
const { setOsForTest } = await import('../../base/core/platform')
const { wire } = await import('../../shell/boot/wire')
const { dropCatalog, useCatalog } = await import('../../base/keys/keymap.fixture')

wire()

beforeEach(() => {
  prefs.level = 'standard'
  peekKey.held = false
  store.loaded = true
  store.view = { ...store.view, folders: ['D:/a'] } as never
  store.photos = [{ id: 'p1', path: 'D:/a/p1.jpg', name: 'p1.jpg', w: 100, h: 100 }] as never
})
afterEach(() => {
  cleanup()
  setOsForTest(null)
  dropCatalog()
  peekKey.held = false
})

const grid = () => document.querySelector<HTMLElement>('.grid')!
const key = (type: string, init: KeyboardEventInit) => {
  window.dispatchEvent(new KeyboardEvent(type, init))
  flushSync()
}

describe('title плитки: вторая строка о жесте (Т4)', () => {
  const title = () => document.querySelector<HTMLElement>('.thumb')!.getAttribute('title')

  it('клавиша из реестра, Windows/Linux — Ctrl', () => {
    useCatalog()
    setOsForTest('windows')
    render(Library)
    flushSync()
    expect(title()).toBe('p1.jpg\nCtrl+правый щелчок — крупно (Z)')
  })
  it('Mac — ⌘; переназначенная клавиша видна сразу, снятая — без скобок', () => {
    useCatalog({ photoPreview: ['KeyB'] })
    setOsForTest('mac')
    render(Library)
    flushSync()
    expect(title()).toBe('p1.jpg\n⌘+правый щелчок — крупно (B)')
    useCatalog({ photoPreview: [] })
    flushSync()
    expect(title()).toBe('p1.jpg\n⌘+правый щелчок — крупно')
  })
})

describe('курсор-лупа над лентой при зажатом Ctrl', () => {
  it('Ctrl вниз — класс peek на сетке, keyup и blur снимают', () => {
    setOsForTest('windows')
    render(Library)
    flushSync()
    expect(grid().classList.contains('peek')).toBe(false)
    key('keydown', { key: 'Control', ctrlKey: true })
    expect(grid().classList.contains('peek')).toBe(true)
    key('keyup', { key: 'Control', ctrlKey: false })
    expect(grid().classList.contains('peek')).toBe(false)
    key('keydown', { key: 'Control', ctrlKey: true })
    expect(grid().classList.contains('peek')).toBe(true)
    window.dispatchEvent(new Event('blur'))
    flushSync()
    expect(grid().classList.contains('peek')).toBe(false)
  })

  it('на Mac лупу даёт ⌘, а не Ctrl', () => {
    setOsForTest('mac')
    render(Library)
    flushSync()
    key('keydown', { key: 'Control', ctrlKey: true })
    expect(grid().classList.contains('peek')).toBe(false)
    key('keydown', { key: 'Meta', metaKey: true })
    expect(grid().classList.contains('peek')).toBe(true)
  })

  it('pointermove с Ctrl тоже ставит признак, без Ctrl — снимает', () => {
    setOsForTest('linux')
    render(Library)
    flushSync()
    window.dispatchEvent(new MouseEvent('pointermove', { ctrlKey: true }))
    flushSync()
    expect(grid().classList.contains('peek')).toBe(true)
    window.dispatchEvent(new MouseEvent('pointermove', { ctrlKey: false }))
    flushSync()
    expect(grid().classList.contains('peek')).toBe(false)
  })

  it('экземпляр отдельный: cutKey ленты разворотов не трогается', () => {
    setOsForTest('linux')
    render(Library)
    flushSync()
    key('keydown', { key: 'Control', ctrlKey: true })
    expect(peekKey.held).toBe(true)
    expect(cutKey.held).toBe(false)
  })

  it('лента размонтирована — слушатели сняты, признак сброшен', () => {
    setOsForTest('linux')
    render(Library)
    flushSync()
    key('keydown', { key: 'Control', ctrlKey: true })
    cleanup()
    expect(peekKey.held).toBe(false)
    key('keydown', { key: 'Control', ctrlKey: true })
    expect(peekKey.held).toBe(false)
  })
})
