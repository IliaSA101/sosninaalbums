import { describe, expect, it } from 'vitest'
import type { AutoPlan, AutoRange } from '../../base/core/types'
import { memOf, RANGE_HINT, seriesText, settleRange, VARIETY_OPTIONS } from './autobuild'
import { AutoDialog, type PlanFn } from './autobuild.svelte'

// Диапазон «Фото на разворот» (051 §9). Go подменён по §8: Auto — от среднего (3…16 при 100 фото, N = 11),
// Свой — как набрано; пределы N — [⌈n/до⌉, ⌊n/от⌋].
function fakeGo(photos = 100, cuts = 0) {
  const asked: { spreads: number; range: AutoRange }[] = []
  const gates: (() => void)[] = []
  const ask: PlanFn = (_ids, spreads, range) => {
    asked.push({ spreads, range })
    const from = range.auto ? Math.max(1, Math.round(photos / 33)) : range.minPer
    const to = range.auto ? Math.max(1, Math.round((1.75 * photos) / 11)) : range.maxPer
    const minSpreads = Math.min(photos, Math.ceil(photos / to))
    const maxSpreads = Math.max(minSpreads, Math.floor(photos / from))
    const n = Math.max(minSpreads, Math.min(maxSpreads, spreads))
    const answer: AutoPlan = {
      photos,
      spreads: n,
      minSpreads,
      maxSpreads,
      avg: photos / n,
      min: from,
      max: to,
      reuse: 0,
      from,
      to,
      cuts: range.series ? cuts : 0,
    }
    return new Promise((res) => gates.push(() => res(answer)))
  }
  const release = async () => {
    gates.shift()?.()
    await Promise.resolve()
    await new Promise((r) => setTimeout(r, 0))
  }
  return { ask, asked, release, waiting: () => gates.length }
}

const ids = Array.from({ length: 100 }, (_, i) => String(i))
type Saved = ConstructorParameters<typeof AutoDialog>[2]
const open = async (saved: Saved = { spreads: 11 }, photos = 100, cuts = 0) => {
  const go = fakeGo(photos, cuts)
  const d = new AutoDialog(ids.slice(0, photos), go.ask, saved)
  d.start()
  await go.release()
  return { d, go }
}

describe('чистая часть 051', () => {
  it('settleRange: тронутое поле не меняется, подтягивается второе', () => {
    expect(settleRange(3, 16, 'min')).toEqual([3, 16])
    expect(settleRange(20, 16, 'min')).toEqual([20, 20])
    expect(settleRange(20, 16, 'max')).toEqual([16, 16])
    expect(settleRange(5, 5, 'max')).toEqual([5, 5])
  })
  it('memOf: «Авто» без полей, «Свой» — [от, до]', () => {
    expect(memOf({ auto: true, minPer: 1, maxPer: 64, series: true })).toBe('auto')
    expect(memOf({ auto: false, minPer: 8, maxPer: 10, series: false })).toEqual([8, 10])
  })
  it('«серий: N» — на одну больше границ; нет границ — пусто', () => {
    expect(seriesText(0)).toBe('')
    expect(seriesText(1)).toBe('серий: 2')
    expect(seriesText(8)).toBe('серий: 9')
  })
  it('слова спеки: подсказка диапазона и «Больше»', () => {
    expect(RANGE_HINT).toBe('Шире — альбом живее, уже — ровнее')
    expect(VARIETY_OPTIONS[2].hint).toBe('Чаще ленты, сетки и воздух, соседние развороты разной формы')
  })
})

describe('AutoDialog: диапазон 051', () => {
  it('первое открытие — «Авто», серии включены; поля показывают посчитанное, до ответа пусто', async () => {
    const go = fakeGo()
    const d = new AutoDialog(ids, go.ask, { spreads: null })
    expect([d.auto, d.series, d.from, d.to]).toEqual([true, true, null, null])
    d.start()
    await go.release()
    expect(go.asked[0].range).toEqual({ auto: true, minPer: 1, maxPer: 64, series: true })
    expect([d.from, d.to]).toEqual([3, 16])
  })

  it('«Авто»: поля едут за N (пределы N — прежние)', async () => {
    const { d, go } = await open()
    d.setSpreads(20)
    await go.release()
    expect(d.auto).toBe(true)
    expect(d.n).toBe(20)
  })

  it('ввод в поле «от» — «Свой»: второе поле берёт показанное', async () => {
    const { d, go } = await open()
    d.setMinPer(5)
    expect([d.auto, d.minPer, d.maxPer]).toEqual([false, 5, 16])
    await go.release()
    expect(go.asked.at(-1)?.range).toEqual({ auto: false, minPer: 5, maxPer: 16, series: true })
    expect([d.from, d.to]).toEqual([5, 16])
  })

  it('ввод в поле «до» — «Свой» от показанного «от»', async () => {
    const { d, go } = await open()
    d.setMaxPer(10)
    await go.release()
    expect([d.auto, d.from, d.to]).toEqual([false, 3, 10])
  })

  it('«Авто» возвращает автодиапазон; повторный щелчок по выбранному — без запроса', async () => {
    const { d, go } = await open()
    d.setAuto() // уже «Авто»
    expect(go.waiting()).toBe(0)
    d.setMinPer(8)
    await drain(go) // запомненный N первая смена диапазона пересчитывает — спрашивает заново
    d.setOwn() // уже «Свой»
    expect(go.waiting()).toBe(0)
    d.setAuto()
    await drain(go)
    expect(d.auto).toBe(true)
    expect([d.from, d.to]).toEqual([3, 16])
    expect(go.asked.at(-1)?.range.auto).toBe(true)
  })

  it('кнопка «Свой» оставляет показанные числа', async () => {
    const { d, go } = await open()
    d.setOwn()
    await go.release()
    expect([d.auto, d.minPer, d.maxPer]).toEqual([false, 3, 16])
  })

  it('Свой 8–10 на 100 фото зажимает N в [10, 12]', async () => {
    const { d, go } = await open({ spreads: 11, range: [8, 10] })
    expect(d.auto).toBe(false)
    d.setSpreads(3)
    await go.release()
    await go.release()
    expect(d.n).toBe(10)
    expect([d.plan?.minSpreads, d.plan?.maxSpreads]).toEqual([10, 12])
    d.setSpreads(31)
    await go.release()
    await go.release()
    expect(d.n).toBe(12)
  })

  it('«Свой»: поля показывают набранное, даже если Go опустил «от» в ответе; «Авто» — ответ Go', async () => {
    const { d, go } = await open({ spreads: 11, range: [8, 10] })
    expect([d.from, d.to]).toEqual([8, 10])
    d.setMinPer(9)
    await go.release()
    // Go опускает «от» до ⌊n/N⌋ (решения №27/№29): в ответе 5, в поле остаётся введённое 9
    d.plan = { ...(d.plan as AutoPlan), from: 5 }
    expect([d.from, d.to]).toEqual([9, 10])
    d.setAuto()
    await go.release()
    expect([d.from, d.to]).toEqual([d.plan?.from, d.plan?.to])
  })

  it('якорь «фото на разворот» зажимается в те же пределы', async () => {
    const { d, go } = await open({ spreads: 11, range: [8, 10] })
    d.setPer(50) // N = 2 → не ниже 10
    await go.release()
    await go.release()
    expect(d.n).toBe(10)
  })

  it('«от» > «до»: запроса нет, собрать нельзя; после ухода фокуса подтягивается второе', async () => {
    const { d, go } = await open()
    d.setMinPer(20) // до = 16
    expect(d.inverted).toBe(true)
    await Promise.resolve()
    expect(go.waiting()).toBe(0)
    expect(d.canBuild).toBe(false)
    d.settle()
    expect([d.minPer, d.maxPer]).toEqual([20, 20]) // тронутое «от» осталось
    await go.release()
    await go.release()
    expect(d.inverted).toBe(false)
    expect(d.canBuild).toBe(true)
  })

  it('«до» меньше «от»: подтягивается «от»', async () => {
    const { d, go } = await open({ spreads: 11, range: [5, 12] })
    d.setMaxPer(2)
    d.settle()
    expect([d.minPer, d.maxPer]).toEqual([2, 2])
    await go.release()
    await go.release()
    expect(d.canBuild).toBe(true)
  })

  it('промежуточный ввод «до»: 1 → 18 не ломает диапазон', async () => {
    const { d, go } = await open({ spreads: 11, range: [3, 16] })
    d.setMaxPer(1)
    d.setMaxPer(18)
    d.settle()
    await go.release()
    await go.release()
    expect([d.minPer, d.maxPer]).toEqual([3, 18])
  })

  it('«Собрать» с неподтянутым полем подтягивает и собирает', async () => {
    const { d, go } = await open({ spreads: 11, range: [3, 16] })
    d.setMinPer(30)
    const got: AutoRange[] = []
    const done = d.submit((_n, r) => got.push(r))
    await go.release()
    await go.release()
    expect(await done).toBe(true)
    expect(got).toEqual([{ auto: false, minPer: 30, maxPer: 30, series: true }])
  })

  it('поля зажимаются в 1…64, мусор не трогает', async () => {
    const { d } = await open({ spreads: 11, range: [3, 16] })
    d.setMinPer(0)
    d.setMaxPer(500)
    expect([d.minPer, d.maxPer]).toEqual([1, 64])
    d.setMinPer(NaN)
    expect(d.minPer).toBe(1)
  })

  it('фото меньше «от» — собирается один разворот', async () => {
    const { d, go } = await open({ spreads: 5, range: [8, 10] }, 5)
    await go.release()
    expect(d.n).toBe(1)
    expect(d.canBuild).toBe(true)
  })

  it('мало фото: «Авто» собирается', async () => {
    const { d, go } = await open({ spreads: null }, 5)
    await go.release()
    expect(d.canBuild).toBe(true)
    expect(d.n).toBeGreaterThanOrEqual(1)
  })

  it('флажок серий уходит в запрос, «серий» гаснет вместе с ним', async () => {
    const { d, go } = await open({ spreads: 11 }, 100, 8)
    expect(d.plan?.cuts).toBe(8)
    d.setSeries(false)
    expect(d.series).toBe(false)
    await go.release()
    expect(go.asked.at(-1)?.range.series).toBe(false)
    expect(d.plan?.cuts).toBe(0)
  })

  it('«Собрать»: диапазон и серии уходят в сборку; память — «Авто» или [от, до]', async () => {
    const { d, go } = await open({ spreads: 11, series: false })
    const got: AutoRange[] = []
    await d.submit((_n, r) => got.push(r))
    expect(got).toEqual([{ auto: true, minPer: 1, maxPer: 64, series: false }])
    expect(memOf(got[0])).toBe('auto')
    d.setMinPer(4)
    await drain(go)
    got.length = 0
    await d.submit((_n, r) => got.push(r))
    expect(memOf(got[0])).toEqual([4, 16])
  })

  it('запомненный «Свой» восстанавливается; «от» > «до» в памяти схлопывается', async () => {
    const { d } = await open({ spreads: 11, range: [8, 10], series: true })
    expect([d.auto, d.minPer, d.maxPer]).toEqual([false, 8, 10])
    const go = fakeGo()
    expect(new AutoDialog(ids, go.ask, { spreads: 11, range: [12, 4] }).minPer).toBe(4)
  })

  it('нет фото: собирать нельзя', async () => {
    const go = fakeGo(0)
    const empty: PlanFn = (i, n, r) => go.ask(i, n, r).then((p) => ({ ...p, photos: 0, minSpreads: 0, maxSpreads: 0 }))
    const d = new AutoDialog([], empty, { spreads: null })
    d.start()
    await go.release()
    expect(d.canBuild).toBe(false)
  })
})

// Ответить на все запросы, включая те, что насос задаёт следом (N сдвинулся — спрашиваем заново).
const drain = async (go: ReturnType<typeof fakeGo>) => {
  for (let i = 0; i < 10 && go.waiting() > 0; i++) await go.release()
  await go.release()
}

// 070 В: N от середины диапазона, пока его не трогали руками.
describe('AutoDialog: N при смене диапазона (070 В)', () => {
  it('«Авто» → «Свой 1–3» при 100 фото: N = round(100 / 2), в пределах', async () => {
    const { d, go } = await open({ spreads: null })
    expect(d.n).toBe(25) // по 4 фото
    d.setMinPer(1)
    await drain(go)
    d.setMaxPer(3)
    await drain(go)
    expect([d.minPer, d.maxPer]).toEqual([1, 3])
    expect(d.n).toBe(50)
    expect(d.n).toBeGreaterThanOrEqual(d.plan?.minSpreads ?? 0)
    expect(d.n).toBeLessThanOrEqual(d.plan?.maxSpreads ?? 0)
  })

  it('диапазон меняют дальше («до», «Авто») — N снова от середины', async () => {
    const { d, go } = await open({ spreads: null })
    d.setOwn()
    await drain(go)
    d.setMinPer(8)
    await drain(go)
    d.setMaxPer(12)
    await drain(go)
    expect(d.n).toBe(10) // 100 / 10
    d.setAuto()
    await drain(go)
    expect(d.n).toBe(25)
  })

  it('N набрали руками — смена диапазона его держит (в пределах)', async () => {
    const { d, go } = await open({ spreads: null })
    d.setSpreads(30)
    await drain(go)
    d.setMinPer(2)
    await drain(go)
    d.setMaxPer(4)
    await drain(go)
    expect(d.n).toBe(30)
  })

  it('запомненный N с прошлой сборки: при открытии он, первая смена диапазона пересчитывает от середины', async () => {
    const { d, go } = await open({ spreads: 30 })
    expect(d.n).toBe(30)
    d.setMinPer(1)
    await drain(go)
    d.setMaxPer(3)
    await drain(go)
    expect(d.n).toBe(50)
  })

  it('запомненное «X на разворот» — то же: при открытии N из X, смена диапазона пересчитывает', async () => {
    const { d, go } = await open({ spreads: 20, per: 5, by: 'per' })
    expect(d.n).toBe(20)
    d.setMinPer(1)
    await drain(go)
    d.setMaxPer(3)
    await drain(go)
    expect(d.n).toBe(50)
  })

  it('запомненный N, затем ввод руками, затем смена диапазона — держится', async () => {
    const { d, go } = await open({ spreads: 30 })
    d.setSpreads(31)
    await drain(go)
    d.setMinPer(2)
    await drain(go)
    d.setMaxPer(4)
    await drain(go)
    expect(d.n).toBe(31)
  })

  it('«X фото на разворот» и ↑↓ — тоже ручной ввод', async () => {
    const a = await open({ spreads: null })
    a.d.setPer(5) // N = 20
    await drain(a.go)
    a.d.setMinPer(1)
    await drain(a.go)
    a.d.setMaxPer(3)
    await drain(a.go)
    expect(a.d.anchor).toBe('per')
    expect(a.d.n).toBeGreaterThanOrEqual(a.d.plan?.minSpreads ?? 0)
    expect(a.d.n).not.toBe(50)
    const b = await open({ spreads: null })
    b.d.stepPer(1)
    await drain(b.go)
    b.d.setMinPer(1)
    await drain(b.go)
    b.d.setMaxPer(3)
    await drain(b.go)
    expect(b.d.n).not.toBe(50)
  })

  it('якорь «среднее» выбран без ввода: N всё равно от середины, X следует за ним', async () => {
    const { d, go } = await open({ spreads: null })
    d.activate('per')
    d.setMinPer(1)
    await drain(go)
    d.setMaxPer(3)
    await drain(go)
    expect(d.n).toBe(50)
    expect(d.perTyped).toBe(2) // 100 фото / 50 разворотов
  })

  it('мало фото и «до» меньше «от» (подтяжка) — N не выходит за пределы', async () => {
    const { d, go } = await open({ spreads: null }, 5)
    d.setMinPer(20)
    d.settle()
    await drain(go)
    expect(d.n).toBe(1)
    expect(d.canBuild).toBe(true)
  })
})
