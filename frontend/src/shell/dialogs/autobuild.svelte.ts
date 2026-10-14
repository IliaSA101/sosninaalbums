import { errText, capFirst } from '../../base/core/errtext'
import type { AutoPlan, Variety } from '../../base/core/types'
import {
  MAX_PER,
  clampMaxPer,
  clampSpreads,
  defaultSpreads,
  firstSpreads,
  normVariety,
  perStep,
  spreadsForPer,
  type Anchor,
  type Lim,
} from './autobuild'

export type PlanFn = (ids: readonly string[], spreads: number, maxPer: number) => Promise<AutoPlan>

// Состояние открытого окна автосборки: N, «не больше m», среднее X и ответ Go. Запрос в полёте один:
// пока он идёт, новое значение только запоминается и уходит следом (ответ на устаревшее N не показываем).
// Поля N и X связаны (034): X задаёт N = round(n / X); якорь — поле, которое трогали последним, его запоминает окно.
export class AutoDialog {
  n = $state(1)
  maxPer = $state(MAX_PER)
  variety = $state<Variety>(1) // 041: на сам план не влияет (группы те же), уходит в сборку
  anchor = $state<Anchor>('spreads')
  perTyped = $state<number | null>(null) // X, который задал пользователь (для шага — фактическое n / N)
  plan = $state<AutoPlan | null>(null)
  pending = $state(true)
  error = $state('')

  private running = false
  private waiters: (() => void)[] = []
  private firstTime: boolean

  constructor(
    readonly ids: readonly string[],
    private ask: PlanFn,
    saved: { spreads: number | null; maxPer: number; per?: number | null; by?: Anchor; variety?: Variety },
  ) {
    this.variety = normVariety(saved.variety)
    const byPer = saved.by === 'per' && saved.per != null
    this.firstTime = saved.spreads === null && !byPer
    this.maxPer = clampMaxPer(saved.maxPer)
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

  // ↑↓ в поле среднего (dir = +1 — среднее больше): ≈ 0,5 фото, N меняется хотя бы на 1.
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

  setMaxPer(v: number) {
    this.maxPer = clampMaxPer(v)
    void this.pump()
  }

  // Можно собирать: ответ получен, он про текущие значения, фото есть.
  get canBuild(): boolean {
    const p = this.plan
    return !this.pending && !this.error && !!p && p.photos > 0 && p.maxSpreads >= 1 && this.n >= p.minSpreads && this.n <= p.maxSpreads
  }

  // Ответ на текущее N (или ошибку) дождаться: «Собрать» не уходит с устаревшей подсказкой.
  idle(): Promise<void> {
    return this.running ? new Promise((res) => this.waiters.push(res)) : Promise.resolve()
  }

  // «Собрать»/Enter: ждёт ответа, потом build(N, m, X, якорь, разнообразие); false — собирать нечего.
  async submit(build: (spreads: number, maxPer: number, per: number, by: Anchor, variety: Variety) => unknown): Promise<boolean> {
    await this.idle()
    if (!this.canBuild) return false
    await build(this.n, this.maxPer, this.perTyped ?? this.photos / this.n, this.anchor, this.variety)
    return true
  }

  private async pump() {
    if (this.running) return
    this.running = true
    this.pending = true
    try {
      for (;;) {
        const n = this.n
        const m = this.maxPer
        let res: AutoPlan
        try {
          res = await this.ask(this.ids, n, m)
        } catch (e) {
          this.plan = null
          this.error = capFirst(errText(e))
          return
        }
        if (n !== this.n || m !== this.maxPer) continue // за время ответа значения ушли дальше
        // Первый раз без запомненного N — round(фото / 4) по числу, которое насчитал Go; дальше зажим в пределы ответа.
        // Якорь «среднее»: N заново из X по числу фото и пределам ответа (Go мог взять меньше фото, «не больше» сдвинул низ).
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
