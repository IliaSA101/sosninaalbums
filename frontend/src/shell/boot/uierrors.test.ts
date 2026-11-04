import { describe, expect, it, vi } from 'vitest'
import { installUiErrors } from './uierrors'

// Окно-заглушка: события шлём руками, как это делает браузер.
function fakeWindow() {
  const h = new Map<string, (e: Event) => void>()
  return {
    addEventListener: (t: string, f: (e: Event) => void) => void h.set(t, f),
    removeEventListener: (t: string) => void h.delete(t),
    fire: (t: string, e: object) => h.get(t)?.(e as Event),
    has: (t: string) => h.has(t),
  }
}

const ev = (message: string, extra: object = {}) => ({ message, filename: '', lineno: 0, colno: 0, ...extra })

function setup(send = vi.fn((_t: string) => Promise.resolve())) {
  const w = fakeWindow()
  const off = installUiErrors(w as unknown as Window, send)
  return { w, send, off }
}

describe('uierrors', () => {
  it('error шлёт сообщение, место и начало стека', () => {
    const { w, send } = setup()
    const error = new Error('сломалось')
    error.stack = Array.from({ length: 8 }, (_, i) => `строка${i}`).join('\n')
    w.fire('error', ev('сломалось', { filename: 'http://x/app.js', lineno: 7, colno: 3, error }))
    expect(send).toHaveBeenCalledTimes(1)
    const text = send.mock.calls[0][0]
    expect(text.startsWith('сломалось @ http://x/app.js:7:3\n')).toBe(true)
    expect(text).toContain('строка4')
    expect(text).not.toContain('строка5')
  })

  it('error без файла и без объекта ошибки: только сообщение', () => {
    const { w, send } = setup()
    w.fire('error', ev('Script error.', { error: null }))
    expect(send).toHaveBeenCalledWith('Script error.')
  })

  it('unhandledrejection: Error и строка', () => {
    const { w, send } = setup()
    const err = new Error('нет сети')
    err.stack = 'Error: нет сети\n  at f'
    w.fire('unhandledrejection', { reason: err })
    w.fire('unhandledrejection', { reason: 'просто текст' })
    expect(send.mock.calls[0][0]).toBe('отказ: нет сети\nError: нет сети\n  at f')
    expect(send.mock.calls[1][0]).toBe('отказ: просто текст')
  })

  it('ResizeObserver loop не шлётся', () => {
    const { w, send } = setup()
    w.fire('error', ev('ResizeObserver loop completed with undelivered notifications.'))
    w.fire('error', ev('ResizeObserver loop limit exceeded'))
    expect(send).not.toHaveBeenCalled()
  })

  it('сбой LogUI не зовёт LogUI снова', async () => {
    // Отказ промиса — молча, одна попытка.
    const { w, send } = setup(vi.fn(() => Promise.reject(new Error('go упал'))))
    w.fire('error', ev('a'))
    await Promise.resolve()
    expect(send).toHaveBeenCalledTimes(1)
  })

  it('нет Go (LogUI бросает синхронно): тихо, без исключения', () => {
    const { w, send } = setup(
      vi.fn(() => {
        throw new TypeError("Cannot read properties of undefined (reading 'go')")
      }),
    )
    expect(() => w.fire('error', ev('b'))).not.toThrow()
    expect(send).toHaveBeenCalledTimes(1)
  })

  it('ошибка, всплывшая во время отправки, не шлётся повторно', () => {
    const w = fakeWindow()
    const send = vi.fn((_t: string) => {
      w.fire('error', ev('вложенная'))
      return Promise.resolve()
    })
    installUiErrors(w as unknown as Window, send)
    w.fire('error', ev('первая'))
    expect(send).toHaveBeenCalledTimes(1)
    // после отправки обработчик снова работает
    w.fire('error', ev('вторая'))
    expect(send).toHaveBeenCalledTimes(2)
  })

  it('длиннее 2000 знаков режется до отправки', () => {
    const { w, send } = setup()
    w.fire('error', ev('я'.repeat(5000)))
    w.fire('unhandledrejection', { reason: 'я'.repeat(5000) })
    expect(send.mock.calls[0][0].length).toBe(2000)
    expect(send.mock.calls[1][0].length).toBe(2000)
  })

  it('функция снятия убирает оба слушателя', () => {
    const { w, off } = setup()
    expect(w.has('error') && w.has('unhandledrejection')).toBe(true)
    off()
    expect(w.has('error') || w.has('unhandledrejection')).toBe(false)
  })
})
