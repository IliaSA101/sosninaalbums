import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

// Тост «до фокуса» (150, Toasts.hold) и разбор текста по путям (toastParts) отдельно от агентов.
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', () => ({}))
vi.stubGlobal('window', globalThis)
let focused = true
const focusFns = new Set<() => void>()
vi.stubGlobal('document', { hasFocus: () => focused })
vi.stubGlobal('addEventListener', (n: string, cb: () => void) => {
  if (n === 'focus') focusFns.add(cb)
})
vi.stubGlobal('removeEventListener', (n: string, cb: () => void) => {
  if (n === 'focus') focusFns.delete(cb)
})
const gainFocus = () => {
  focused = true
  for (const f of [...focusFns]) f()
}

const { Toasts, HOLD_AFTER_MS, toastParts } = await import('./toasts.svelte')

beforeEach(() => {
  vi.useFakeTimers()
  focused = true
  focusFns.clear()
})
afterEach(() => vi.useRealTimers())

describe('Toasts.hold', () => {
  it('окно в фокусе: HOLD_AFTER_MS и тост уходит', () => {
    const t = new Toasts()
    t.hold('правка')
    expect(t.toast?.hold).toBe(true)
    vi.advanceTimersByTime(HOLD_AFTER_MS - 1)
    expect(t.toast).not.toBeNull()
    vi.advanceTimersByTime(1)
    expect(t.toast).toBeNull()
  })

  it('окно без фокуса: тост не уходит по времени, фокус запускает HOLD_AFTER_MS, слушатель снимается', () => {
    focused = false
    const t = new Toasts()
    t.hold('правка')
    vi.advanceTimersByTime(60 * 60_000)
    expect(t.toast?.text).toBe('правка')
    expect(focusFns.size).toBe(1)
    gainFocus()
    expect(focusFns.size).toBe(0)
    vi.advanceTimersByTime(HOLD_AFTER_MS)
    expect(t.toast).toBeNull()
  })

  it('повторный hold не плодит слушателей', () => {
    focused = false
    const t = new Toasts()
    t.hold('a')
    t.hold('a. b')
    t.hold('a. b. c')
    expect(focusFns.size).toBe(1)
    expect(t.toast?.text).toBe('a. b. c')
  })

  it('обычный тост заменяет hold: слушатель снят, старый таймер новый тост не гасит', () => {
    focused = false
    const t = new Toasts()
    t.hold('правка')
    t.notify('другое')
    expect(focusFns.size).toBe(0)
    gainFocus() // фокус пришёл: ожидание hold уже снято и 8 с от него не пойдут
    vi.advanceTimersByTime(3999)
    expect(t.toast?.text).toBe('другое')
    vi.advanceTimersByTime(1) // уходит по своим 4 с
    expect(t.toast).toBeNull()
  })

  it('hold после обычного тоста: его 4-секундный таймер не гасит hold', () => {
    const t = new Toasts()
    t.notify('сначала')
    t.hold('правка')
    vi.advanceTimersByTime(5000)
    expect(t.toast?.text).toBe('правка')
  })

  it('действие, rev и пути хранятся в тосте', () => {
    const t = new Toasts()
    const run = vi.fn()
    t.hold('x', { action: { label: 'Вернуть', run, undo: true }, rev: 4, paths: ['D:\\a'] })
    expect(t.toast).toMatchObject({ hold: true, rev: 4, paths: ['D:\\a'], kind: 'info' })
    expect(t.toast?.action?.label).toBe('Вернуть')
  })

  it('тост закрыт вручную до фокуса — фокус ничего не включает', () => {
    focused = false
    const t = new Toasts()
    t.hold('правка')
    t.toast = null
    gainFocus()
    vi.advanceTimersByTime(HOLD_AFTER_MS)
    expect(t.toast).toBeNull()
  })
})

describe('toastParts', () => {
  it('без путей — один кусок текста; пустой текст — пусто', () => {
    expect(toastParts('простой', undefined)).toEqual([{ t: 'простой', path: false }])
    expect(toastParts('простой', [])).toEqual([{ t: 'простой', path: false }])
    expect(toastParts('', ['D:\\a'])).toEqual([])
  })

  it('путь вырезается отдельным куском, остальное сохраняется', () => {
    expect(toastParts('Клиент · Добавлена папка D:\\a. Готово', ['D:\\a'])).toEqual([
      { t: 'Клиент · Добавлена папка ', path: false },
      { t: 'D:\\a', path: true },
      { t: '. Готово', path: false },
    ])
  })

  it('несколько путей и повтор одного; пустой путь в списке игнорируется', () => {
    expect(toastParts('D:\\a и E:\\b и D:\\a', ['E:\\b', 'D:\\a', ''])).toEqual([
      { t: 'D:\\a', path: true },
      { t: ' и ', path: false },
      { t: 'E:\\b', path: true },
      { t: ' и ', path: false },
      { t: 'D:\\a', path: true },
    ])
  })

  it('один путь — начало другого: берётся длиннейший', () => {
    expect(toastParts('D:\\Фото\\2026', ['D:\\Фото', 'D:\\Фото\\2026'])).toEqual([{ t: 'D:\\Фото\\2026', path: true }])
  })

  it('кусочки склеиваются обратно в исходный текст', () => {
    const text = 'a D:\\x b D:\\y c'
    expect(
      toastParts(text, ['D:\\x', 'D:\\y'])
        .map((p) => p.t)
        .join(''),
    ).toBe(text)
  })
})
