// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it } from 'vitest'
import { flushSync } from 'svelte'
import { CutKey } from './cutkey.svelte'

// Новый экземпляр на тест; сброс модульного признака «последний Ctrl» — blur окна, как у настоящего браузера.
let key: CutKey
let off: (() => void) | undefined
beforeEach(() => {
  window.dispatchEvent(new Event('blur'))
  key = new CutKey()
})
afterEach(() => {
  off?.()
  off = undefined
})

const send = (type: string, ctrlKey: boolean) => {
  window.dispatchEvent(new KeyboardEvent(type, { key: 'Control', ctrlKey }))
  flushSync()
}
const move = (ctrlKey: boolean) => {
  window.dispatchEvent(new MouseEvent('pointermove', { ctrlKey }))
  flushSync()
}

describe('CutKey: начальное состояние при attach (158, ревью)', () => {
  it('attach при зажатом Ctrl без движения мыши — сразу held', () => {
    send('keydown', true) // Ctrl нажат, пока слушателей attach ещё нет
    off = key.attach()
    expect(key.held).toBe(true)
  })

  it('Ctrl по последнему движению мыши — тоже сразу held', () => {
    move(true)
    off = key.attach()
    expect(key.held).toBe(true)
  })

  it('повторный attach: снят и подключён снова при зажатом Ctrl — held без движения', () => {
    off = key.attach()
    send('keydown', true)
    off()
    expect(key.held).toBe(false)
    off = key.attach()
    expect(key.held).toBe(true)
  })

  it('Ctrl отпущен до attach — held не залипает', () => {
    send('keydown', true)
    send('keyup', false)
    off = key.attach()
    expect(key.held).toBe(false)
  })

  it('Ctrl отпущен, пока attach снят — повторный attach стартует без held', () => {
    off = key.attach()
    send('keydown', true)
    off()
    send('keyup', false)
    off = key.attach()
    expect(key.held).toBe(false)
  })

  it('blur окна до attach сбрасывает признак', () => {
    send('keydown', true)
    window.dispatchEvent(new Event('blur'))
    off = key.attach()
    expect(key.held).toBe(false)
  })

  it('смена видимости при видимом окне признак не трогает; скрытое окно — сбрасывает', () => {
    send('keydown', true)
    document.dispatchEvent(new Event('visibilitychange'))
    off = key.attach()
    expect(key.held).toBe(true)
    off()
    Object.defineProperty(document, 'hidden', { configurable: true, get: () => true })
    try {
      document.dispatchEvent(new Event('visibilitychange'))
    } finally {
      delete (document as unknown as { hidden?: boolean }).hidden
    }
    off = key.attach()
    expect(key.held).toBe(false)
  })

  it('без Ctrl обычный attach — held false', () => {
    off = key.attach()
    expect(key.held).toBe(false)
  })
})
