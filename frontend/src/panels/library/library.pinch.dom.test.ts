// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, render } from '@testing-library/svelte'
import { flushSync } from 'svelte'

// Щипок Mac над лентой фото (134, сценарии 7–8 спеки): размер миниатюр меняется, во время переноса фото — нет.
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', () => ({ Marks: async () => ({}) }))

const { default: Library } = await import('./Library.svelte')
const { store } = await import('../../base/core/store.svelte')
const { modes } = await import('../../base/prefs/view.svelte')
const { prefs } = await import('../../base/prefs/prefs.svelte')
const { drag } = await import('../../base/drag/drag.svelte')
const { wire } = await import('../../shell/boot/wire')

wire()

const ph = (n: number) => ({ id: `p${n}`, path: `D:/a/p${n}.jpg`, name: `p${n}.jpg`, w: 100, h: 100 })

beforeEach(() => {
  prefs.level = 'standard'
  vi.spyOn(HTMLElement.prototype, 'clientWidth', 'get').mockReturnValue(1000)
  vi.spyOn(HTMLElement.prototype, 'clientHeight', 'get').mockReturnValue(300)
  store.loaded = true
  store.view = { ...store.view, folders: ['D:/a'] } as never
  store.photos = Array.from({ length: 30 }, (_, i) => ph(i)) as never
  modes.setThumb(110)
  render(Library)
  flushSync()
})
afterEach(() => {
  cleanup()
  vi.restoreAllMocks()
  drag.active = false
})

const lib = () => document.querySelector<HTMLElement>('.library')!
const gesture = (type: string, scale: number) => {
  const e = new Event(type, { bubbles: true, cancelable: true })
  for (const [k, v] of Object.entries({ scale, clientX: 50, clientY: 50 })) Object.defineProperty(e, k, { value: v })
  lib().dispatchEvent(e)
  return e
}
const wheel = (init: { deltaY: number; ctrlKey?: boolean }) => {
  const e = new WheelEvent('wheel', { bubbles: true, cancelable: true, deltaY: init.deltaY })
  Object.defineProperty(e, 'ctrlKey', { value: !!init.ctrlKey })
  lib().dispatchEvent(e)
  return e
}
const frame = () => new Promise<void>((r) => requestAnimationFrame(() => r()))

describe('щипок над лентой фото', () => {
  it('развёл пальцы — миниатюры крупнее, жест погашен', async () => {
    expect(gesture('gesturestart', 1).defaultPrevented).toBe(true)
    expect(gesture('gesturechange', 2).defaultPrevented).toBe(true)
    await vi.waitFor(() => expect(modes.thumbSize).toBe(Math.round(110 * Math.SQRT2)))
    gesture('gestureend', 2)
  })

  it('во время переноса фото размер не меняется, жест всё равно погашен', async () => {
    drag.active = true
    gesture('gesturestart', 1)
    expect(gesture('gesturechange', 2).defaultPrevented).toBe(true)
    await frame()
    await frame()
    expect(modes.thumbSize).toBe(110)
    gesture('gestureend', 2)
  })

  it('Ctrl-колесо посреди щипка размер не меняет; без жеста меняет', async () => {
    gesture('gesturestart', 1)
    expect(wheel({ deltaY: -100, ctrlKey: true }).defaultPrevented).toBe(true)
    await frame()
    await frame()
    expect(modes.thumbSize).toBe(110)
    gesture('gestureend', 1)
    wheel({ deltaY: -100, ctrlKey: true })
    await vi.waitFor(() => expect(modes.thumbSize).toBeGreaterThan(110))
  })
})
