// @vitest-environment happy-dom
import { afterEach, describe, expect, it } from 'vitest'
import { cleanup, render } from '@testing-library/svelte'
import { createRawSnippet } from 'svelte'
import TplRow from './TplRow.svelte'

// Колесо над рядом шаблонов (134-8): простое листает вбок, Ctrl/⌘+колесо (и синтетическое колесо щипка) гасится и не листает.
afterEach(() => cleanup())

const body = createRawSnippet(() => ({ render: () => '<div class="item">x</div>' }))
function row() {
  const r = render(TplRow, { props: { count: 3, children: body } })
  return r.container.querySelector<HTMLElement>('.scroll')!
}
const wheel = (box: HTMLElement, init: { deltaY?: number; deltaX?: number; ctrlKey?: boolean; metaKey?: boolean }) => {
  const e = new WheelEvent('wheel', { bubbles: true, cancelable: true, deltaY: init.deltaY ?? 0, deltaX: init.deltaX ?? 0 })
  for (const k of ['ctrlKey', 'metaKey'] as const) Object.defineProperty(e, k, { value: !!init[k] })
  box.dispatchEvent(e)
  return e
}

describe('колесо над рядом шаблонов', () => {
  it('простое колесо листает вбок', () => {
    const box = row()
    box.scrollLeft = 0
    wheel(box, { deltaY: 50 })
    expect(box.scrollLeft).toBe(50)
  })

  it('Ctrl+колесо и ⌘+колесо гасятся и не листают', () => {
    const box = row()
    box.scrollLeft = 10
    for (const mod of [{ ctrlKey: true }, { metaKey: true }]) {
      const e = wheel(box, { deltaY: 50, ...mod })
      expect(e.defaultPrevented).toBe(true)
      expect(box.scrollLeft).toBe(10)
    }
  })
})
