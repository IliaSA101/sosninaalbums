import { describe, expect, it } from 'vitest'
import type { AutoPlan, AutoPreview, Family } from '../../base/core/types'
import { model } from '../../../wailsjs/go/models'
import { AutoDialog, type PlanFn, type PreviewFn, type PreviewOpts } from './autobuild.svelte'

// Ряд «Раскладки» (065): Go подменён, ответы плана и предпросмотра держит тест.
const plan = (p: Partial<AutoPlan> = {}): AutoPlan => ({
  photos: 100,
  spreads: 25,
  minSpreads: 2,
  maxSpreads: 100,
  avg: 4,
  min: 3,
  max: 5,
  reuse: 0,
  from: 3,
  to: 5,
  cuts: 0,
  ...p,
})
const ids = Array.from({ length: 100 }, (_, i) => String(i))
const pv = (tag: number, fallback = 0): AutoPreview =>
  model.AutoPreview.createFrom({
    w: 600,
    h: 300,
    // в «Вариантах» есть все шесть семейств — сегменты фильтра все на экране
    samples: [
      {
        photos: 4,
        group: 0,
        inOrder: (['dense', 'hero', 'air', 'bleed', 'strip', 'figure'] as Family[]).map((family) => ({
          key: family,
          rects: [],
          family,
          twin: -1,
          formName: '',
        })),
        extra: [],
      },
    ],
    spreads: Array.from({ length: tag }, (_, i) => ({
      key: `k${i}`,
      rects: [],
      family: 'dense',
      twin: -1,
      formName: '',
      photos: 4,
      fallback: false,
    })),
    fallback,
  })

function fake(photos = 100) {
  const asked: PreviewOpts[] = []
  const gates: ((v: AutoPreview | Error) => void)[] = []
  const planFn: PlanFn = async (_ids, spreads) => plan({ photos, spreads, minSpreads: 1, maxSpreads: photos })
  const previewFn: PreviewFn = (_ids, o) => {
    asked.push(o)
    return new Promise((res, rej) => gates.push((v) => (v instanceof Error ? rej(v) : res(v))))
  }
  // Ответить на самый ранний запрос ряда и дать цепочкам промисов отработать
  const answer = async (v: AutoPreview | Error) => {
    gates.shift()?.(v)
    await new Promise((r) => setTimeout(r, 0))
  }
  const settle = () => new Promise((r) => setTimeout(r, 0))
  return { planFn, previewFn, asked, answer, settle, waiting: () => gates.length }
}

describe('AutoDialog: ряд «Раскладки» (065)', () => {
  it('после ответа плана ряд спрашивается с текущими значениями; пока ждём — previewBusy', async () => {
    const f = fake()
    const d = new AutoDialog(ids, f.planFn, { spreads: 10, range: [1, 64], variety: 2, families: ['dense', 'hero'] }, f.previewFn)
    d.start()
    await f.settle()
    expect(f.asked).toHaveLength(1)
    expect(f.asked[0]).toMatchObject({ spreads: 10, variety: 2, families: ['dense', 'hero'] })
    expect(d.previewBusy).toBe(true)
    expect(d.preview).toBeNull()
    await f.answer(pv(10))
    expect(d.preview?.spreads).toHaveLength(10)
    expect(d.previewBusy).toBe(false)
    expect(d.previewError).toBe(false)
  })

  it('щелчки по всем шести сегментам — «Все» и один запрос с пустым набором', async () => {
    const f = fake()
    const d = new AutoDialog(
      ids,
      f.planFn,
      { spreads: 10, range: [1, 64], families: ['dense', 'hero', 'air', 'bleed', 'strip'] },
      f.previewFn,
    )
    d.start()
    await f.settle()
    await f.answer(pv(10))
    d.setFamilies('figure')
    expect(d.families).toEqual([])
    await f.settle()
    expect(f.asked.at(-1)?.families).toEqual([])
  })

  it('смена семейства и разнообразия пересчитывает ряд', async () => {
    const f = fake()
    const d = new AutoDialog(ids, f.planFn, { spreads: 10, range: [1, 64] }, f.previewFn)
    d.start()
    await f.settle()
    await f.answer(pv(10))
    d.setFamilies('strip')
    expect(d.families).toEqual(['strip'])
    await f.settle()
    expect(f.asked).toHaveLength(2)
    expect(f.asked[1].families).toEqual(['strip'])
    await f.answer(pv(10))
    d.setVariety(0)
    await f.settle()
    expect(f.asked[2].variety).toBe(0)
    await f.answer(pv(10))
  })

  it('устаревший ответ отбрасывается: показывается ряд про последние значения, запрос один в полёте', async () => {
    const f = fake()
    const d = new AutoDialog(ids, f.planFn, { spreads: 10, range: [1, 64] }, f.previewFn)
    d.start()
    await f.settle()
    await f.answer(pv(10))
    d.setFamilies('dense')
    d.setFamilies('air') // второй щелчок, пока первый запрос в полёте
    await f.settle()
    expect(f.waiting()).toBe(1) // один в полёте
    await f.answer(pv(1)) // ответ про ['dense'] — устарел
    expect(d.preview?.spreads).toHaveLength(10) // прежний ряд, не устаревший
    await f.settle()
    expect(f.asked.at(-1)?.families).toEqual(['dense', 'air'])
    await f.answer(pv(2))
    expect(d.preview?.spreads).toHaveLength(2)
  })

  it('ползунок: ряд ждёт ответа плана, потом спрашивает про последнее N', async () => {
    const f = fake()
    const d = new AutoDialog(ids, f.planFn, { spreads: 10, range: [1, 64] }, f.previewFn)
    d.start()
    await f.settle()
    await f.answer(pv(10))
    d.setSpreads(20)
    d.setSpreads(30)
    await f.settle()
    await f.settle()
    expect(f.asked.map((a) => a.spreads)).toEqual([10, 30])
    await f.answer(pv(30))
    expect(d.preview?.spreads).toHaveLength(30)
  })

  it('ошибка ряда: previewError, «Собрать» не блокируется', async () => {
    const f = fake()
    const d = new AutoDialog(ids, f.planFn, { spreads: 10, range: [1, 64] }, f.previewFn)
    d.start()
    await f.settle()
    await f.answer(new Error('нет'))
    expect(d.previewError).toBe(true)
    expect(d.preview).toBeNull()
    expect(d.canBuild).toBe(true)
    // следующая смена значений снимает ошибку
    d.setFamilies('air')
    await f.settle()
    await f.answer(pv(10))
    expect(d.previewError).toBe(false)
  })

  it('нет фото: ряд не спрашиваем', async () => {
    const f = fake(0)
    const d = new AutoDialog([], f.planFn, { spreads: null, range: [1, 64] }, f.previewFn)
    d.start()
    await f.settle()
    expect(f.asked).toHaveLength(0)
    expect(d.preview).toBeNull()
  })

  it('ошибка плана сбрасывает ряд', async () => {
    const f = fake()
    let fail = false
    const planFn: PlanFn = (a, b, c, p) => (fail ? Promise.reject(new Error('плохо')) : f.planFn(a, b, c, p))
    const d = new AutoDialog(ids, planFn, { spreads: 10, range: [1, 64] }, f.previewFn)
    d.start()
    await f.settle()
    await f.answer(pv(10))
    fail = true
    d.setSpreads(11)
    await f.settle()
    expect(d.preview).toBeNull()
    expect(d.error).not.toBe('')
  })

  it('submit отдаёт семейства шестым аргументом в порядке сегментов', async () => {
    const f = fake()
    const d = new AutoDialog(ids, f.planFn, { spreads: 10, range: [1, 64], families: ['figure', 'dense'] }, f.previewFn)
    d.start()
    await f.settle()
    await f.answer(pv(10))
    const got: unknown[][] = []
    expect(await d.submit((...a) => got.push(a))).toBe(true)
    expect(got[0][5]).toEqual(['dense', 'figure'])
  })

  it('память: мусор в семействах — «Все»; без насоса предпросмотра окно работает как раньше', async () => {
    const f = fake()
    const bad = new AutoDialog(ids, f.planFn, { spreads: 10, families: ['x', 5] as unknown as Family[] })
    expect(bad.families).toEqual([])
    bad.start()
    await f.settle()
    bad.setFamilies('hero') // нет ряда — нет и запросов, ошибки тоже
    expect(bad.preview).toBeNull()
    expect(bad.previewError).toBe(false)
  })

  it('режим ряда: по умолчанию «Варианты», переключается без запросов; повторный щелчок — то же', () => {
    const f = fake()
    const d = new AutoDialog(ids, f.planFn, { spreads: 10, range: [1, 64] }, f.previewFn)
    expect(d.rowMode).toBe('samples')
    d.setRowMode('build')
    expect(d.rowMode).toBe('build')
    d.setRowMode('build')
    expect(d.rowMode).toBe('build')
    expect(f.asked).toHaveLength(0)
  })
})
