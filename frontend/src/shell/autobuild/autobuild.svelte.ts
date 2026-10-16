import { errText, capFirst } from '../../base/words/errtext'
import type { AutoPlan, AutoRange, Variety } from '../../base/core/types'
import type { AutoMem } from '../../base/prefs/view.svelte'
import {
  MAX_PER,
  clampMaxPer,
  clampSpreads,
  defaultSpreads,
  firstSpreads,
  normVariety,
  perStep,
  settleRange,
  spreadsForPer,
  type Anchor,
  type Lim,
} from './autobuild'

export type PlanFn = (ids: readonly string[], spreads: number, range: AutoRange) => Promise<AutoPlan>

// Состояние открытого окна автосборки: N, диапазон «от … до …» (051), среднее X и ответ Go. Запрос в полёте один:
// пока он идёт, новое значение только запоминается и уходит следом (ответ на устаревшее N не показываем).
// Поля N и X связаны (034): X задаёт N = round(n / X); якорь — поле, которое трогали последним, его запоминает окно.
export class AutoDialog {
  n = $state(1)
  // 051: «Авто» — диапазон от среднего (поля показывают plan.from/to); иначе «Свой» [minPer, maxPer].
  auto = $state(true)
  minPer = $state(3)
  maxPer = $state(MAX_PER)
  series = $state(true) // не разрывать серии съёмки
  variety = $state<Variety>(1) // 041: на сам план не влияет (группы те же), уходит в сборку
  anchor = $state<Anchor>('spreads')
  perTyped = $state<number | null>(null) // X, который задал пользователь (для шага — фактическое n / N)
  plan = $state<AutoPlan | null>(null)
  pending = $state(true)
  error = $state('')

  private running = false
  private waiters: (() => void)[] = []
  private firstTime: boolean
  private touched: 'min' | 'max' = 'max' // какое поле диапазона трогали последним

  constructor(
    readonly ids: readonly string[],
    private ask: PlanFn,
    saved: { spreads: number | null; range?: AutoMem; series?: boolean; per?: number | null; by?: Anchor; variety?: Variety },
  ) {
    this.variety = normVariety(saved.variety)
    const byPer = saved.by === 'per' && saved.per != null
    this.firstTime = saved.spreads === null && !byPer
    this.series = saved.series !== false
    if (Array.isArray(saved.range)) {
      this.auto = false
      ;[this.minPer, this.maxPer] = settleRange(clampMaxPer(saved.range[0]), clampMaxPer(saved.range[1]), 'max')
    }
    this.anchor = byPer ? 'per' : 'spreads'
    this.perTyped = byPer ? (saved.per ?? null) : null
    this.n = firstSpreads(ids.length, saved, { min: 1, max: Math.max(1, ids.length) })
  }

  // Фото в сборку: ответ Go, пока его нет — всё, что пришло.
  get photos(): number {
    return this.plan?.photos ?? this.ids.length
  }

  // Пределы ползунка: ответ Go, пока его нет — [1, фото].
  get lim(): Lim {
    const p = this.plan
    return p && p.maxSpreads >= 1 ? { min: Math.max(1, p.minSpreads), max: p.maxSpreads } : { min: 1, max: Math.max(1, this.ids.length) }
  }

  start() {
    void this.pump()
  }

  setSpreads(v: number) {
    if (!Number.isFinite(v)) return
    this.n = clampSpreads(Math.round(v), this.plan) || 1
    this.anchor = 'spreads'
    this.perTyped = null
    void this.pump()
  }

  // Поле выбрано (фокус, щелчок по карточке) без ввода: оно становится якорем сразу, значения не меняются.
  // У «среднего» X — фактическое n / N; уже заданное руками (якорь тот же) не трогаем.
  activate(which: Anchor) {
    if (this.anchor === which) return
    this.anchor = which
    this.perTyped = which === 'per' && this.n > 0 ? this.photos / this.n : null
  }

  // Ввод среднего «X фото на разворот»: N = round(n / X). Мусор — N и якорь не трогаем.
  setPer(x: number) {
    const n = spreadsForPer(this.photos, x, this.lim)
    if (n === null) return
    this.n = n
    this.anchor = 'per'
    this.perTyped = x
    void this.pump()
  }

  // ↑↓ в поле среднего (dir = +1 — среднее больше): показанное целое на 1, N меняется хотя бы на 1.
  stepPer(dir: 1 | -1) {
    if (this.photos < 1) return
    this.n = perStep(this.photos, this.n, dir, this.lim)
    this.anchor = 'per'
    this.perTyped = this.photos / this.n
    void this.pump()
  }

  setVariety(v: Variety) {
    this.variety = v
  }

  // Что уходит в Go: «Авто» поля не использует, «Свой» — как набрано.
  get range(): AutoRange {
    return { auto: this.auto, minPer: this.auto ? 1 : this.minPer, maxPer: this.auto ? MAX_PER : this.maxPer, series: this.series }
  }

  // Что показано в полях: в «Авто» — посчитанное Go (до ответа пусто), в «Свой» — набранное.
  get from(): number | null {
    return this.auto ? (this.plan?.from ?? null) : this.minPer
  }
  get to(): number | null {
    return this.auto ? (this.plan?.to ?? null) : this.maxPer
  }

  // «от» > «до»: пока поле не потеряло фокус, ничего не спрашиваем и собрать нельзя.
  get inverted(): boolean {
    return !this.auto && this.minPer > this.maxPer
  }

  setAuto() {
    if (this.auto) return
    this.auto = true
    void this.pump()
  }

  // Ввод в поле (или выбор «Свой») — «Свой»: сначала поля берут то, что показывали в «Авто».
  private own() {
    if (!this.auto) return
    this.auto = false
    this.minPer = clampMaxPer(this.plan?.from ?? this.minPer)
    this.maxPer = clampMaxPer(this.plan?.to ?? this.maxPer)
  }

  setOwn() {
    if (!this.auto) return
    this.own()
    void this.pump()
  }

  setMinPer(v: number) {
    if (!Number.isFinite(v)) return
    this.own()
    this.minPer = clampMaxPer(v)
    this.touched = 'min'
    void this.pump()
  }

  setMaxPer(v: number) {
    if (!Number.isFinite(v)) return
    this.own()
    this.maxPer = clampMaxPer(v)
    this.touched = 'max'
    void this.pump()
  }

  // Фокус ушёл с поля: «от» > «до» — подтягиваем второе, тронутое не меняется.
  settle() {
    if (!this.inverted) return
    ;[this.minPer, this.maxPer] = settleRange(this.minPer, this.maxPer, this.touched)
    void this.pump()
  }

  setSeries(v: boolean) {
    this.series = v
    void this.pump()
  }

  // Можно собирать: ответ получен, он про текущие значения, фото есть.
  get canBuild(): boolean {
    const p = this.plan
    return (
      !this.pending &&
      !this.error &&
      !this.inverted &&
      !!p &&
      p.photos > 0 &&
      p.maxSpreads >= 1 &&
      this.n >= p.minSpreads &&
      this.n <= p.maxSpreads
    )
  }

  // Ответ на текущее N (или ошибку) дождаться: «Собрать» не уходит с устаревшей подсказкой.
  idle(): Promise<void> {
    return this.running ? new Promise((res) => this.waiters.push(res)) : Promise.resolve()
  }

  // «Собрать»/Enter: подтягивает «от» > «до», ждёт ответа, потом build(N, диапазон, X, якорь, разнообразие); false — собирать нечего.
  async submit(build: (spreads: number, range: AutoRange, per: number, by: Anchor, variety: Variety) => unknown): Promise<boolean> {
    this.settle()
    await this.idle()
    if (!this.canBuild) return false
    await build(this.n, this.range, this.perTyped ?? this.photos / this.n, this.anchor, this.variety)
    return true
  }

  private key = () => `${this.auto}|${this.minPer}|${this.maxPer}|${this.series}`

  private async pump() {
    if (this.running) return
    this.running = true
    this.pending = true
    try {
      for (;;) {
        if (this.inverted) return // ждём ухода фокуса: settle() подтянет второе поле и запустит снова
        const n = this.n
        const k = this.key()
        let res: AutoPlan
        try {
          res = await this.ask(this.ids, n, this.range)
        } catch (e) {
          this.plan = null
          this.error = capFirst(errText(e))
          return
        }
        if (n !== this.n || k !== this.key()) continue // за время ответа значения ушли дальше
        // Первый раз без запомненного N — round(фото / 4) по числу, которое насчитал Go; дальше зажим в пределы ответа.
        // Якорь «среднее»: N заново из X по числу фото и пределам ответа (Go мог взять меньше фото, диапазон сдвинул низ).
        let want = this.firstTime ? defaultSpreads(res.photos) : n
        if (!this.firstTime && this.anchor === 'per' && this.perTyped !== null && res.photos > 0) {
          want = spreadsForPer(res.photos, this.perTyped, { min: Math.max(1, res.minSpreads), max: Math.max(1, res.maxSpreads) }) ?? n
        }
        want = clampSpreads(want, res)
        this.firstTime = false
        if (want !== n) {
          this.n = want
          continue
        }
        this.plan = res
        this.error = ''
        return
      }
    } finally {
      this.running = false
      this.pending = false
      for (const w of this.waiters.splice(0)) w()
    }
  }
}
