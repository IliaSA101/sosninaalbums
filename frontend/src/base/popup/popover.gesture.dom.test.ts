// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, render } from '@testing-library/svelte'
import { createRawSnippet, flushSync } from 'svelte'
import Popover from './Popover.svelte'

// Окно панели и щипок Mac (134-8): gesturestart вне окна закрывает его, как колесо; внутри и по якорю — нет.
const body = createRawSnippet(() => ({ render: () => '<button class="in">внутри</button>' }))
let anchor: HTMLElement
let other: HTMLElement
let ro: typeof ResizeObserver

beforeEach(() => {
  ro = globalThis.ResizeObserver
  globalThis.ResizeObserver = class {
    observe() {}
    unobserve() {}
    disconnect() {}
  } as never
  anchor = document.body.appendChild(document.createElement('button'))
  other = document.body.appendChild(document.createElement('div'))
})
afterEach(() => {
  cleanup()
  anchor.remove()
  other.remove()
  globalThis.ResizeObserver = ro
})

const isOpen = () => !!document.querySelector('[data-barpop="t"]')
const gesture = (target: Element) => {
  const e = new Event('gesturestart', { bubbles: true, cancelable: true })
  target.dispatchEvent(e)
  flushSync()
  return e
}
const wheel = (target: Element) => {
  target.dispatchEvent(new WheelEvent('wheel', { bubbles: true, cancelable: true, deltaY: 10 }))
  flushSync()
}
const open = () => {
  render(Popover, { props: { anchor, open: true, label: 'Окно', kind: 't', children: body } })
  flushSync()
  expect(isOpen()).toBe(true)
}

describe('щипок и колесо вне окна панели', () => {
  it('gesturestart вне окна закрывает его', () => {
    open()
    gesture(other)
    expect(isOpen()).toBe(false)
  })

  it('gesturestart внутри окна и по якорю не закрывает', () => {
    open()
    gesture(document.querySelector('.in')!)
    expect(isOpen()).toBe(true)
    gesture(anchor)
    expect(isOpen()).toBe(true)
  })

  it('колесо вне окна по-прежнему закрывает', () => {
    open()
    wheel(other)
    expect(isOpen()).toBe(false)
  })

  it('закрытое окно жест не трогает (и не гасит)', () => {
    render(Popover, { props: { anchor, open: false, label: 'Окно', kind: 't', children: body } })
    flushSync()
    expect(gesture(other).defaultPrevented).toBe(false)
    expect(isOpen()).toBe(false)
  })

  it('после размонтирования снят тот же слушатель, что был повешен', () => {
    const add = vi.spyOn(window, 'addEventListener')
    const del = vi.spyOn(window, 'removeEventListener')
    open()
    const fn = add.mock.calls.find((c) => c[0] === 'gesturestart')?.[1]
    expect(fn).toBeTypeOf('function')
    cleanup()
    expect(del.mock.calls.some((c) => c[0] === 'gesturestart' && c[1] === fn)).toBe(true)
  })
})
