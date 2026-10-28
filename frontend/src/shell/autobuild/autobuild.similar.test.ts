import { describe, expect, it, vi } from 'vitest'
import type { AutoPlan, PhotoOpts, SigState, SimilarDrop } from '../../base/core/types'
import { AutoDialog, type PlanFn } from './autobuild.svelte'
import type { DropsFn, SigsFn } from './autosimilar.svelte'

// 078: «Убирать похожие кадры» — подписи, «Собрать» ждёт, ряд отброшенных и «Вернуть». Go подменён.
const ids = Array.from({ length: 20 }, (_, i) => `p${i}`)
const plan = (p: Partial<AutoPlan> = {}): AutoPlan => ({
  photos: 20,
  spreads: 5,
  minSpreads: 1,
  maxSpreads: 20,
  avg: 4,
  min: 3,
  max: 5,
  reuse: 0,
  from: 3,
  to: 5,
  cuts: 0,
  ...p,
})
const settle = () => new Promise((r) => setTimeout(r, 0))
const drop = (id: string, like = 'p0'): SimilarDrop => ({ id, like, dist: 5 })

interface Over {
  sigs?: SigsFn | null
  drops?: DropsFn | null
  saved?: { similar?: number; gray?: number }
  similarCount?: number
}
function make(o: Over = {}) {
  const asked: PhotoOpts[] = []
  const planFn: PlanFn = async (_i, spreads, _r, ph) => {
    asked.push(ph)
    return plan({ spreads, similar: o.similarCount })
  }
  const sigs = vi.fn<SigsFn>(
    o.sigs === undefined ? async () => ({ done: 20, total: 20 }) : (o.sigs ?? (async () => ({ done: 0, total: 0 }))),
  )
  const d = new AutoDialog(ids, planFn, { spreads: null, ...o.saved }, null, {
    sigs: o.sigs === null ? null : sigs,
    drops: o.drops ?? null,
  })
  return { d, asked, sigs }
}

describe('блок «Фото»: по умолчанию как 0.19', () => {
  it('«Нет»: в Go нули, подписи не готовятся, план сразу', async () => {
    const { d, asked, sigs } = make()
    d.start()
    await settle()
    expect(asked).toEqual([{ similar: 0, keep: [], gray: 0, variant: 1, again: false }])
    expect(sigs).not.toHaveBeenCalled()
    expect(d.canBuild).toBe(true)
    expect(d.ph.waiting).toBe(false)
  })

  it('память окна: уровень доходит до Go, мусор — «Нет»', async () => {
    const { d, asked } = make({ saved: { similar: 2, gray: 9 } })
    d.start()
    await settle()
    expect(asked[0]).toMatchObject({ similar: 2, gray: 0 })
  })
})

describe('подписи кадров (PrepareSigs, sigs:progress)', () => {
  it('подписи готовы сразу: один вопрос плана, не два', async () => {
    const { d, asked, sigs } = make({ saved: { similar: 1 } })
    d.start()
    await settle()
    expect(sigs).toHaveBeenCalledWith(ids)
    expect(asked).toHaveLength(1)
    expect(d.canBuild).toBe(true)
  })

  it('идёт подготовка: «Собрать» ждёт, план не спрашивается; done = total — план и сборка доступны', async () => {
    const { d, asked } = make({ saved: { similar: 1 }, sigs: async () => ({ done: 5, total: 20 }) })
    d.start()
    await settle()
    expect(d.ph.waiting).toBe(true)
    expect(d.canBuild).toBe(false)
    expect(asked).toHaveLength(0)
    d.ph.onProgress({ done: 12, total: 20 })
    expect(d.ph.sigs).toEqual({ done: 12, total: 20 })
    expect(d.canBuild).toBe(false)
    d.ph.onProgress({ done: 20, total: 20 })
    await settle()
    expect(d.ph.waiting).toBe(false)
    expect(asked).toHaveLength(1)
    expect(d.canBuild).toBe(true)
  })

  it('событие обогнало ответ: готовое не откатывается', async () => {
    let res: (s: SigState) => void = () => {}
    const { d } = make({ saved: { similar: 1 }, sigs: () => new Promise((r) => (res = r)) })
    d.start()
    d.ph.onProgress({ done: 18, total: 20 })
    res({ done: 3, total: 20 })
    await settle()
    expect(d.ph.sigs).toEqual({ done: 18, total: 20 })
  })

  it('включили «Похожие» уже в окне: подписи готовятся один раз, смена уровня их не повторяет', async () => {
    const { d, asked, sigs } = make()
    d.start()
    await settle()
    d.ph.setSimilar(1)
    await settle()
    d.ph.setSimilar(2)
    await settle()
    expect(sigs).toHaveBeenCalledTimes(1)
    expect(asked.map((a) => a.similar)).toEqual([0, 1, 2])
  })

  it('«Нет» после «Похожих»: ждать нечего, план без подписей', async () => {
    const { d, asked } = make({ saved: { similar: 1 }, sigs: async () => ({ done: 1, total: 20 }) })
    d.start()
    await settle()
    expect(d.ph.waiting).toBe(true)
    d.ph.setSimilar(0)
    await settle()
    expect(d.ph.waiting).toBe(false)
    expect(asked.at(-1)?.similar).toBe(0)
    expect(d.canBuild).toBe(true)
  })

  it('закрыли окно: PrepareSigs([]) один раз, поздние события молчат', async () => {
    const { d, sigs } = make({ saved: { similar: 1 }, sigs: async () => ({ done: 1, total: 20 }) })
    d.start()
    await settle()
    d.dispose()
    d.dispose()
    expect(sigs.mock.calls.map((c) => c[0].length)).toEqual([20, 0])
    d.ph.onProgress({ done: 20, total: 20 })
    expect(d.ph.sigs).toEqual({ done: 1, total: 20 })
  })

  it('окно без подписей не отменяет то, чего не просило', async () => {
    const { d, sigs } = make()
    d.start()
    await settle()
    d.dispose()
    expect(sigs).not.toHaveBeenCalled()
  })

  it('PrepareSigs упал: «Собрать» не ждёт вечно', async () => {
    const { d } = make({ saved: { similar: 3 }, sigs: async () => Promise.reject(new Error('нет')) })
    d.start()
    await settle()
    expect(d.ph.waiting).toBe(false)
    expect(d.canBuild).toBe(true)
  })

  it('пустой набор: total = 0 — готово', async () => {
    const { d } = make({ saved: { similar: 1 }, sigs: async () => ({ done: 0, total: 0 }) })
    d.start()
    await settle()
    expect(d.ph.waiting).toBe(false)
  })
})

describe('«Вернуть»: keep уходит в план и сборку', () => {
  it('щелчок возвращает, повторный — убирает снова; план спрашивается заново', async () => {
    const { d, asked } = make({ saved: { similar: 1 } })
    d.start()
    await settle()
    d.ph.toggleKeep('p3')
    await settle()
    expect(asked.at(-1)?.keep).toEqual(['p3'])
    d.ph.toggleKeep('p3')
    await settle()
    expect(asked.at(-1)?.keep).toEqual([])
    expect(d.ph.keep.size).toBe(0)
  })

  it('к «Собрать» готовы фото-поля и число не взятых похожих из плана', async () => {
    const { d } = make({ saved: { similar: 2 }, similarCount: 7 })
    d.start()
    await settle()
    d.ph.toggleKeep('p1')
    await settle()
    expect(await d.submit(vi.fn())).toBe(true)
    expect(d.ph.opts).toEqual({ similar: 2, keep: ['p1'], gray: 0, variant: 1, again: false })
    expect(d.plan?.similar).toBe(7)
  })
})

describe('ряд отброшенных (AutoSimilar)', () => {
  const rows = (list: SimilarDrop[]) => vi.fn<DropsFn>(async () => list)

  it('спрашивается при раскрытии и только при «Похожих»', async () => {
    const drops = rows([drop('p2'), drop('p5')])
    const { d } = make({ saved: { similar: 1 }, drops })
    d.start()
    await settle()
    expect(drops).not.toHaveBeenCalled() // ряд свёрнут
    d.ph.setDropsOpen(true)
    await settle()
    expect(drops).toHaveBeenCalledTimes(1)
    expect(drops.mock.calls[0][2]).toMatchObject({ similar: 1, keep: [] })
    expect(d.ph.drops?.map((x) => x.id)).toEqual(['p2', 'p5'])
  })

  it('«Нет» — ряд не спрашивается и очищается', async () => {
    const drops = rows([drop('p2')])
    const { d } = make({ drops })
    d.start()
    await settle()
    d.ph.setDropsOpen(true)
    await settle()
    expect(drops).not.toHaveBeenCalled()
    d.ph.setSimilar(1)
    await settle()
    expect(d.ph.drops).toHaveLength(1)
    d.ph.setSimilar(0)
    await settle()
    expect(d.ph.drops).toBeNull()
  })

  it('подписи не готовы — ряд ждёт, потом спрашивает', async () => {
    const drops = rows([drop('p2')])
    const { d } = make({ saved: { similar: 1 }, drops, sigs: async () => ({ done: 0, total: 20 }) })
    d.start()
    d.ph.setDropsOpen(true)
    await settle()
    expect(drops).not.toHaveBeenCalled()
    d.ph.onProgress({ done: 20, total: 20 })
    await settle()
    expect(drops).toHaveBeenCalledTimes(1)
  })

  it('вернули плитку — Go её уже не отдаёт, а в ряду она остаётся «вернётся»; порядок ленты', async () => {
    const drops = vi.fn<DropsFn>(async (_i, _r, ph) => [drop('p9'), drop('p2'), drop('p5')].filter((x) => !ph.keep.includes(x.id)))
    const { d } = make({ saved: { similar: 1 }, drops })
    d.start()
    d.ph.setDropsOpen(true)
    await settle()
    expect(d.ph.drops?.map((x) => x.id)).toEqual(['p2', 'p5', 'p9'])
    d.ph.toggleKeep('p5')
    await settle()
    expect(drops.mock.calls.at(-1)?.[2].keep).toEqual(['p5'])
    expect(d.ph.drops?.map((x) => x.id)).toEqual(['p2', 'p5', 'p9'])
    expect(d.ph.keep.has('p5')).toBe(true)
    d.ph.toggleKeep('p5') // убрали снова: Go отдаёт её опять
    await settle()
    expect(d.ph.drops?.map((x) => x.id)).toEqual(['p2', 'p5', 'p9'])
  })

  it('«Вернуть все» возвращает всё показанное одним запросом плана', async () => {
    const drops = rows([drop('p2'), drop('p5'), drop('p9')])
    const { d, asked } = make({ saved: { similar: 1 }, drops })
    d.start()
    d.ph.setDropsOpen(true)
    await settle()
    const before = asked.length
    d.ph.keepAll()
    await settle()
    expect([...d.ph.keep].sort()).toEqual(['p2', 'p5', 'p9'])
    expect(asked.length).toBe(before + 1)
    expect(asked.at(-1)?.keep).toHaveLength(3)
  })

  it('устаревший ответ отбрасывается: в полёте один запрос, ответ про прежний keep не показывается', async () => {
    const gates: ((v: SimilarDrop[]) => void)[] = []
    const calls: string[][] = []
    const drops: DropsFn = (_i, _r, ph) => {
      calls.push(ph.keep)
      return new Promise((res) => gates.push(res))
    }
    const { d } = make({ saved: { similar: 1 }, drops })
    d.start()
    d.ph.setDropsOpen(true)
    await settle()
    expect(calls).toHaveLength(1)
    d.ph.toggleKeep('p2') // пока ответ идёт
    d.ph.toggleKeep('p3')
    await settle()
    expect(calls).toHaveLength(1) // второй запрос не уходит, пока идёт первый
    gates[0]([drop('p7')])
    await settle()
    expect(d.ph.drops).toBeNull() // ответ про прежний keep не принят
    expect(calls).toHaveLength(2)
    expect(calls[1].sort()).toEqual(['p2', 'p3'])
    gates[1]([drop('p8')])
    await settle()
    expect(d.ph.drops?.map((x) => x.id)).toEqual(['p8'])
  })

  it('ошибка Go: плашка вместо ряда, окно живо', async () => {
    const drops: DropsFn = async () => Promise.reject(new Error('плохо'))
    const { d } = make({ saved: { similar: 1 }, drops })
    d.start()
    d.ph.setDropsOpen(true)
    await settle()
    expect(d.ph.dropsError).toBe(true)
    expect(d.canBuild).toBe(true)
  })

  it('свёрнули — запросов больше нет', async () => {
    const drops = rows([drop('p2')])
    const { d } = make({ saved: { similar: 1 }, drops })
    d.start()
    d.ph.setDropsOpen(true)
    await settle()
    d.ph.setDropsOpen(false)
    d.ph.toggleKeep('p2')
    await settle()
    expect(drops).toHaveBeenCalledTimes(1)
  })
})
