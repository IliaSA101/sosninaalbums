// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))

const { spot, seek } = await import('./spotstate.svelte')

// Режим hold (113): кольцо тура держится, пока открыт шаг; обычное кольцо гасит любой щелчок и клавиша.
const later = (ms: number) => new Promise<void>((r) => setTimeout(r, ms))

function button(attrs: Record<string, string> = {}): HTMLElement {
  const el = document.createElement('button')
  for (const [k, v] of Object.entries(attrs)) el.setAttribute(k, v)
  document.body.append(el)
  el.getBoundingClientRect = () => ({ left: 10, top: 20, width: 50, height: 30, right: 60, bottom: 50, x: 10, y: 20, toJSON: () => ({}) })
  return el
}

beforeEach(() => {
  Object.defineProperty(window, 'innerWidth', { value: 1200, configurable: true })
  Object.defineProperty(window, 'innerHeight', { value: 700, configurable: true })
})
afterEach(() => {
  spot.cancel()
  document.body.innerHTML = ''
})

describe('Spot: режим hold', () => {
  it('щелчок и клавиша кольцо не гасят, cancel() гасит', async () => {
    const el = button()
    spot.flash(el, '', '', undefined, { hold: true })
    expect(spot.cur?.hold).toBe(true)
    await later(80)
    window.dispatchEvent(new Event('pointerdown'))
    window.dispatchEvent(new KeyboardEvent('keydown', { key: 'a' }))
    expect(spot.cur?.el).toBe(el)
    expect(el.hasAttribute('data-spot')).toBe(true)
    spot.cancel()
    expect(spot.cur).toBeNull()
    expect(el.hasAttribute('data-spot')).toBe(false)
  })

  it('рамка кольца считается сразу и после resize', async () => {
    const el = button()
    spot.flash(el, '', '', undefined, { hold: true })
    expect(spot.box).toMatchObject({ left: 6, top: 16, width: 58, height: 38 })
    el.getBoundingClientRect = () => ({
      left: 100,
      top: 20,
      width: 50,
      height: 30,
      right: 150,
      bottom: 50,
      x: 100,
      y: 20,
      toJSON: () => ({}),
    })
    window.dispatchEvent(new Event('resize'))
    expect(spot.box?.left).toBe(96)
  })

  it('без hold щелчок гасит кольцо (прежнее поведение)', async () => {
    const el = button()
    spot.flash(el, 'Название', 'где')
    expect(spot.cur?.hold).toBe(false)
    await later(80)
    window.dispatchEvent(new Event('pointerdown'))
    expect(spot.cur).toBeNull()
  })

  it('следующий flash заменяет hold-кольцо', () => {
    const a = button()
    const b = button()
    spot.flash(a, '', '', undefined, { hold: true })
    spot.flash(b, '', '', undefined, { hold: true })
    expect(spot.cur?.el).toBe(b)
    expect(a.hasAttribute('data-spot')).toBe(false)
  })
})

describe('seek: общий поиск места', () => {
  it('находит по data-guide и по «⋯» с подсказкой', async () => {
    const a = button({ 'data-guide': 'topbar-export' })
    expect((await seek('ui:topbar-export'))?.el).toBe(a)
    expect((await seek('ui:topbar-export', 'подсказка'))?.hint).toBe('подсказка')
    const m = button({ 'data-guide-hold': 'flip' })
    expect(await seek('ui:flip')).toMatchObject({ el: m, hint: 'В меню «⋯»' })
  })

  it('места нет — null', async () => {
    expect(await seek('ui:no-such-place')).toBeNull()
  })

  it('отмена: поиск без места кончается сразу, а не через 600 мс; с отменённым сигналом документ не читается', async () => {
    const ctl = new AbortController()
    const t0 = performance.now()
    const p = seek('ui:no-such-place', '', ctl.signal)
    ctl.abort()
    expect(await p).toBeNull()
    expect(performance.now() - t0).toBeLessThan(500)
    const q = vi.spyOn(document, 'querySelectorAll')
    expect(await seek('ui:topbar-export', '', ctl.signal)).toBeNull()
    expect(q).not.toHaveBeenCalled()
    q.mockRestore()
  })
})
