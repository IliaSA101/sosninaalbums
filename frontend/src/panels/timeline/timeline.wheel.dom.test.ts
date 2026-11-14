// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, render } from '@testing-library/svelte'
import { tick } from 'svelte'

// Колесо и щипок над лентой альбома (134-8): колесо листает вбок и с Ctrl (зажатый Ctrl — режим разреза, 094 §1),
// щипок Mac только гасится: ни прокрутки, ни масштаба страницы.
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', () => ({}))

class SeenIO {
  constructor(private cb: IntersectionObserverCallback) {}
  observe(target: Element) {
    this.cb([{ target, isIntersecting: true } as IntersectionObserverEntry], this as never)
  }
  unobserve() {}
  disconnect() {}
}
vi.stubGlobal('IntersectionObserver', SeenIO)

const { default: Timeline } = await import('./Timeline.svelte')
const { store } = await import('../../base/core/store.svelte')

const rect = { x: 0, y: 0, w: 100, h: 100 }
const sp = (id: string) => ({ id, name: '', photos: [`${id}1`], rects: [rect], w: 400, h: 200, kind: '' })

beforeEach(() => {
  store.view = { ...store.view, rev: 1, spreads: ['a', 'b', 'c'].map(sp), settings: {} } as never
  store.go(0)
})
afterEach(() => cleanup())

async function strip() {
  const r = render(Timeline)
  await tick()
  await tick()
  return r.container.querySelector<HTMLElement>('.timeline')!
}
const wheel = (box: HTMLElement, init: { deltaY?: number; deltaX?: number; ctrlKey?: boolean; metaKey?: boolean }) => {
  const e = new WheelEvent('wheel', { bubbles: true, cancelable: true, deltaY: init.deltaY ?? 0, deltaX: init.deltaX ?? 0 })
  for (const k of ['ctrlKey', 'metaKey'] as const) Object.defineProperty(e, k, { value: !!init[k] })
  box.dispatchEvent(e)
  return e
}

describe('колесо над лентой альбома', () => {
  it('простое колесо листает вбок и гасится', async () => {
    const box = await strip()
    box.scrollLeft = 0
    const e = wheel(box, { deltaY: 60 })
    expect(e.defaultPrevented).toBe(true)
    expect(box.scrollLeft).toBe(60)
  })

  it('Ctrl+колесо и ⌘+колесо листают, как прежде (режим разреза: крутить до места разреза)', async () => {
    const box = await strip()
    box.scrollLeft = 10
    for (const mod of [{ ctrlKey: true }, { metaKey: true }]) {
      const e = wheel(box, { deltaY: 60, ...mod })
      expect(e.defaultPrevented).toBe(true)
    }
    expect(box.scrollLeft).toBe(130)
  })

  it('щипок Mac гасится и ленту не двигает', async () => {
    const box = await strip()
    box.scrollLeft = 10
    for (const type of ['gesturestart', 'gesturechange', 'gestureend']) {
      const e = new Event(type, { bubbles: true, cancelable: true })
      Object.defineProperty(e, 'scale', { value: 2 })
      box.dispatchEvent(e)
      expect(e.defaultPrevented, type).toBe(true)
    }
    expect(box.scrollLeft).toBe(10)
  })

  it('боковое колесо (тачпад вбок) не гасится и лентой не двигается кодом', async () => {
    const box = await strip()
    box.scrollLeft = 10
    const e = wheel(box, { deltaX: 80, deltaY: 5 })
    expect(e.defaultPrevented).toBe(false)
    expect(box.scrollLeft).toBe(10)
  })
})
