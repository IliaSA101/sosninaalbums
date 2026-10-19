import { describe, expect, it } from 'vitest'
import type { AutoPlan, AutoPreview, PhotoOpts } from '../../base/core/types'
import { AutoDialog, type PlanFn, type PreviewFn } from './autobuild.svelte'
import { clampVariant, MAX_VARIANT, PhotosState, type DropsFn } from './autosimilar.svelte'
import { albumAfter, sourceLine, spreadsBefore } from './autobuild'

// 078: «Вариант ‹ k ›» и признак пересборки в запросах окна; режим пересборки окна (U2). Тост и вход — autobuild.lastauto.test.ts.
const ids = Array.from({ length: 12 }, (_, i) => `p${i}`)
const plan = (p: Partial<AutoPlan> = {}): AutoPlan => ({
  photos: 12,
  spreads: 3,
  minSpreads: 1,
  maxSpreads: 12,
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

function make(saved: { variant?: number; again?: boolean; keep?: string[] } = {}) {
  const asked: PhotoOpts[] = []
  const previews: PhotoOpts[] = []
  const planFn: PlanFn = async (_i, spreads, _r, ph) => {
    asked.push(ph)
    return plan({ spreads })
  }
  const previewFn: PreviewFn = async (_i, o) => {
    previews.push(o.ph)
    return { w: 1, h: 1, samples: [], spreads: [], fallback: 0 } as unknown as AutoPreview
  }
  const d = new AutoDialog(ids, planFn, { spreads: null, ...saved }, previewFn)
  return { d, asked, previews }
}

describe('Вариант сборки', () => {
  it('по умолчанию 1 (сборка как 0.19), не запоминается между окнами', async () => {
    const { d, asked } = make()
    d.start()
    await settle()
    expect(d.ph.variant).toBe(1)
    expect(asked[0].variant).toBe(1)
  })

  it('› и ‹ меняют номер: план и «Что соберётся» спрашиваются заново', async () => {
    const { d, asked, previews } = make()
    d.start()
    await settle()
    d.ph.setVariant(d.ph.variant + 1)
    await settle()
    expect(d.ph.variant).toBe(2)
    expect(asked.at(-1)?.variant).toBe(2)
    expect(previews.at(-1)?.variant).toBe(2)
    d.ph.setVariant(d.ph.variant - 1)
    await settle()
    expect(previews.at(-1)?.variant).toBe(1)
  })

  it('предел снизу — 1 (‹ неактивна), сверху — предел Go; мусор — 1', async () => {
    expect(clampVariant(0)).toBe(1)
    expect(clampVariant(-4)).toBe(1)
    expect(clampVariant(Number.NaN)).toBe(1)
    expect(clampVariant(2.6)).toBe(3)
    expect(clampVariant(MAX_VARIANT + 5)).toBe(MAX_VARIANT)
    const { d, asked } = make()
    d.start()
    await settle()
    const n = asked.length
    d.ph.setVariant(1) // тот же номер — запроса нет
    d.ph.setVariant(0)
    await settle()
    expect(asked.length).toBe(n)
  })

  it('быстрые щелчки по › — в Go уходит последний номер, лишних ответов не показываем', async () => {
    const { d, asked } = make()
    d.start()
    await settle()
    for (let i = 0; i < 4; i++) d.ph.setVariant(d.ph.variant + 1)
    await settle()
    expect(d.ph.variant).toBe(5)
    expect(asked.at(-1)?.variant).toBe(5)
    expect(d.pending).toBe(false)
  })

  it('вариант не меняет число фото и N: ползунок на месте', async () => {
    const { d } = make()
    d.start()
    await settle()
    const n = d.n
    d.ph.setVariant(3)
    await settle()
    expect(d.n).toBe(n)
    expect(d.canBuild).toBe(true)
  })
})

describe('Пересборка в запросах окна', () => {
  it('again доходит до плана и предпросмотра; вариант задаёт вызывающий', async () => {
    const { d, asked, previews } = make({ again: true, variant: 4 })
    d.start()
    await settle()
    expect(d.ph.again).toBe(true)
    expect(asked[0]).toMatchObject({ again: true, variant: 4 })
    expect(previews.at(-1)).toMatchObject({ again: true, variant: 4 })
  })

  it('без признака — обычная сборка', async () => {
    const { d, asked } = make()
    d.start()
    await settle()
    expect(asked[0].again).toBe(false)
  })
})

describe('Окно в режиме пересборки (U2)', () => {
  it('строка источника: «Пересборка: те же N фото»', () => {
    const d = { from: 'selected' as const, count: 240, inAlbum: 3, skipped: 0, filtered: false, again: true }
    expect(sourceLine(d, 240)).toBe('Пересборка: те же 240 фото')
    expect(sourceLine(d, 230)).toBe('Пересборка: те же 240 фото; 10 из сеток и с фонов не беру')
    expect(sourceLine({ ...d, again: false }, 240)).toContain('Выделенные фото')
  })

  it('приписка «из сеток и с фонов» не включает убранные похожие (они — отдельной строкой)', () => {
    const sel = { from: 'selected' as const, count: 80, inAlbum: 0, skipped: 0, filtered: false }
    // 80 выбрано, 43 в плане, 37 из них убрано как похожие — «из сеток» нечего приписывать
    expect(sourceLine(sel, 43, 37)).toBe('Выделенные фото: 80')
    // часть — похожие (37), часть — сетки и фоны (3)
    expect(sourceLine(sel, 40, 37)).toBe('Выделенные фото: 80; 3 из сеток и с фонов не беру')
    // без «Дублей» прежнее поведение
    expect(sourceLine(sel, 77)).toBe('Выделенные фото: 80; 3 из сеток и с фонов не беру')
    // пересборка: те же 217 фото, 38 убрано похожими
    const again = { ...sel, count: 217, again: true }
    expect(sourceLine(again, 179, 38)).toBe('Пересборка: те же 217 фото')
    expect(sourceLine(again, 175, 38)).toBe('Пересборка: те же 217 фото; 4 из сеток и с фонов не беру')
    // план ещё не пришёл / пуст — приписки нет
    expect(sourceLine(sel, null, 5)).toBe('Выделенные фото: 80')
    expect(sourceLine(sel, 0, 5)).toBe('Выделенные фото: 80')
  })

  it('«В альбоме станет»: при пересборке развороты прошлой сборки не прибавляются', () => {
    // до сборки в альбоме 0 разворотов, сборка дала 12; сейчас в альбоме 12
    expect(spreadsBefore(true, 12, { have: 0 })).toBe(0)
    expect(albumAfter(spreadsBefore(true, 12, { have: 0 }), 12)).toBe('') // пустой альбом — строки нет
    // был 1 разворот, сборка добавила 12, пересборка тоже на 12 → станет 13, а не 25
    expect(albumAfter(spreadsBefore(true, 13, { have: 1 }), 12)).toBe('В альбоме станет 13 разворотов (26 стр.)')
    // обычная сборка и нет снимка — число из вида
    expect(spreadsBefore(false, 13, { have: 1 })).toBe(13)
    expect(spreadsBefore(undefined, 13, { have: 1 })).toBe(13)
    expect(spreadsBefore(true, 13, null)).toBe(13)
  })

  it('«Вернуть» прошлой сборки сохраняются: в запросе keep, ключ плана учитывает набор', async () => {
    const { d, asked } = make({ again: true, variant: 3, keep: ['p1', 'p2'] })
    d.start()
    await settle()
    expect([...d.ph.keep].sort()).toEqual(['p1', 'p2'])
    expect([...asked[0].keep].sort()).toEqual(['p1', 'p2'])
    expect(d.ph.key).not.toBe(make({ again: true, variant: 3 }).d.ph.key)
  })

  it('первый вопрос ряда отброшенных идёт без «Вернуть» — плитки вернувшихся видны, дальше с ними', async () => {
    const seen: string[][] = []
    const dropsFn: DropsFn = async (_i, _r, o) => {
      seen.push([...o.keep])
      return [{ id: 'p1', like: 'p0', dist: 5 }]
    }
    const host = { range: () => ({ auto: true, minPer: 1, maxPer: 64, series: true }), replan: () => {} }
    const ph = new PhotosState(ids, host, { similar: 1, keep: ['p1'] }, true, null, dropsFn)
    ph.start()
    ph.setDropsOpen(true)
    await settle()
    expect(seen).toEqual([[]]) // первый вопрос
    expect(ph.drops?.map((x) => x.id)).toEqual(['p1'])
    expect(ph.keep.has('p1')).toBe(true) // плитка «вернётся»
    ph.toggleKeep('p9')
    await settle()
    expect(seen.at(-1)).toContain('p1') // следующие — с набором
  })
})
