// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, render } from '@testing-library/svelte'

// Щипок трекпада Mac над холстом (134-4): тот же зум к курсору, что у Ctrl+колеса, с теми же запретами; один вызов за кадр.
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', () => ({}))

const { default: Stage } = await import('./Stage.svelte')
const { store } = await import('../../base/core/store.svelte')
const { canvas } = await import('./canvas.svelte')
const { prefs } = await import('../../base/prefs/prefs.svelte')
const { row } = await import('../spread/spreadrow.svelte')
const { BAR } = await import('../inspector/panels')

const sheet = (id: string) => ({
  id,
  photos: [],
  w: 600,
  h: 300,
  mid: 300,
  frames: [],
  texts: [],
  captions: [],
  bg: ['#fff', '#fff'],
  bgSet: ['', ''],
  backdrops: null,
  n: 0,
})

const gesture = (target: Element, type: string, init: { scale?: number; clientX?: number; clientY?: number }) => {
  const e = new Event(type, { bubbles: true, cancelable: true })
  for (const [k, v] of Object.entries(init)) Object.defineProperty(e, k, { value: v })
  target.dispatchEvent(e)
  return e
}
const wheel = (target: Element, init: WheelEventInit) => {
  const e = new WheelEvent('wheel', { bubbles: true, cancelable: true, ...init })
  for (const k of ['ctrlKey', 'metaKey'] as const) Object.defineProperty(e, k, { value: !!init[k] })
  target.dispatchEvent(e)
  return e
}
const stage = () => document.querySelector<HTMLElement>('.stage')!
const frames = () => new Promise<void>((r) => requestAnimationFrame(() => r()))

let zoomAt: ReturnType<typeof vi.spyOn>

beforeEach(() => {
  store.view = { rev: 1, folders: [], settings: { pageW: 300, pageH: 300 }, spreads: [sheet('s1')] } as never
  store.currentId = 's1'
  prefs.level = 'full'
  canvas.busy = false
  canvas.pressed = false
  zoomAt = vi.spyOn(canvas, 'zoomAt').mockImplementation(() => {})
  vi.spyOn(HTMLElement.prototype, 'getBoundingClientRect').mockReturnValue({
    left: 10,
    top: 20,
    width: 800,
    height: 600,
    right: 810,
    bottom: 620,
    x: 10,
    y: 20,
    toJSON: () => ({}),
  })
  vi.spyOn(HTMLElement.prototype, 'clientWidth', 'get').mockReturnValue(1200) // без размера Stage не рисует полосу
  vi.spyOn(HTMLElement.prototype, 'clientHeight', 'get').mockReturnValue(700)
  render(Stage)
})
afterEach(() => {
  cleanup()
  vi.restoreAllMocks()
  canvas.busy = false
  canvas.pressed = false
  row.atNew = false
  prefs.level = 'simple'
})

describe('щипок над холстом', () => {
  it('множитель жеста у курсора, погашен; два шага за кадр — один вызов с произведением', async () => {
    const start = gesture(stage(), 'gesturestart', { scale: 1, clientX: 300, clientY: 200 })
    const a = gesture(stage(), 'gesturechange', { scale: 1.2, clientX: 300, clientY: 200 })
    const b = gesture(stage(), 'gesturechange', { scale: 1.8, clientX: 310, clientY: 210 })
    expect([start, a, b].every((e) => e.defaultPrevented)).toBe(true)
    await frames()
    expect(zoomAt).toHaveBeenCalledTimes(1)
    const [f, x, y] = zoomAt.mock.calls[0] as number[]
    expect(f).toBeCloseTo(1.8, 10)
    expect([x, y]).toEqual([310 - 10, 210 - 20 - BAR])
  })

  it('над полосой и панелями (data-chrome) — только гасится', async () => {
    const chrome = document.querySelector('[data-chrome]')!
    gesture(chrome, 'gesturestart', { scale: 1 })
    const e = gesture(chrome, 'gesturechange', { scale: 2, clientX: 5, clientY: 5 })
    expect(e.defaultPrevented).toBe(true)
    await frames()
    expect(zoomAt).not.toHaveBeenCalled()
  })

  it('запреты zoomGate: жест правки, слот «новый», уровень без «Масштаба»', async () => {
    const pinch = async () => {
      gesture(stage(), 'gesturestart', { scale: 1 })
      gesture(stage(), 'gesturechange', { scale: 2, clientX: 50, clientY: 50 })
      gesture(stage(), 'gestureend', { scale: 2 })
      await frames()
    }
    canvas.busy = true
    await pinch()
    canvas.busy = false
    canvas.pressed = true
    await pinch()
    canvas.pressed = false
    row.atNew = true
    await pinch()
    row.atNew = false
    prefs.level = 'simple'
    await pinch()
    expect(zoomAt).not.toHaveBeenCalled()
  })

  it('Ctrl-колесо посреди жеста гасится и не зумит; после gestureend зумит как всегда', async () => {
    gesture(stage(), 'gesturestart', { scale: 1 })
    const mid = wheel(stage(), { deltaY: -100, ctrlKey: true, clientX: 50, clientY: 60 })
    expect(mid.defaultPrevented).toBe(true)
    gesture(stage(), 'gestureend', { scale: 1 })
    await frames()
    expect(zoomAt).not.toHaveBeenCalled()
    wheel(stage(), { deltaY: -100, ctrlKey: true, clientX: 50, clientY: 60 })
    await frames()
    expect(zoomAt).toHaveBeenCalledTimes(1)
  })
})
