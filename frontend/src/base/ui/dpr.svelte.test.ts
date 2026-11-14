import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushSync } from 'svelte'
import { screenDpr } from './dpr.svelte'

// Подменный matchMedia: слушатели каждого запроса; «смена масштаба» вызывает все живые.
type Mql = { query: string; listeners: Set<() => void> }
let mqls: Mql[]

beforeEach(() => {
  mqls = []
  vi.stubGlobal('devicePixelRatio', 1)
  vi.stubGlobal('matchMedia', (query: string) => {
    const m: Mql = { query, listeners: new Set() }
    mqls.push(m)
    return {
      addEventListener: (_: string, f: () => void) => m.listeners.add(f),
      removeEventListener: (_: string, f: () => void) => m.listeners.delete(f),
    }
  })
})
afterEach(() => vi.unstubAllGlobals())

const live = () => mqls.filter((m) => m.listeners.size > 0)
function setRatio(v: number) {
  vi.stubGlobal('devicePixelRatio', v)
  // как браузер: запрос на прежнее значение перестал совпадать
  for (const m of live()) for (const f of [...m.listeners]) f()
}

describe('screenDpr', () => {
  it('без подписчика читает текущее значение и ничего не вешает', () => {
    vi.stubGlobal('devicePixelRatio', 1.5)
    expect(screenDpr.value).toBe(1.5)
    expect(live()).toHaveLength(0)
  })

  it('нет devicePixelRatio — 1', () => {
    vi.stubGlobal('devicePixelRatio', 0)
    expect(screenDpr.value).toBe(1)
  })

  it('смена значения пересчитывает $derived, запрос перевешивается на новое', () => {
    const seen: number[] = []
    const stop = $effect.root(() => {
      const d = $derived(screenDpr.value * 100)
      $effect(() => {
        seen.push(d)
      })
    })
    flushSync()
    expect(seen).toEqual([100])
    expect(live().map((m) => m.query)).toEqual(['(resolution: 1dppx)'])

    setRatio(1.5)
    flushSync()
    expect(seen).toEqual([100, 150])
    expect(live().map((m) => m.query)).toEqual(['(resolution: 1.5dppx)'])
    stop()
  })

  it('два подписчика делят один слушатель; отписка снимает его', async () => {
    const a: number[] = []
    const b: number[] = []
    const stopA = $effect.root(() => {
      $effect(() => {
        a.push(screenDpr.value)
      })
    })
    const stopB = $effect.root(() => {
      $effect(() => {
        b.push(screenDpr.value)
      })
    })
    flushSync()
    expect(live()).toHaveLength(1)
    setRatio(2)
    flushSync()
    expect(a).toEqual([1, 2])
    expect(b).toEqual([1, 2])

    stopA()
    await Promise.resolve()
    expect(live()).toHaveLength(1) // B ещё читает
    stopB()
    await Promise.resolve() // createSubscriber снимает слушатель отложенно (микрозадача)
    expect(live()).toHaveLength(0)
  })

  it('после отписки смена масштаба никого не будит', async () => {
    const seen: number[] = []
    const stop = $effect.root(() => {
      $effect(() => {
        seen.push(screenDpr.value)
      })
    })
    flushSync()
    stop()
    await Promise.resolve()
    setRatio(2)
    flushSync()
    expect(seen).toEqual([1])
  })
})
