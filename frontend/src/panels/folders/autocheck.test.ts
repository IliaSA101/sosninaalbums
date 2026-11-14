import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', () => ({ CheckFolders: vi.fn(async () => {}) }))
vi.stubGlobal('window', globalThis)

const Go = await import('../../../wailsjs/go/app/App')
const { AutoCheck, checkFolders } = await import('./autocheck')

// Подставные окно и документ: события шлём руками, видимость и фокус — поля.
function fakes() {
  const win = new EventTarget()
  const doc = Object.assign(new EventTarget(), {
    visibilityState: 'visible',
    focused: true,
    hasFocus() {
      return this.focused
    },
  })
  return { win, doc }
}

describe('AutoCheck', () => {
  let calls: boolean[]
  let busy: boolean
  let ac: InstanceType<typeof AutoCheck>
  let f: ReturnType<typeof fakes>
  beforeEach(() => {
    vi.useFakeTimers()
    calls = []
    busy = false
    ac = new AutoCheck({
      call: (full) => (calls.push(full), Promise.resolve()),
      busy: () => busy,
    })
    f = fakes()
    ac.start(f.win as unknown as Window, f.doc as unknown as Document)
  })
  afterEach(() => {
    ac.stop()
    vi.useRealTimers()
  })

  it('фокус окна — проверка full', () => {
    f.win.dispatchEvent(new Event('focus'))
    expect(calls).toEqual([true])
  })

  it('окно снова видно — full; скрылось — ничего', () => {
    f.doc.visibilityState = 'hidden'
    f.doc.dispatchEvent(new Event('visibilitychange'))
    expect(calls).toEqual([])
    f.doc.visibilityState = 'visible'
    f.doc.dispatchEvent(new Event('visibilitychange'))
    expect(calls).toEqual([true])
  })

  it('таймер 10 с — обычная проверка, только при фокусе', () => {
    vi.advanceTimersByTime(9_999)
    expect(calls).toEqual([])
    vi.advanceTimersByTime(1)
    expect(calls).toEqual([false])
    f.doc.focused = false
    vi.advanceTimersByTime(10_000)
    expect(calls).toEqual([false])
  })

  it('скрытое окно — таймер стоит', () => {
    f.doc.visibilityState = 'hidden'
    vi.advanceTimersByTime(30_000)
    expect(calls).toEqual([])
  })

  it('во время переноса — пропуск', () => {
    busy = true
    f.win.dispatchEvent(new Event('focus'))
    vi.advanceTimersByTime(10_000)
    ac.soon()
    vi.advanceTimersByTime(300)
    expect(calls).toEqual([])
  })

  it('ошибки миниатюр — один вызов за 300 мс', () => {
    ac.soon()
    ac.soon()
    ac.soon()
    vi.advanceTimersByTime(299)
    expect(calls).toEqual([])
    vi.advanceTimersByTime(1)
    expect(calls).toEqual([false])
    ac.soon()
    vi.advanceTimersByTime(300)
    expect(calls).toEqual([false, false])
  })

  it('soon(true) в пачке делает вызов полным', () => {
    ac.soon()
    ac.soon(true)
    vi.advanceTimersByTime(300)
    expect(calls).toEqual([true])
  })

  it('stop снимает слушатели, таймер и отложенный вызов', () => {
    ac.soon()
    ac.stop()
    f.win.dispatchEvent(new Event('focus'))
    f.doc.dispatchEvent(new Event('visibilitychange'))
    vi.advanceTimersByTime(30_000)
    expect(calls).toEqual([])
  })

  it('повторный start не удваивает слушатели', () => {
    ac.start(f.win as unknown as Window, f.doc as unknown as Document)
    f.win.dispatchEvent(new Event('focus'))
    expect(calls).toEqual([true])
  })

  it('onDone: после удачной сверки зовётся, после сбоя — нет, во время переноса — нет', async () => {
    const done = vi.fn()
    ac.onDone(done)
    f.win.dispatchEvent(new Event('focus'))
    await Promise.resolve()
    await Promise.resolve()
    expect(done).toHaveBeenCalledTimes(1)
    busy = true
    f.win.dispatchEvent(new Event('focus'))
    await Promise.resolve()
    expect(done).toHaveBeenCalledTimes(1)
    const bad = new AutoCheck({ call: () => Promise.reject(new Error('нет Go')), busy: () => false })
    const failed = vi.fn()
    bad.onDone(failed)
    const w = fakes()
    bad.start(w.win as unknown as Window, w.doc as unknown as Document)
    w.win.dispatchEvent(new Event('focus'))
    await Promise.resolve()
    await Promise.resolve()
    expect(failed).not.toHaveBeenCalled()
    bad.stop()
  })

  it('сбой вызова не всплывает', async () => {
    const bad = new AutoCheck({ call: () => Promise.reject(new Error('нет Go')), busy: () => false })
    const w = fakes()
    bad.start(w.win as unknown as Window, w.doc as unknown as Document)
    w.win.dispatchEvent(new Event('focus'))
    await Promise.resolve()
    bad.stop()
  })

  it('checkFolders зовёт Go-метод CheckFolders с тем же full', async () => {
    await checkFolders(true)
    expect(Go.CheckFolders).toHaveBeenLastCalledWith(true)
    await checkFolders(false)
    expect(Go.CheckFolders).toHaveBeenLastCalledWith(false)
  })
})
