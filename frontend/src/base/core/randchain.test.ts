import { afterEach, describe, expect, it, vi } from 'vitest'
import { chainSeeds, chainSteps, chainTimeout, drain, rand, rng, runChain, type Chain } from './randchain'

afterEach(() => vi.unstubAllEnvs())

describe('randchain: помощник цепочек состояний', () => {
  it('одно семя — одна последовательность, разные семена — разные', () => {
    const a = rng(42)
    const b = rng(42)
    const xs = Array.from({ length: 5 }, () => a())
    expect(xs).toEqual(Array.from({ length: 5 }, () => b()))
    expect(xs.every((x) => x >= 0 && x < 1)).toBe(true)
    expect(rng(43)()).not.toBe(xs[0])
  })

  it('rand: int в границах, pick из списка, chance 0 и 1', () => {
    const r = rand(1)
    for (let i = 0; i < 100; i++) {
      expect(r.int(3)).toBeGreaterThanOrEqual(0)
      expect(r.int(3)).toBeLessThan(3)
      expect(['a', 'b']).toContain(r.pick(['a', 'b']))
    }
    expect(r.chance(0)).toBe(false)
    expect(r.chance(1)).toBe(true)
  })

  it('«-short»: SA_TEST_SHORT=1 даёт 50 шагов вместо 200', () => {
    vi.stubEnv('SA_TEST_SHORT', '')
    expect(chainSteps()).toBe(200)
    vi.stubEnv('SA_TEST_SHORT', '1')
    expect(chainSteps()).toBe(50)
    expect(chainSteps(10, 3)).toBe(3)
  })

  it('предел цепочки: 50 мс на шаг, не меньше 5 с, умножается на SA_TEST_SLOW', () => {
    vi.stubEnv('SA_TEST_SLOW', '')
    expect(chainTimeout(50)).toBe(5000)
    expect(chainTimeout(200)).toBe(10000)
    vi.stubEnv('SA_TEST_SLOW', '4')
    expect(chainTimeout(200)).toBe(40000)
    vi.stubEnv('SA_TEST_SLOW', 'abc')
    expect(chainTimeout(200)).toBe(10000)
    vi.stubEnv('SA_TEST_SLOW', '0.5')
    expect(chainTimeout(200)).toBe(10000)
  })

  it('drain отдаёт ход цепочке промисов без таймеров', async () => {
    let n = 0
    void Promise.resolve()
      .then(() => n++)
      .then(() => n++)
      .then(() => n++)
    await drain()
    expect(n).toBe(3)
  })

  it('семена: три по умолчанию, SA_CHAIN_SEED оставляет одно', () => {
    vi.stubEnv('SA_CHAIN_SEED', '')
    expect(chainSeeds()).toHaveLength(3)
    vi.stubEnv('SA_CHAIN_SEED', '77')
    expect(chainSeeds()).toEqual([77])
    vi.stubEnv('SA_CHAIN_SEED', 'abc')
    expect(chainSeeds()).toHaveLength(3)
  })

  it('провал печатает семя, номер шага и всю цепочку', async () => {
    const chain: Chain<{ n: number }> = {
      actions: [
        {
          name: 'inc',
          run: (m) => {
            m.n++
            return String(m.n)
          },
        },
      ],
      invariants: [(m) => (m.n > 2 ? `n=${m.n}` : null)],
    }
    await expect(runChain({ n: 0 }, chain, 9, 10)).rejects.toThrow(
      /инвариант нарушен: n=3[\s\S]*семя 9, шаг 3 из 10[\s\S]*inc\(1\) → inc\(2\) → inc\(3\)/,
    )
  })

  it('исключение действия тоже печатает семя и цепочку', async () => {
    const chain: Chain<object> = {
      actions: [
        {
          name: 'boom',
          run: () => {
            throw new Error('упало')
          },
        },
      ],
      invariants: [],
    }
    await expect(runChain({}, chain, 5, 3)).rejects.toThrow(/упало[\s\S]*семя 5[\s\S]*цепочка: boom/)
  })

  it('прогон без нарушений проходит все шаги и вызывает settle', async () => {
    const settle = vi.fn()
    const seen: string[] = []
    const chain: Chain<null> = {
      actions: [
        { name: 'a', run: () => void seen.push('a') },
        { name: 'b', run: () => void seen.push('b') },
      ],
      invariants: [],
      settle,
    }
    await runChain(null, chain, 3, 20)
    expect(seen).toHaveLength(20)
    expect(settle).toHaveBeenCalledTimes(20)
    expect(new Set(seen).size).toBe(2)
  })
})
