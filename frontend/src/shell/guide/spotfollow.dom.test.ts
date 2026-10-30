// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))

const { spot, seek } = await import('./spotstate.svelte')
const { modes } = await import('../../base/prefs/view.svelte')

// F1 «Показать» к полю в разделе панели деталей при закрытой панели (123 QA): раздел раскрывается ~0,45 с, высота
// прокрутки растёт, а первая подводка упёрлась бы в старый предел — поле осталось бы за краем. Подводка идёт после роста.
const later = (ms: number) => new Promise<void>((r) => setTimeout(r, ms))
const VIEW = 400 // высота видимой части панели
const FULL = 2600 // высота содержимого после раскрытия
const AT = 1900 // положение поля в содержимом (раскладка полная и до роста: тело раздела обрезано, но размечено целиком)
const GROW = 450

function mountPanel() {
  const aside = document.body.appendChild(document.createElement('aside'))
  aside.style.overflowY = 'auto'
  aside.innerHTML = '<div data-guide="page-px"><input /></div>'
  const target = aside.firstElementChild as HTMLElement
  const t0 = performance.now()
  const total = () => 800 + (FULL - 800) * Math.min(1, (performance.now() - t0) / GROW)
  let top = 0
  Object.defineProperty(aside, 'scrollHeight', { get: total })
  Object.defineProperty(aside, 'clientHeight', { get: () => VIEW })
  Object.defineProperty(aside, 'scrollTop', {
    get: () => top,
    set: (v: number) => (top = Math.min(Math.max(0, v), Math.max(0, total() - VIEW))),
  })
  const rect = (t: number, h: number) => ({
    left: 0,
    right: 200,
    width: 200,
    top: t,
    bottom: t + h,
    height: h,
    x: 0,
    y: t,
    toJSON: () => ({}),
  })
  aside.getBoundingClientRect = () => rect(0, VIEW) as DOMRect
  target.getBoundingClientRect = () => rect(AT - top, 30) as DOMRect
  return { aside, target }
}

beforeEach(() => {
  modes.reveal = null
  Object.defineProperty(window, 'innerWidth', { value: 1200, configurable: true })
  Object.defineProperty(window, 'innerHeight', { value: 3000, configurable: true })
})
afterEach(() => {
  spot.cancel()
  document.body.innerHTML = ''
  modes.reveal = null
})

describe('F1 «Показать»: подводка после роста раздела', () => {
  it('поле в видимой области после первого «Показать», хотя раздел ещё растёт', async () => {
    const m = mountPanel()
    modes.reveal = { name: 'settings', n: 1 } // панель деталей целит в верх раздела — подводка забирает прокрутку себе
    const found = await seek('ui:page-px')
    expect(found?.el).toBe(m.target)
    await later(GROW + 500)
    const r = m.target.getBoundingClientRect()
    expect(r.top).toBeGreaterThanOrEqual(0)
    expect(r.bottom).toBeLessThanOrEqual(VIEW)
    expect(modes.reveal).toBeNull()
  })

  it('колесо пользователя обрывает подводку', async () => {
    const m = mountPanel()
    await seek('ui:page-px')
    window.dispatchEvent(new Event('wheel'))
    m.aside.scrollTop = 0
    await later(GROW + 100)
    expect(m.aside.scrollTop).toBe(0)
  })

  it('щипок Mac (134) тоже обрывает подводку: gesturestart', async () => {
    const m = mountPanel()
    await seek('ui:page-px')
    window.dispatchEvent(new Event('gesturestart'))
    m.aside.scrollTop = 0
    await later(GROW + 100)
    expect(m.aside.scrollTop).toBe(0)
  })

  it('остановка подводки снимает слушатель gesturestart (тот же, что повесили)', async () => {
    const add = vi.spyOn(window, 'addEventListener')
    const del = vi.spyOn(window, 'removeEventListener')
    mountPanel()
    await seek('ui:page-px')
    const fn = add.mock.calls.find((c) => c[0] === 'gesturestart')?.[1]
    expect(fn).toBeTypeOf('function')
    window.dispatchEvent(new Event('wheel'))
    expect(del.mock.calls.some((c) => c[0] === 'gesturestart' && c[1] === fn)).toBe(true)
    vi.restoreAllMocks()
  })

  it('уже видимое поле не двигает панель', async () => {
    const m = mountPanel()
    m.target.getBoundingClientRect = () =>
      ({ top: 100, bottom: 130, left: 0, right: 200, width: 200, height: 30, x: 0, y: 100, toJSON: () => ({}) }) as DOMRect
    await seek('ui:page-px')
    await later(100)
    expect(m.aside.scrollTop).toBe(0)
  })
})
