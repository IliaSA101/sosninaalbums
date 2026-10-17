import { describe, expect, it } from 'vitest'
import type { AutoPlan } from '../../base/core/types'
import { unitOf } from '../../base/geom/sheet'
import {
  albumAfter,
  anchorField,
  clampMaxPer,
  clampSpreads,
  defaultSpreads,
  firstSpreads,
  otherField,
  perStep,
  perText,
  planHint,
  rangeText,
  sourceLine,
  spreadsForPer,
  spreadsLabel,
  unitWord,
  VARIETY_OPTIONS,
  normVariety,
} from './autobuild'
import { AutoDialog, type PlanFn } from './autobuild.svelte'

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

describe('подписи', () => {
  it('разворотов и страниц', () => {
    expect(spreadsLabel(1)).toBe('1 разворот (2 стр.)')
    expect(spreadsLabel(3)).toBe('3 разворота (6 стр.)')
    expect(spreadsLabel(5)).toBe('5 разворотов (10 стр.)')
    expect(spreadsLabel(21)).toBe('21 разворот (42 стр.)')
  })
  it('лист виньетки: без страниц, «на лист»', () => {
    const u = unitOf({ sheet: true })
    expect(spreadsLabel(1, u)).toBe('1 лист')
    expect(spreadsLabel(3, u)).toBe('3 листа')
    expect(spreadsLabel(5, u)).toBe('5 листов')
    expect(planHint(plan({ avg: 4, min: 4, max: 4 }), u)).toBe('по 4 фото на лист')
    expect(planHint(plan({ avg: 4.3333, min: 3, max: 6 }), u)).toBe('от 3 до 6 фото на лист')
  })
  it('подсказка: только границы, без дроби', () => {
    expect(planHint(plan({ avg: 4.3333, min: 3, max: 6 }))).toBe('от 3 до 6 фото на разворот')
    expect(planHint(plan({ avg: 4.04, min: 4, max: 5 }))).toBe('от 4 до 5 фото на разворот')
  })
  it('a = b — «по N», нет фото — своя фраза', () => {
    expect(planHint(plan({ avg: 4, min: 4, max: 4 }))).toBe('по 4 фото на разворот')
    expect(planHint(plan({ photos: 0, avg: 0, min: 0, max: 0, minSpreads: 0, maxSpreads: 0 }))).toBe('Нет фото для сборки')
  })
})

describe('подписи 034', () => {
  it('среднее в поле: одна цифра, запятая', () => {
    expect(perText(642, 30)).toBe('21')
    expect(perText(42, 10)).toBe('4')
    expect(perText(91, 10)).toBe('9')
    expect(perText(12, 3)).toBe('4')
    expect(perText(0, 0)).toBe('')
  })
  it('«от a до b» и «по a»', () => {
    expect(rangeText({ min: 14, max: 28 })).toBe('от 14 до 28')
    expect(rangeText({ min: 4, max: 4 })).toBe('по 4')
  })
  it('откуда фото: выделение', () => {
    const d = { from: 'selected' as const, count: 120, inAlbum: 0, skipped: 0, filtered: false }
    expect(sourceLine(d, 120)).toBe('Выделенные фото: 120')
    expect(sourceLine({ ...d, inAlbum: 7 }, 120)).toBe('Выделенные фото: 120, из них 7 уже в альбоме — переедут в новые развороты')
  })
  it('откуда фото: лента, фильтр, отклонённые', () => {
    const d = { from: 'free' as const, count: 642, inAlbum: 0, skipped: 0, filtered: false }
    expect(sourceLine(d, null)).toBe('Все свободные фото ленты: 642')
    expect(sourceLine({ ...d, filtered: true }, 642)).toBe('Все свободные фото ленты: 642 (фильтр ленты)')
    expect(sourceLine({ ...d, filtered: true, skipped: 12 }, 642)).toBe('Все свободные фото ленты: 642 (фильтр ленты), без 12 отклонённых')
    expect(sourceLine({ ...d, skipped: 1 }, 642)).toBe('Все свободные фото ленты: 642, без 1 отклонённого')
  })
  it('откуда фото: разница с Go — сетки и фоны', () => {
    const d = { from: 'free' as const, count: 645, inAlbum: 0, skipped: 12, filtered: false }
    expect(sourceLine(d, 642)).toBe('Все свободные фото ленты: 645, без 12 отклонённых; 3 из сеток и с фонов не беру')
    expect(sourceLine({ ...d, from: 'selected', skipped: 0 }, 642)).toBe('Выделенные фото: 645; 3 из сеток и с фонов не беру')
  })
  it('станет в альбоме', () => {
    expect(albumAfter(3, 30)).toBe('В альбоме станет 33 разворота (66 стр.)')
    expect(albumAfter(0, 30)).toBe('')
    expect(albumAfter(2, 3, unitOf({ sheet: true }))).toBe('В альбоме станет 5 листов')
  })
  it('albumAfter учитывает занятые пустые', () => {
    // have = 3 (с пустым хвостом 1), N = 1: итог (3 − 1) + 1 = 3
    expect(albumAfter(3, 1, undefined, 1)).toBe('В альбоме станет 3 разворота (6 стр.) — пустые в конце (1) займёт сборка')
    expect(albumAfter(5, 4, undefined, 2)).toBe('В альбоме станет 7 разворотов (14 стр.) — пустые в конце (2) займёт сборка')
    expect(albumAfter(2, 3, unitOf({ sheet: true }), 1)).toBe('В альбоме станет 4 листа — пустые в конце (1) займёт сборка')
    // весь альбом — пустой хвост: как пустой альбом, строки нет
    expect(albumAfter(1, 3, undefined, 1)).toBe('')
    expect(albumAfter(4, 9, undefined, 4)).toBe('')
    // reuse = 0 — прежний текст
    expect(albumAfter(3, 30, undefined, 0)).toBe('В альбоме станет 33 разворота (66 стр.)')
  })
  it('подпись числа склоняется', () => {
    const w = (n: number) => unitWord(n)
    expect([1, 2, 3, 5, 11, 21, 22, 25].map(w)).toEqual([
      'разворот',
      'разворота',
      'разворота',
      'разворотов',
      'разворотов',
      'разворот',
      'разворота',
      'разворотов',
    ])
    expect(unitWord(1, unitOf({ sheet: true }))).toBe('лист')
    expect(unitWord(3, unitOf({ sheet: true }))).toBe('листа')
  })
})

describe('зажимы 034', () => {
  const lim = { min: 10, max: 642 }
  it('N из среднего', () => {
    expect(spreadsForPer(642, 4, lim)).toBe(161)
    expect(spreadsForPer(642, 0.5, lim)).toBe(642)
    expect(spreadsForPer(642, 100, lim)).toBe(10)
    for (const bad of [NaN, 0, -3, Infinity]) expect(spreadsForPer(642, bad, lim)).toBeNull()
  })
  it('шаг среднего меняет N хотя бы на 1', () => {
    const l = { min: 1, max: 642 }
    expect(perStep(642, 30, 1, l)).toBe(29)
    expect(perStep(642, 30, -1, l)).toBe(32) // N=31 даёт то же целое 21
    expect(perStep(42, 10, 1, { min: 1, max: 42 })).toBe(9)
    expect(perStep(42, 10, -1, { min: 1, max: 42 })).toBe(13) // 11 и 12 дают то же 4
    expect(perStep(642, 400, 1, l)).toBeLessThan(400)
    expect(perStep(642, 400, -1, l)).toBeGreaterThan(400)
  })
  it('показанное целое меняется ровно на 1, N — хотя бы на 1', () => {
    const l = { min: 1, max: 642 }
    for (const [n, N] of [
      [642, 30],
      [642, 400],
      [91, 10],
      [42, 10],
      [9, 4],
    ] as const) {
      for (const dir of [1, -1] as const) {
        const next = perStep(n, N, dir, l)
        expect(next).not.toBe(N)
        expect(Math.sign(N - next)).toBe(dir)
        expect(Math.round(n / next) - Math.round(n / N)).toBe(dir)
      }
    }
  })
  it('когда целого с шагом 1 нет (N крупный шаг), берётся ближайший сдвиг', () => {
    // 100/7 → 14, 100/6 → 17: пропуск неизбежен, N всё равно сдвигается
    expect(perStep(100, 7, 1, { min: 1, max: 100 })).toBe(6)
  })
  it('на краю пределов N не выходит за них', () => {
    expect(perStep(642, 642, -1, { min: 1, max: 642 })).toBe(642)
    expect(perStep(642, 1, 1, { min: 1, max: 642 })).toBe(1)
    expect(perStep(642, 30, 1, { min: 30, max: 642 })).toBe(30)
    expect(perStep(642, 100, -1, { min: 10, max: 100 })).toBe(100)
  })
  it('первое N по якорю', () => {
    const l = { min: 1, max: 90 }
    expect(firstSpreads(90, { spreads: 10, per: 4.5, by: 'per' }, l)).toBe(20)
    expect(firstSpreads(90, { spreads: 10, per: 4.5, by: 'spreads' }, l)).toBe(10)
    expect(firstSpreads(90, { spreads: 500, per: null, by: 'spreads' }, l)).toBe(90)
    expect(firstSpreads(90, { spreads: null, per: null, by: 'spreads' }, l)).toBe(23) // round(90 / 4) = 22,5 → 23
    expect(firstSpreads(90, { spreads: 10, per: null, by: 'per' }, l)).toBe(10) // X нет — запомненное N
  })
})

describe('зажимы', () => {
  it('запомненный N зажимается в пределы ползунка', () => {
    const p = { minSpreads: 3, maxSpreads: 20 }
    expect(clampSpreads(1, p)).toBe(3)
    expect(clampSpreads(8, p)).toBe(8)
    expect(clampSpreads(500, p)).toBe(20)
  })
  it('без фото (предела нет) N не трогаем', () => {
    expect(clampSpreads(7, { minSpreads: 0, maxSpreads: 0 })).toBe(7)
    expect(clampSpreads(7, null)).toBe(7)
  })
  it('первый раз N = round(фото / 4), не меньше 1', () => {
    expect(defaultSpreads(100)).toBe(25)
    expect(defaultSpreads(10)).toBe(3)
    expect(defaultSpreads(1)).toBe(1)
  })
  it('«Свой от–до»: N от середины диапазона, не меньше 1', () => {
    expect(defaultSpreads(100, [1, 3])).toBe(50)
    expect(defaultSpreads(100, [3, 5])).toBe(25)
    expect(defaultSpreads(1, [1, 64])).toBe(1)
    expect(firstSpreads(100, { spreads: null }, { min: 1, max: 100 }, [1, 3])).toBe(50)
    expect(firstSpreads(100, { spreads: 7 }, { min: 1, max: 100 }, [1, 3])).toBe(7) // запомненное не трогаем
  })
  it('«не больше» — 1…64, мусор — 64', () => {
    expect(clampMaxPer(0)).toBe(1)
    expect(clampMaxPer(12)).toBe(12)
    expect(clampMaxPer(200)).toBe(64)
    expect(clampMaxPer(NaN)).toBe(64)
  })
})

// Go подменён: ответ можно держать, пока тест не отпустит.
function fakeGo(photos = 100) {
  const calls: [number, number][] = []
  const gates: (() => void)[] = []
  const ask: PlanFn = (_ids, spreads, { maxPer }) => {
    calls.push([spreads, maxPer])
    const minSpreads = Math.ceil(photos / maxPer)
    const n = Math.max(minSpreads, Math.min(photos, spreads))
    const answer = plan({
      photos,
      spreads: n,
      minSpreads,
      maxSpreads: photos,
      avg: photos / n,
      min: Math.floor(photos / n),
      max: Math.ceil(photos / n),
    })
    return new Promise((res) => gates.push(() => res(answer)))
  }
  const release = async () => {
    gates.shift()?.()
    await Promise.resolve()
    await new Promise((r) => setTimeout(r, 0))
  }
  return { ask, calls, release, waiting: () => gates.length }
}

// «Свой» 1…64 с серией: так ушёл бы старый «не больше 64»
const R64 = { auto: false, minPer: 1, maxPer: 64, series: true }
const ids = Array.from({ length: 100 }, (_, i) => String(i))

describe('AutoDialog', () => {
  it('первый раз: «Считаю…», потом N = round(фото / 4)', async () => {
    const go = fakeGo()
    const d = new AutoDialog(ids, go.ask, { spreads: null }) // «Авто»
    d.start()
    expect(d.pending).toBe(true)
    expect(d.plan).toBeNull()
    await go.release()
    expect(d.pending).toBe(false)
    expect(d.n).toBe(25)
    expect(d.plan?.spreads).toBe(25)
    expect(d.canBuild).toBe(true)
  })

  it('«Свой 1–3» без запомненного N: 100 фото → 50 разворотов, не нижний предел 34', async () => {
    const go = fakeGo()
    const d = new AutoDialog(ids, go.ask, { spreads: null, range: [1, 3] })
    d.start()
    await go.release() // первый запрос уже про 50 (середина 2), нижний предел Go — 34
    expect(go.calls).toEqual([[50, 3]])
    expect(d.n).toBe(50)
    expect(d.plan?.spreads).toBe(50)
  })

  it('«Свой 1–3» с запомненным N: N не трогается', async () => {
    const go = fakeGo()
    const d = new AutoDialog(ids, go.ask, { spreads: 40, range: [1, 3] })
    d.start()
    await go.release()
    expect(d.n).toBe(40)
    expect(go.calls).toEqual([[40, 3]])
  })

  it('запомненный N вне пределов зажимается, и подсказка про зажатое значение', async () => {
    const go = fakeGo(100)
    const d = new AutoDialog(ids, go.ask, { spreads: 1, range: [1, 20] })
    d.start()
    await go.release() // ответ на N = 1 при m = 20: низ ползунка 5
    await go.release() // повторный запрос уже про 5
    expect(go.calls).toEqual([
      [1, 20],
      [5, 20],
    ])
    expect(d.n).toBe(5)
    expect(d.plan?.spreads).toBe(5)
    expect(d.pending).toBe(false)
  })

  it('быстрые движения ползунка: запрос один, ответ на устаревшее N отбрасывается', async () => {
    const go = fakeGo()
    const d = new AutoDialog(ids, go.ask, { spreads: 10, range: [1, 64] })
    d.start()
    await go.release()
    d.setSpreads(20)
    d.setSpreads(21)
    d.setSpreads(22)
    expect(go.waiting()).toBe(1) // в полёте только запрос про 20
    await go.release()
    expect(d.pending).toBe(true) // ответ про 20 не показан, ушёл запрос про 22
    expect(d.plan?.spreads).not.toBe(20)
    await go.release()
    expect(go.calls.map((c) => c[0])).toEqual([10, 20, 22])
    expect(d.plan?.spreads).toBe(22)
    expect(d.pending).toBe(false)
  })

  it('«Собрать» ждёт ответа на текущее N', async () => {
    const go = fakeGo()
    const d = new AutoDialog(ids, go.ask, { spreads: 10, range: [1, 64] })
    d.start()
    await go.release()
    d.setSpreads(30)
    const built: [number, number][] = []
    const done = d.submit((n, r) => built.push([n, r.maxPer]))
    await Promise.resolve()
    expect(built).toEqual([]) // ответа ещё нет
    await go.release()
    expect(await done).toBe(true)
    expect(built).toEqual([[30, 64]])
  })

  it('«не больше» поднимает низ ползунка и подтягивает N', async () => {
    const go = fakeGo()
    const d = new AutoDialog(ids, go.ask, { spreads: 10, range: [1, 64] })
    d.start()
    await go.release()
    d.setMaxPer(5) // 100 фото по 5 — не меньше 20 разворотов
    await go.release()
    await go.release()
    expect(d.maxPer).toBe(5)
    expect(d.n).toBe(20)
    expect(d.plan?.minSpreads).toBe(20)
  })

  it('нет фото для сборки: собирать нельзя', async () => {
    const ask: PlanFn = async () => plan({ photos: 0, spreads: 0, minSpreads: 0, maxSpreads: 0, avg: 0, min: 0, max: 0 })
    const d = new AutoDialog([], ask, { spreads: null, range: [1, 64] })
    d.start()
    await d.idle()
    expect(d.canBuild).toBe(false)
    expect(await d.submit(() => 0)).toBe(false)
  })

  it('ошибка Go показывается в окне, собирать нельзя', async () => {
    const ask: PlanFn = async () => {
      throw 'в альбоме уже 200 разворотов'
    }
    const d = new AutoDialog(ids, ask, { spreads: 5, range: [1, 64] })
    d.start()
    await d.idle()
    expect(d.error).toBe('В альбоме уже 200 разворотов')
    expect(d.canBuild).toBe(false)
    expect(d.pending).toBe(false)
  })

  it('ввод среднего: N по фактическому числу фото, один запрос', async () => {
    const go = fakeGo(90) // Go насчитал 90 фото, а пришло 100
    const d = new AutoDialog(ids, go.ask, { spreads: 10, range: [1, 64] })
    d.start()
    await go.release()
    d.setPer(4.5)
    expect(d.anchor).toBe('per')
    expect(d.n).toBe(20) // план уже есть: round(90 / 4,5)
    expect(go.waiting()).toBe(1)
    await go.release()
    expect(d.n).toBe(20)
    expect(d.plan?.spreads).toBe(20)
    expect(d.pending).toBe(false)
    expect(go.calls.at(-1)).toEqual([20, 64])
  })

  it('открытие по якорю X: до ответа n = ids.length, после — plan.photos', async () => {
    const go = fakeGo(90)
    const d = new AutoDialog(ids, go.ask, { spreads: 10, range: [1, 64], per: 5, by: 'per' })
    expect(d.anchor).toBe('per')
    expect(d.n).toBe(20) // round(100 / 5)
    d.start()
    await go.release() // ответ про 90 фото: round(90 / 5) = 18
    await go.release()
    expect(d.n).toBe(18)
    expect(d.plan?.photos).toBe(90)
    expect(d.pending).toBe(false)
  })

  it('мусор в поле среднего: N и якорь не меняются', async () => {
    const go = fakeGo()
    const d = new AutoDialog(ids, go.ask, { spreads: 10, range: [1, 64] })
    d.start()
    await go.release()
    d.setPer(NaN)
    d.setPer(0)
    d.setPer(-2)
    expect(d.n).toBe(10)
    expect(d.anchor).toBe('spreads')
    expect(go.waiting()).toBe(0)
  })

  it('шаг среднего: ↑ — меньше разворотов, якорь — среднее', async () => {
    const go = fakeGo(642)
    const d = new AutoDialog(
      Array.from({ length: 642 }, (_, i) => String(i)),
      go.ask,
      { spreads: 30, range: [1, 64] },
    )
    d.start()
    await go.release()
    d.stepPer(1)
    expect(d.n).toBe(29)
    expect(d.anchor).toBe('per')
    await go.release()
    d.stepPer(-1)
    d.stepPer(-1)
    expect(d.n).toBe(32) // 29 → 30 (21 вместо 22) → 32 (20: N=31 даёт то же 21)
  })

  it('«не больше» при якоре X: N заново из X в новых пределах', async () => {
    const go = fakeGo()
    const d = new AutoDialog(ids, go.ask, { spreads: 10, range: [1, 64] })
    d.start()
    await go.release()
    d.setPer(10) // N = 10
    await go.release()
    d.setMaxPer(5) // низ 20: N поднимется до 20, X остаётся 10
    await go.release()
    await go.release()
    expect(d.n).toBe(20)
    expect(d.perTyped).toBe(10)
    d.setMaxPer(64)
    await go.release()
    await go.release()
    expect(d.n).toBe(10) // X не потерян
  })

  it('якорь: последний тронутый; submit отдаёт N, m, X и якорь', async () => {
    const go = fakeGo()
    const d = new AutoDialog(ids, go.ask, { spreads: 10, range: [1, 64] })
    d.start()
    await go.release()
    expect(d.anchor).toBe('spreads')
    d.setPer(5)
    expect(d.anchor).toBe('per')
    d.setSpreads(25)
    expect(d.anchor).toBe('spreads')
    d.setPer(4)
    await go.release()
    await go.release()
    const got: unknown[][] = []
    expect(await d.submit((...a) => got.push(a))).toBe(true)
    expect(got).toEqual([[25, R64, 4, 'per', 1]])
    d.setSpreads(20)
    await go.release()
    await go.release()
    got.length = 0
    await d.submit((...a) => got.push(a))
    expect(got).toEqual([[20, R64, 5, 'spreads', 1]]) // X — фактическое 100 / 20
  })
})

describe('разнообразие раскладок (041)', () => {
  it('три положения и пояснение к каждому — слово в слово из спеки', () => {
    expect(VARIETY_OPTIONS.map((o) => [o.v, o.label])).toEqual([
      [0, 'Меньше'],
      [1, 'Обычное'],
      [2, 'Больше'],
    ])
    expect(VARIETY_OPTIONS.map((o) => o.hint)).toEqual([
      'На каждом развороте самая плотная раскладка',
      'Соседние развороты не повторяются, иногда — воздух и крупное фото',
      'Чаще ленты, сетки и воздух, соседние развороты разной формы', // 051 №5: прежнее слово заменено
    ])
  })
  it('normVariety: мусор и пусто — «Обычное»', () => {
    expect([0, 1, 2].map(normVariety)).toEqual([0, 1, 2])
    expect([undefined, null, 3, -1, '2', 1.5].map(normVariety)).toEqual([1, 1, 1, 1, 1, 1])
  })
  it('по умолчанию «Обычное»; запомненное берётся из памяти окна', () => {
    expect(new AutoDialog(ids, fakeGo().ask, { spreads: 10, range: [1, 64] }).variety).toBe(1)
    expect(new AutoDialog(ids, fakeGo().ask, { spreads: 10, range: [1, 64], variety: 2 }).variety).toBe(2)
    expect(new AutoDialog(ids, fakeGo().ask, { spreads: 10, range: [1, 64], variety: 0 }).variety).toBe(0)
  })
  it('выбор уходит в «Собрать» пятым аргументом и не гоняет запрос плана', async () => {
    const go = fakeGo()
    const d = new AutoDialog(ids, go.ask, { spreads: 10, range: [1, 64] })
    d.start()
    await go.release()
    d.setVariety(2)
    expect(go.calls).toHaveLength(1) // план от ручки не зависит
    const got: unknown[][] = []
    expect(await d.submit((...a) => got.push(a))).toBe(true)
    expect(got).toEqual([[10, R64, 10, 'spreads', 2]])
  })
})

describe('активное поле (щелчок по карточке)', () => {
  it('otherField: ⇄ ведёт во второе поле', () => {
    expect(otherField('spreads')).toBe('auto-per')
    expect(otherField('per')).toBe('auto-n')
  })

  it('anchorField: автофокус — в поле якоря', () => {
    expect(anchorField('per')).toBe('auto-per')
    expect(anchorField('spreads')).toBe('auto-n')
  })

  it('запомненное «среднее» переживает открытие без действий: якорь, X и N на месте', async () => {
    const go = fakeGo()
    const d = new AutoDialog(ids, go.ask, { spreads: 7, range: [1, 64], per: 4.5, by: 'per' })
    d.start()
    await go.release()
    const n = d.n
    expect(n).toBe(22) // round(100 / 4,5)
    expect(d.anchor).toBe('per')
    expect(d.perTyped).toBe(4.5)
    const got: unknown[][] = []
    await d.submit((...a) => got.push(a))
    expect(d.n).toBe(n)
    expect(got).toEqual([[n, R64, 4.5, 'per', 1]])
  })

  it('первое открытие: запомненное 40 фото на разворот при 80 фото — N = 2 сразу и после ответа', async () => {
    const go = fakeGo(80)
    const d = new AutoDialog(
      Array.from({ length: 80 }, (_, i) => String(i)),
      go.ask,
      { spreads: 1, range: [1, 64], per: 40, by: 'per' },
    )
    expect(d.n).toBe(2)
    d.start()
    await go.release()
    expect(d.n).toBe(2)
    expect(d.plan?.minSpreads).toBe(2)
    expect(d.canBuild).toBe(true)
  })

  it('activate: выбор без ввода меняет якорь, но не значения', async () => {
    const go = fakeGo()
    const d = new AutoDialog(ids, go.ask, { spreads: 20, range: [1, 64] })
    d.start()
    await go.release()
    expect(d.anchor).toBe('spreads')
    d.activate('per')
    expect(d.anchor).toBe('per')
    expect(d.n).toBe(20)
    expect(d.perTyped).toBe(d.photos / 20) // X — фактическое среднее
    expect(go.calls.length).toBe(1) // нового запроса нет
    d.activate('spreads')
    expect(d.anchor).toBe('spreads')
    expect(d.perTyped).toBeNull()
  })

  it('activate: уже заданный руками X не затирается повторным выбором того же поля', async () => {
    const go = fakeGo()
    const d = new AutoDialog(ids, go.ask, { spreads: 20, range: [1, 64] })
    d.start()
    await go.release()
    d.setPer(6.2)
    d.activate('per')
    expect(d.perTyped).toBe(6.2)
  })
})
