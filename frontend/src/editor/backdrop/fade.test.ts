import { describe, expect, it } from 'vitest'
import { fadePct, opacityOf, pctFade } from './fade'
import { fadeLive } from './fade.svelte'

describe('fadePct / pctFade', () => {
  it('доля ↔ проценты', () => {
    expect(fadePct(0.4)).toBe(40)
    expect(pctFade(40)).toBe(0.4)
    expect(fadePct(0.07)).toBe(7)
    expect(pctFade(7)).toBe(0.07)
  })
  it('края и зажим', () => {
    expect(fadePct(0)).toBe(0)
    expect(fadePct(1)).toBe(100)
    expect(fadePct(2)).toBe(100)
    expect(fadePct(-1)).toBe(0)
    expect(pctFade(100)).toBe(1)
    expect(pctFade(150)).toBe(1)
    expect(pctFade(-5)).toBe(0)
  })
  it('мусор — 0', () => {
    expect(fadePct(NaN)).toBe(0)
    expect(pctFade(NaN)).toBe(0)
    expect(pctFade(Infinity)).toBe(0)
  })
  it('дробные проценты округляются до целых', () => {
    expect(pctFade(40.6)).toBe(0.41)
  })
})

describe('opacityOf', () => {
  it('0 % — без opacity', () => expect(opacityOf(0)).toBeUndefined())
  it('1 − fade', () => {
    expect(opacityOf(0.4)).toBe(0.6)
    expect(opacityOf(1)).toBe(0)
  })
})

describe('fadeLive', () => {
  it('просмотр действует только на свою область', () => {
    fadeLive.set('s1', 'left', 0.5)
    expect(fadeLive.of('s1', 'left', 0.1)).toBe(0.5)
    expect(fadeLive.of('s1', 'right', 0.1)).toBe(0.1)
    expect(fadeLive.of('s2', 'left', 0.1)).toBe(0.1)
    fadeLive.clear()
    expect(fadeLive.of('s1', 'left', 0.1)).toBe(0.1)
  })
})

describe('fadeLive: очередь коммитов', () => {
  it('коммиты идут по порядку, просмотр снимается после последнего', async () => {
    const log: string[] = []
    const slow = (n: string, ms: number) => () =>
      new Promise((r) =>
        setTimeout(() => {
          log.push(n)
          r(null)
        }, ms),
      )
    fadeLive.set('s1', 'left', 0.5)
    const a = fadeLive.commit(slow('a', 20))
    const b = fadeLive.commit(slow('b', 1))
    await a
    expect(fadeLive.cur).not.toBeNull()
    await b
    expect(log).toEqual(['a', 'b'])
    expect(fadeLive.cur).toBeNull()
  })
  it('release не снимает просмотр при коммите в пути', async () => {
    fadeLive.set('s1', 'left', 0.5)
    const p = fadeLive.commit(() => Promise.resolve())
    fadeLive.release()
    expect(fadeLive.cur).not.toBeNull()
    await p
    expect(fadeLive.cur).toBeNull()
    fadeLive.set('s1', 'left', 0.5)
    fadeLive.release()
    expect(fadeLive.cur).toBeNull()
  })
  it('ошибка коммита не рвёт очередь', async () => {
    fadeLive.set('s1', 'left', 0.5)
    const bad = fadeLive.commit(() => Promise.reject(new Error('x')))
    const ok = fadeLive.commit(() => Promise.resolve())
    await Promise.all([bad, ok])
    expect(fadeLive.cur).toBeNull()
  })
})
