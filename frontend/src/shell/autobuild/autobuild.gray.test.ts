import { describe, expect, it, vi } from 'vitest'
import type { AutoPlan, AutoPreview, PhotoOpts } from '../../base/core/types'
import { unitOf } from '../../base/geom/sheet'
import { AutoDialog, type PlanFn, type PreviewFn } from './autobuild.svelte'
import { spreadNote } from './autolayouts'
import { GRAY_OPTIONS, grayText } from './autosimilar'

// 078: «Ч/б развороты» — строка только с функцией «Ч/б и тон фото», подписи Go готовит так же, как для «похожих».
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

function make(saved: { gray?: number; similar?: number } = {}, grayOn = true, grayCount = 2) {
  const asked: PhotoOpts[] = []
  const previews: PhotoOpts[] = []
  const planFn: PlanFn = async (_i, spreads, _r, ph) => {
    asked.push(ph)
    return plan({ spreads, gray: grayCount })
  }
  const previewFn: PreviewFn = async (_i, o) => {
    previews.push(o.ph)
    return { w: 1, h: 1, samples: [], spreads: [], fallback: 0 } as unknown as AutoPreview
  }
  const sigs = vi.fn(async () => ({ done: 12, total: 12 }))
  const d = new AutoDialog(ids, planFn, { spreads: null, ...saved }, previewFn, { grayOn, sigs })
  return { d, asked, previews, sigs }
}

describe('Ч/б развороты: уровни и Go', () => {
  it('«Нет» по умолчанию; уровень уходит в план и предпросмотр, подписи готовятся', async () => {
    const { d, asked, previews, sigs } = make()
    d.start()
    await settle()
    expect(asked[0].gray).toBe(0)
    expect(sigs).not.toHaveBeenCalled()
    d.ph.setGray(2)
    await settle()
    expect(sigs).toHaveBeenCalledTimes(1) // цветность — из подписи
    expect(asked.at(-1)?.gray).toBe(2)
    expect(previews.at(-1)?.gray).toBe(2)
    expect(d.plan?.gray).toBe(2) // «Ч/б станет: 2 разворота»
  })

  it('без функции «Ч/б и тон фото» (уровень «Простой») в Go уходит 0, подписи не нужны, память не теряется', async () => {
    const { d, asked, sigs } = make({ gray: 3 }, false)
    d.start()
    await settle()
    expect(asked[0].gray).toBe(0)
    expect(d.ph.gray).toBe(3) // запомненное остаётся для следующего уровня
    expect(d.ph.needSigs).toBe(false)
    expect(sigs).not.toHaveBeenCalled()
    expect(d.canBuild).toBe(true)
  })

  it('«Нет» после ч/б: подписи ждать не нужно, в Go 0', async () => {
    const { d, asked } = make({ gray: 1 })
    d.start()
    await settle()
    expect(asked[0].gray).toBe(1)
    d.ph.setGray(0)
    await settle()
    expect(asked.at(-1)?.gray).toBe(0)
    expect(d.ph.needSigs).toBe(false)
  })

  it('повторный выбор того же уровня — без запроса', async () => {
    const { d, asked } = make({ gray: 2 })
    d.start()
    await settle()
    const n = asked.length
    d.ph.setGray(2)
    d.ph.setGray(7) // мусор — «Нет»
    await settle()
    expect(asked.length).toBe(n + 1)
    expect(d.ph.gray).toBe(0)
  })

  it('«похожие» и ч/б вместе готовят подписи один раз', async () => {
    const { d, sigs } = make({ similar: 1, gray: 1 })
    d.start()
    await settle()
    d.ph.setGray(2)
    d.ph.setSimilar(2)
    await settle()
    expect(sigs).toHaveBeenCalledTimes(1)
  })
})

describe('подписи ч/б', () => {
  const u = unitOf(undefined)
  it('«Ч/б станет: K разворотов» по числу из плана', () => {
    expect(grayText(2, 1, u)).toBe('Ч/б станет: 1 разворот')
    expect(grayText(2, 3, u)).toBe('Ч/б станет: 3 разворота')
    expect(grayText(3, 11, u)).toBe('Ч/б станет: 11 разворотов')
    expect(grayText(2, 3, unitOf({ sheet: true }))).toBe('Ч/б станет: 3 листа')
  })
  it('уровень выбран, а ч/б не получится — честная строка; «Нет» — пусто', () => {
    expect(grayText(1, 0, u)).toBe('Ч/б не будет: разворотов мало')
    expect(grayText(0, 5, u)).toBe('')
  })
  it('уровни окна — как в Go (0…3) и подписаны долями', () => {
    expect(GRAY_OPTIONS.map((o) => [o.v, o.label])).toEqual([
      [0, 'Нет'],
      [1, '10 %'],
      [2, '20 %'],
      [3, '30 %'],
    ])
  })
  it('плитка «Что соберётся» помечена «ч/б»', () => {
    expect(spreadNote({ photos: 4, gray: true })).toBe('4 фото · ч/б')
    expect(spreadNote({ photos: 4, gray: false })).toBe('4 фото')
    expect(spreadNote({ photos: 3 })).toBe('3 фото') // Go не прислал поле (omitempty)
  })
})
