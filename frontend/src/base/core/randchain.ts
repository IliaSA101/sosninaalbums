// Тесты-цепочки состояний (К3 ревизии тестов 24.10): случайная последовательность действий над хранилищем UI с семенем и
// инвариантами после каждого шага. Тесты на один шаг не видят утечек вида «тур открылся заново после закрытия» — они
// живут на стыке нескольких действий. Подключается только из *.chain.test.ts; в приложение не входит.

// Генератор mulberry32: одно семя — одна и та же цепочка на любой машине.
export function rng(seed: number): () => number {
  let a = seed >>> 0
  return () => {
    a = (a + 0x6d2b79f5) >>> 0
    let t = a
    t = Math.imul(t ^ (t >>> 15), t | 1)
    t ^= t + Math.imul(t ^ (t >>> 7), t | 61)
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296
  }
}

// «-short» фронта: SA_TEST_SHORT=1 (его ставит scripts/precheck.mjs) — 50 шагов вместо 200.
export const chainSteps = (full = 200, short = 50): number => (process.env.SA_TEST_SHORT ? short : full)

// Предел времени одной цепочки: 50 мс на шаг, не меньше 5 с, умноженный на SA_TEST_SLOW (как testkit.Limit в Go).
// Локально цепочка в 200 шагов идёт сотые доли секунды; запас большой, потому что раннер Windows медленнее в 2–4 раза.
export function chainTimeout(steps: number): number {
  const slow = Number(process.env.SA_TEST_SLOW)
  return Math.max(5000, steps * 50) * (Number.isFinite(slow) && slow >= 1 ? slow : 1)
}

// Отдать ход цепочкам промисов без реального ожидания: setTimeout(0) на Windows стоит ~15 мс, 600 таймеров — секунды.
export async function drain(rounds = 8): Promise<void> {
  for (let i = 0; i < rounds; i++) await Promise.resolve()
}

// Семена: три фиксированных; SA_CHAIN_SEED=<число> прогоняет одну цепочку (воспроизведение провала).
export function chainSeeds(): number[] {
  const one = Number(process.env.SA_CHAIN_SEED)
  return Number.isInteger(one) && one > 0 ? [one] : [20261024, 7, 424242]
}

export interface Rand {
  (): number
  int(n: number): number
  pick<T>(list: readonly T[]): T
  chance(p: number): boolean
}

export function rand(seed: number): Rand {
  const next = rng(seed)
  const r = (() => next()) as Rand
  r.int = (n) => Math.floor(next() * n)
  r.pick = (list) => list[Math.floor(next() * list.length)]
  r.chance = (p) => next() < p
  return r
}

export interface Chain<M> {
  // Действия: имя печатается в цепочке; вернуть можно описание с параметрами («select p1,p2»).
  actions: { name: string; run: (m: M, r: Rand) => string | void | Promise<string | void> }[]
  // Инварианты после каждого шага: текст нарушения или null.
  invariants: ((m: M) => string | null)[]
  // Дать хранилищу «устояться» после шага (эффекты Svelte, микрозадачи).
  settle?: () => void | Promise<void>
}

// Прогнать цепочку; при нарушении бросает ошибку с семенем и всей цепочкой, чтобы провал воспроизвёлся без догадок.
export async function runChain<M>(model: M, chain: Chain<M>, seed: number, steps: number): Promise<void> {
  const r = rand(seed)
  const done: string[] = []
  const fail = (what: string, cause?: unknown): never => {
    const tail = cause instanceof Error ? `\n${cause.stack ?? cause.message}` : ''
    throw new Error(
      `${what}\nсемя ${seed}, шаг ${done.length} из ${steps} (SA_CHAIN_SEED=${seed} повторит цепочку)\nцепочка: ${done.join(' → ')}${tail}`,
    )
  }
  for (let i = 0; i < steps; i++) {
    const a = r.pick(chain.actions)
    try {
      const d = await a.run(model, r)
      done.push(d ? `${a.name}(${d})` : a.name)
    } catch (e) {
      done.push(a.name)
      fail(`действие бросило исключение: ${e instanceof Error ? e.message : String(e)}`, e)
    }
    await chain.settle?.()
    for (const inv of chain.invariants) {
      const bad = inv(model)
      if (bad) fail(`инвариант нарушен: ${bad}`)
    }
  }
}
