// Щипок трекпада Mac (134): WebKit шлёт gesturestart/change/end, у change scale накопленный от начала жеста.
// Берём отношение соседних событий и отдаём приёмнику множитель «как есть» (Safari масштабирует так же).
// Чистый модуль зоны: без Svelte и состояния проекта, под тест.
export interface PinchLike {
  scale: number
  clientX: number
  clientY: number
}

const STEP_MIN = 0.5 // шаг за одно событие; больше — сбой, не жест
const STEP_MAX = 2
const EPS = 1e-4 // множитель 1 ± EPS — поворот двумя пальцами (scale не меняется)
const STALE_MS = 1500 // жест без событий дольше — считаем оборванным (потерянный gestureend)

const ok = (n: number): boolean => Number.isFinite(n) && n > 0

// Живые жесты: страховка колеса (pinching). Время последнего события — чтобы потерянный gestureend не глушил колесо навсегда.
const live = new Map<PinchTrack, number>()
const now = (): number => (typeof performance === 'undefined' ? Date.now() : performance.now())

export class PinchTrack {
  private prev = 0 // прежний scale; 0 — жеста нет

  get active(): boolean {
    return this.prev > 0
  }
  // База — scale начала жеста, а не 1: он не обязательно ровно 1.
  start(e: PinchLike): void {
    this.prev = ok(e.scale) ? e.scale : 1
    live.set(this, now())
  }
  // Множитель к прежнему событию, в [0,5; 2]; мусор или нет жеста — 1 (прежний scale остаётся).
  step(e: PinchLike): number {
    if (!this.active || !ok(e.scale)) return 1
    live.set(this, now())
    const f = Math.min(STEP_MAX, Math.max(STEP_MIN, e.scale / this.prev))
    this.prev = e.scale
    return f
  }
  // Последний шаг и сброс; повторный gestureend (WebKit#233137) даёт 1.
  end(e: PinchLike): number {
    const f = this.step(e)
    this.cancel()
    return f
  }
  cancel(): void {
    this.prev = 0
    live.delete(this)
  }
}

// Хоть один жест идёт: Ctrl-колесо в это время не зумит (синтетическое колесо старого WebKit).
export function pinching(): boolean {
  const t = now()
  for (const [tr, at] of live) {
    if (t - at > STALE_MS) live.delete(tr) // жест остаётся «идущим» для step: следующее событие вернёт запись
  }
  return live.size > 0
}

export type PinchSink = (factor: number, clientX: number, clientY: number) => void

type GestureEv = Event & Partial<PinchLike>

// Слушатели жеста на узле. preventDefault у каждого события — иначе WebKit пришлёт ещё и Ctrl-колесо (двойной зум).
// skip(target) — над этим жест только гасится (полоса и панели над холстом). Нет clientX/clientY — середина узла.
// Возвращает снятие слушателей.
export function pinchOn(node: HTMLElement, sink: PinchSink, opts: { skip?: (t: EventTarget | null) => boolean } = {}): () => void {
  const track = new PinchTrack()
  const point = (e: GestureEv): PinchLike => {
    let { clientX: x, clientY: y } = e
    if (!Number.isFinite(x) || !Number.isFinite(y)) {
      const r = node.getBoundingClientRect()
      x = r.left + r.width / 2
      y = r.top + r.height / 2
    }
    return { scale: e.scale as number, clientX: x as number, clientY: y as number }
  }
  const give = (f: number, p: PinchLike) => {
    if (Math.abs(f - 1) > EPS) sink(f, p.clientX, p.clientY)
  }
  // skip решается один раз на gesturestart и держится до gestureend: цель жеста не меняется, а конец жеста
  // обязан сбросить состояние, даже если элемент под пальцами уже другой.
  let skipped = false
  const handler = (kind: 'start' | 'change' | 'end') => (ev: Event) => {
    ev.preventDefault()
    const e = ev as GestureEv
    if (kind === 'start') skipped = !!opts.skip?.(e.target)
    const p = point(e)
    if (kind === 'end') {
      const f = track.end(p)
      if (!skipped) give(f, p)
      skipped = false
    } else if (!skipped) {
      if (kind === 'start') track.start(p)
      else give(track.step(p), p)
    }
  }
  const on = { gesturestart: handler('start'), gesturechange: handler('change'), gestureend: handler('end') }
  for (const [name, fn] of Object.entries(on)) node.addEventListener(name, fn, { passive: false })
  return () => {
    for (const [name, fn] of Object.entries(on)) node.removeEventListener(name, fn)
    track.cancel()
  }
}
