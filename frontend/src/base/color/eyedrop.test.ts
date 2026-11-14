import { afterEach, describe, expect, it, vi } from 'vitest'
import { eyedrop } from './eyedrop.svelte'

afterEach(() => eyedrop.cancel())

describe('eyedrop: режим пипетки', () => {
  it('start включает режим, finish выключает и отдаёт цвет владельцу', () => {
    const done = vi.fn()
    const a = {}
    eyedrop.start(a, 's1', done)
    expect(eyedrop.on).toBe(true)
    expect(eyedrop.owner).toBe(a)
    eyedrop.finish('#112233')
    expect(done).toHaveBeenCalledExactlyOnceWith('#112233')
    expect(eyedrop.on).toBe(false)
  })

  it('новый start заменяет прежнего владельца; его cancel чужой режим не снимает', () => {
    const [a, b] = [{}, {}]
    const doneA = vi.fn()
    const doneB = vi.fn()
    eyedrop.start(a, 's1', doneA)
    eyedrop.start(b, 's2', doneB)
    eyedrop.cancel(a)
    expect(eyedrop.owner).toBe(b)
    eyedrop.finish('#abcdef')
    expect(doneA).not.toHaveBeenCalled()
    expect(doneB).toHaveBeenCalledExactlyOnceWith('#abcdef')
  })

  it('check: тот же лист — режим идёт; другой или никакого — снят', () => {
    const a = {}
    eyedrop.start(a, 's1', vi.fn())
    expect(eyedrop.sheet).toBe('s1')
    eyedrop.check('s1')
    expect(eyedrop.on).toBe(true)
    eyedrop.check('s2')
    expect(eyedrop.on).toBe(false)
    expect(eyedrop.sheet).toBe('')
    eyedrop.start(a, 's1', vi.fn())
    eyedrop.check(undefined)
    expect(eyedrop.on).toBe(false)
  })

  it('cancel без режима и finish без режима — тихо', () => {
    expect(() => eyedrop.cancel()).not.toThrow()
    expect(() => eyedrop.finish('#000000')).not.toThrow()
    expect(eyedrop.on).toBe(false)
  })

  it('cancel снимает режим, цвет не отдаётся', () => {
    const done = vi.fn()
    const a = {}
    eyedrop.start(a, 's1', done)
    eyedrop.cancel(a)
    eyedrop.finish('#111111')
    expect(done).not.toHaveBeenCalled()
  })
})
