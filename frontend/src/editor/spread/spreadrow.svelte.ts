// Ряд разворотов на холсте (спека 012, design §3): позиция `pos` в слотах, анимация (rAF), жесты.
// Источник правды о текущем — store.current: ряд двигает картинку и зовёт store.go, сам current не трогает.
// Слоты 0..n-1 — развороты, слот n — «новый» (точка остановки; текущим остаётся последний).
import { tick, untrack } from 'svelte'
import { bgmode } from '../backdrop/bgmode.svelte'
import { previewUrl } from '../../base/core/media'
import { store } from '../../base/core/store.svelte'
import { drag } from '../../base/drag/drag.svelte'
import { IDLE, dur, easeOut, jumpFrom, nearSlot, rowMax, rowWheel, rubber, snapTarget, stepTarget } from './spreadrow'

const THRESHOLD = 5 // px: меньше — это щелчок по соседу, не перетаскивание
const FRESH = 200 // мс после store.go переход считается навигацией (едем), позже — правкой (встаём)
const HIST = 100 // мс: по каким событиям считается скорость броска
const RAW_MAX = 0.86 // резинка насыщается при (raw − край) = 0,3 / 0,35: дальше копить смысла нет

const clamp = (v: number, lo: number, hi: number) => Math.min(hi, Math.max(lo, v))

type Anim = { from: number; to: number; t0: number; d: number }
// Жест (колесо тачпада или перетаскивание): start — слот в начале, raw — позиция без резинки.
type Gesture = { start: number; raw: number; hist: [number, number][] }

class SpreadRow {
  pos = $state(0) // слоты, дробная; в покое = store.current (или n на слоте «новый»)
  moving = $state(false) // жест или анимация: Stage гасит полосу, зум молчит
  private n = $state(0)
  private P = $state(0)
  private zoomed = $state(false)

  max = $derived(rowMax(this.n))
  near = $derived(nearSlot(this.pos, this.max))
  atNew = $derived(this.n >= 1 && this.near === this.n)

  private tgt: number | null = null // куда едет анимация
  private a: Anim | null = null
  private raf = 0
  private g: Gesture | null = null
  private idle: ReturnType<typeof setTimeout> | undefined // таймер конца потока колеса
  private off: (() => void) | null = null // снятие слушателей перетаскивания
  private stepTo = -1 // цель шага (to): follow, запущенный ею, обходится без дальнего подъезда

  // Ряд работает: есть развороты и не режим фона (сценарий 15).
  private get on() {
    return this.n >= 1 && !bgmode.cur
  }

  // Размеры от Stage: шаг ряда P, число разворотов n, зум. Вызывается из эффекта при каждой смене.
  geo(P: number, n: number, zoomed: boolean) {
    untrack(() => {
      this.P = P
      this.n = n
      this.zoomed = zoomed
      if (n < 1) {
        this.halt()
        this.pos = 0
      } else if (zoomed) {
        // В зуме pos ≡ cur, соседей нет
        this.halt()
        this.pos = store.current
      } else if (!this.moving) {
        this.pos = clamp(this.pos, 0, rowMax(n))
      }
    })
  }

  // Текущий сменился или был вызван go(): свежая навигация — едем, правка — встаём на место.
  // navSeq в зависимостях эффекта: go() на тот же разворот (со слота «новый» назад) тоже вернёт ряд.
  follow(c: number) {
    untrack(() => {
      const stepTo = this.stepTo
      this.stepTo = -1
      if (this.n < 1 || this.zoomed) {
        this.halt()
        this.pos = c
        return
      }
      if (this.tgt === c) return // уже едем туда
      if (this.tgt === null && !this.g && this.pos === c) return
      if (performance.now() - store.navAt < FRESH) {
        this.endGesture()
        if (c !== stepTo) this.pos = jumpFrom(this.pos, c)
        this.animate(c)
      } else {
        this.halt()
        this.pos = c
      }
    })
  }

  // ---------- навигация ----------

  // Шаг от цели, если едем, иначе от ближайшего слота; через слот n тоже.
  step(d: -1 | 1) {
    if (this.n < 1) return
    if (this.zoomed) return store.go(clamp(store.current + d, 0, this.n - 1))
    this.to(stepTarget(this.tgt ?? this.near, d, this.pos, this.max))
  }

  // Плавно к слоту i. Разворот — через store.go (выделение снято, галерея закрыта), едет эффект follow.
  to(i: number) {
    if (this.n < 1) return
    // Режим фона: слота «новый» нет (спека 012 п. 15), дальше последнего разворота не едем
    const t = clamp(Math.round(i), 0, this.zoomed || bgmode.cur ? this.n - 1 : this.max)
    if (t < this.n && t !== store.current) {
      // Шаги колеса копятся до 3 слотов от вида: это не дальний переход, вид не телепортируем
      this.stepTo = t
      return store.go(t)
    }
    if (!this.zoomed) this.animate(t)
  }

  // Мгновенно в цель (перед зумом, при потере фокуса окна).
  finish() {
    const t = this.tgt ?? (this.g ? snapTarget(this.g.start, this.pos, 0, this.max, this.P) : null)
    if (t === null) return
    this.stop()
    this.endGesture()
    this.pos = clamp(t, 0, this.max)
    this.settle()
  }

  // ---------- колесо ----------

  // Колесо без Ctrl при 100 %: щелчок мыши (и Shift+колесо) — шаг на слот, тачпад — px. true — разобрано, Stage гасит событие.
  wheel(e: WheelEvent): boolean {
    if (!this.on || this.zoomed) return false
    if (this.off) return true // ряд тащат мышью — колесо ждёт
    const r = rowWheel(e)
    if (!r) return true
    if (r.kind === 'step') this.step(r.dir)
    else this.wheelPx(r.px)
    return true
  }

  private wheelPx(px: number) {
    if (!this.g) this.begin()
    else clearTimeout(this.idle)
    this.feed(this.g!.raw + px / this.P)
    this.idle = setTimeout(() => this.release(true), IDLE)
  }

  // ---------- перетаскивание ----------

  // Начало перетаскивания ряда: сосед (с порогом), средняя кнопка, Пробел. done(moved) — когда жест кончился.
  grab(e: PointerEvent, done?: (moved: boolean) => void): boolean {
    if (!this.on || this.zoomed) return false
    // Жест тачпада в разгаре: доводим до слота, иначе ряд застрянет между слотами (moving, дробная pos).
    if (this.g) this.finish()
    else this.endGesture()
    const x0 = e.clientX
    const y0 = e.clientY
    const slack = !!(e.target as Element | null)?.closest('[data-peer]')
    let moved = false
    let raw0 = 0
    const begin = () => {
      moved = true
      this.begin()
      raw0 = this.g!.raw
    }
    if (!slack) begin()
    const move = (ev: PointerEvent) => {
      if (!moved) {
        if (Math.hypot(ev.clientX - x0, ev.clientY - y0) < THRESHOLD) return
        begin()
      }
      if (this.g) this.feed(raw0 - (ev.clientX - x0) / this.P)
    }
    const up = () => (moved ? this.release(true) : this.endGesture())
    const cancel = () => (moved ? this.release(false) : this.endGesture())
    let ended = false
    const end = () => {
      if (ended) return
      ended = true
      window.removeEventListener('pointermove', move)
      window.removeEventListener('pointerup', up)
      window.removeEventListener('pointercancel', cancel)
      window.removeEventListener('blur', cancel)
      done?.(moved)
    }
    window.addEventListener('pointermove', move)
    window.addEventListener('pointerup', up)
    window.addEventListener('pointercancel', cancel)
    window.addEventListener('blur', cancel)
    this.off = end
    return true
  }

  // ---------- жест ----------

  private begin() {
    const start = this.tgt ?? nearSlot(this.pos, this.max)
    this.stop()
    this.g = { start, raw: this.pos, hist: [] }
    this.moving = true
  }

  private feed(raw: number) {
    const g = this.g!
    g.raw = clamp(raw, -RAW_MAX, this.max + RAW_MAX)
    this.pos = rubber(g.raw, this.max)
    const now = performance.now()
    g.hist.push([now, g.raw])
    while (g.hist.length > 1 && now - g.hist[0][0] > HIST) g.hist.shift()
    this.after()
  }

  // Слотов/мс по последним событиям; пауза перед отпусканием — без броска.
  private velocity(stale: boolean): number {
    const h = this.g!.hist
    if (h.length < 2) return 0
    const [t0, r0] = h[0]
    const [t1, r1] = h[h.length - 1]
    if (stale && performance.now() - t1 > 80) return 0
    return t1 - t0 >= 8 ? (r1 - r0) / (t1 - t0) : 0
  }

  // Конец жеста: доводка к слоту по смещению и скорости (fling = false: отмена, без броска).
  private release(fling: boolean) {
    const g = this.g
    if (!g) return
    const v = fling ? this.velocity(!!this.off) : 0
    const t = snapTarget(g.start, this.pos, v, this.max, this.P)
    this.animate(t)
  }

  private endGesture() {
    clearTimeout(this.idle)
    this.g = null
    const off = this.off
    this.off = null
    off?.()
  }

  // ---------- анимация ----------

  private animate(t: number) {
    t = clamp(t, 0, this.max)
    this.endGesture()
    this.stop()
    if (Math.abs(t - this.pos) < 1e-6) {
      this.pos = t
      this.settle()
      return
    }
    this.tgt = t
    this.a = { from: this.pos, to: t, t0: performance.now(), d: dur(t - this.pos) }
    this.moving = true
    this.raf = requestAnimationFrame(this.tick)
  }

  private tick = (now: number) => {
    this.raf = 0
    const a = this.a
    if (!a) return
    const k = (now - a.t0) / a.d
    if (k >= 1) {
      this.pos = a.to
      this.settle()
      return
    }
    this.pos = a.from + (a.to - a.from) * easeOut(k)
    this.after()
    this.raf = requestAnimationFrame(this.tick)
  }

  // Приехали: pos — ровно слот. Жест фиксирует текущего (вернулись на свой или слот «новый» — go не зовём).
  private settle() {
    this.stop()
    this.moving = false
    const near = this.near
    if (!this.zoomed && near < this.n && near !== store.current) store.go(near)
    this.after()
    this.warm()
  }

  private stop() {
    if (this.raf) cancelAnimationFrame(this.raf)
    this.raf = 0
    this.a = null
    this.tgt = null
  }

  private halt() {
    this.stop()
    this.endGesture()
    this.moving = false
  }

  // Во время переноса цель под курсором устаревает, пока ряд едет: пересчёт после отрисовки кадра.
  private after() {
    if (drag.active) void tick().then(() => drag.rehit())
  }

  // Прогрев превью соседей ±1: при шаге они уже в кэше WebView2 (сами соседи рисуют миниатюры).
  private warm() {
    if (typeof Image === 'undefined') return
    for (const d of [-1, 1]) for (const id of store.view.spreads[store.current + d]?.photos ?? []) new Image().src = previewUrl(id)
  }
}

export const row = new SpreadRow()
