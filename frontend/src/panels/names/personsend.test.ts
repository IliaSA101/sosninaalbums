import { beforeEach, describe, expect, it, vi } from 'vitest'

// Go и store подменены: проверяем порядок «набрано имя → edit.idle() → записано» (087-9).
vi.mock('../../base/core/store.svelte', () => ({
  store: { run: vi.fn(), spread: undefined, view: { spreads: [] }, selectedCell: null, fonts: [] },
  Api: {},
}))

import { edit } from '../../editor/text/edit.svelte'
import { PersonSend } from './personsend'

const log: string[] = []
let release: (() => void) | null = null
// Вызов Go, который завершается только по release() — «ещё в пути».
const slow = (name: string) => () =>
  new Promise<void>((r) => {
    log.push(name + ':start')
    release = () => {
      log.push(name + ':done')
      r()
    }
  })
const tick = () => new Promise((r) => setTimeout(r, 5))

beforeEach(() => {
  vi.stubGlobal('window', globalThis)
  log.length = 0
  release = null
})

describe('PersonSend + edit.idle (087-9)', () => {
  it('отложенное имя (дебаунс не вышел) досылается и ждётся', async () => {
    const s = new PersonSend()
    const off = edit.onIdle(() => void s.flush())
    s.schedule(() => slow('имя'), 500)
    expect(log).toEqual([])
    let idleDone = false
    const idle = edit.idle().then(() => (idleDone = true))
    await tick()
    expect(log).toEqual(['имя:start'])
    expect(idleDone, 'idle ждёт ответа Go').toBe(false)
    release?.()
    await idle
    expect(log).toEqual(['имя:start', 'имя:done'])
    off()
  })

  it('уже летящее имя (после ухода из поля) ждётся, даже если компонент размонтирован', async () => {
    const s = new PersonSend()
    void s.flush(() => slow('имя')) // blur: ушло в очередь, ответа нет; регистрации onIdle уже нет
    let idleDone = false
    const idle = edit.idle().then(() => (idleDone = true))
    await tick()
    expect(idleDone).toBe(false)
    release?.()
    await idle
    expect(idleDone).toBe(true)
  })

  it('два имени подряд идут по очереди, idle ждёт оба', async () => {
    const s = new PersonSend()
    void s.flush(() => slow('а'))
    void s.flush(() => slow('б'))
    const idle = edit.idle()
    await tick()
    expect(log).toEqual(['а:start'])
    release?.()
    await tick()
    expect(log).toEqual(['а:start', 'а:done', 'б:start'])
    release?.()
    await idle
    expect(log.at(-1)).toBe('б:done')
  })

  it('нечего отправлять — idle не зависает; отписанный отправитель не досылается', async () => {
    const s = new PersonSend()
    const off = edit.onIdle(() => void s.flush())
    s.schedule(() => null, 500)
    await edit.idle()
    expect(log).toEqual([])
    off()
    s.schedule(() => slow('поздно'), 500)
    await edit.idle()
    expect(log).toEqual([])
    void s.flush(() => null) // погасить таймер
  })

  it('сбой прежнего вызова не рвёт очередь', async () => {
    const s = new PersonSend()
    void s.flush(() => () => Promise.reject(new Error('x')))
    void s.flush(() => slow('после'))
    const idle = edit.idle()
    await tick()
    expect(log).toEqual(['после:start'])
    release?.()
    await idle
  })
})
