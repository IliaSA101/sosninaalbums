// Живая перестановка разворотов (096): сессия переноса для ленты и «Все развороты». Поднятый едет за курсором,
// соседи расступаются transform-ом; DOM и порядок не меняются до ответа Go. Решения — в reorder.ts.
import { tick } from 'svelte'
import type { Target } from '../core/types'
import type { DragSession } from './drag.svelte'
import { gapAt, inside, landOf, offsetOf, REORDER_MS, type Area, type Mode, type Off, type Slot } from './reorder'

export interface ReorderSrc {
  mode: Mode
  items: () => HTMLElement[] // группы ленты / плитки сетки по порядку; offsetParent у всех — общий
  origin: () => { x: number; y: number } // где на экране сейчас начало координат offsetLeft/offsetTop
  area: () => Area // видимая часть области: дальше REORDER_OUT — отмена
}

const ZERO: Off = { dx: 0, dy: 0 }
// Сессия, которой нечего делать: перенос ещё приземляется (ответ Go не пришёл) — второй жест ничего не двигает.
const IDLE: DragSession = { hit: () => null, drop: () => {}, settle: () => {}, cancel: () => {} }

const frame = () => new Promise<void>((r) => requestAnimationFrame(() => r()))

export class Reorder {
  phase = $state<'idle' | 'drag' | 'land'>('idle')
  from = $state(-1)
  to = $state(-1)
  out = $state(false)
  src = $state<Off>(ZERO)
  // Смещения ещё не на местах (жест или 150 мс возврата): лента держит черты скрытыми, группы — нарисованными.
  busy = $state(false)
  // Бросок: переходы выключены, пока новый порядок не встанет в DOM и смещения не снимутся.
  landing = $state(false)

  private slots: Slot[] = []
  private mode: Mode = 'line'
  private seq = 0 // номер сессии: запоздалый ответ прежней не трогает новую
  private timer: ReturnType<typeof setTimeout> | undefined

  // transform для элемента i (undefined — на своём месте).
  css(i: number): string | undefined {
    if (this.phase === 'idle') return undefined
    const o =
      i === this.from
        ? this.phase === 'land'
          ? landOf(this.slots, this.from, this.to, this.mode)
          : this.out
            ? ZERO
            : this.src
        : this.out
          ? ZERO
          : offsetOf(this.slots, i, this.from, this.to, this.mode)
    return o.dx || o.dy ? `translate(${o.dx}px, ${o.dy}px)` : undefined
  }

  // Элемент i едет за курсором: без перехода и поверх соседей.
  lifted(i: number): boolean {
    return this.phase === 'drag' && i === this.from && !this.out
  }

  // Начало жеста (после порога): один замер слотов. sx, sy — точка нажатия на экране.
  session(s: ReorderSrc, from: number, sx: number, sy: number): DragSession {
    if (this.phase === 'land') return IDLE
    // offsetLeft/offsetTop не видят transform: замер верен и посреди возврата прежнего жеста.
    const slots = s.items().map((el) => ({ x: el.offsetLeft, y: el.offsetTop, w: el.offsetWidth, h: el.offsetHeight }))
    if (from < 0 || from >= slots.length) return IDLE
    const id = ++this.seq
    const o0 = s.origin()
    const p0 = { x: sx - o0.x, y: sy - o0.y }
    this.slots = slots
    this.mode = s.mode
    clearTimeout(this.timer)
    this.from = from
    this.to = from
    this.out = false
    this.src = ZERO
    this.landing = false
    this.busy = true
    this.phase = 'drag'
    const mine = () => id === this.seq
    return {
      hit: (x, y) => (mine() && this.phase === 'drag' ? this.hit(s, p0, x, y) : null),
      drop: (t) => {
        if (!mine()) return
        if (t) {
          this.landing = true
          this.phase = 'land'
        } else this.back()
      },
      settle: (ok) => {
        if (mine()) void this.settle(ok)
      },
      cancel: () => {
        if (mine()) this.back()
      },
    }
  }

  // Курсор в (x, y): поднятый — за курсором (с учётом прокрутки области), промежуток — по точке.
  private hit(s: ReorderSrc, p0: { x: number; y: number }, x: number, y: number): Target | null {
    const o = s.origin()
    const px = x - o.x
    const py = y - o.y
    this.src = { dx: px - p0.x, dy: this.mode === 'grid' ? py - p0.y : 0 }
    this.out = !inside(s.area(), x, y)
    const to = this.out ? this.from : gapAt(this.slots, this.from, px, py, this.mode)
    this.to = to
    if (to === this.from) return null
    return { kind: 'gap', at: to > this.from ? to + 1 : to, to }
  }

  // Назад на места с переходом (отмена, бросок мимо, ошибка Go).
  private back() {
    this.landing = false
    this.phase = 'idle'
    this.rest(REORDER_MS)
  }

  private async settle(ok: boolean) {
    if (!ok || this.phase !== 'land') return this.back()
    const id = this.seq
    // Новый порядок уже в DOM: смещения снимаются в том же кадре, переходы выключены ещё два кадра —
    // иначе снятие transform и класса попадут в один пересчёт стилей и группы поедут.
    await tick()
    if (id !== this.seq) return
    this.phase = 'idle'
    await frame()
    await frame()
    if (id !== this.seq) return
    this.landing = false
    this.busy = false
  }

  private rest(ms: number) {
    clearTimeout(this.timer)
    const id = this.seq
    this.timer = setTimeout(() => {
      if (id === this.seq && this.phase === 'idle') this.busy = false
    }, ms)
  }
}

// Лента: offsetParent групп — сама лента (position: relative), координаты от её внутреннего края, с прокруткой.
export function lineSource(box: HTMLElement): ReorderSrc {
  return {
    mode: 'line',
    items: () => Array.from(box.querySelectorAll<HTMLElement>('[data-drop="mini"]')),
    origin: () => {
      const r = box.getBoundingClientRect()
      return { x: r.left + box.clientLeft - box.scrollLeft, y: r.top + box.clientTop - box.scrollTop }
    },
    area: () => box.getBoundingClientRect(),
  }
}

// Сетка «Все развороты»: offsetParent плиток — .grid (position: relative), она сама едет с прокруткой панели.
// Видимая часть — пересечение сетки и прокручиваемой панели.
export function gridSource(grid: HTMLElement, scroller: () => HTMLElement | null): ReorderSrc {
  return {
    mode: 'grid',
    items: () => Array.from(grid.querySelectorAll<HTMLElement>('[data-tile]')),
    origin: () => {
      const r = grid.getBoundingClientRect()
      return { x: r.left, y: r.top }
    },
    area: () => {
      const g = grid.getBoundingClientRect()
      const v = scroller()?.getBoundingClientRect() ?? g
      return {
        left: Math.max(g.left, v.left),
        top: Math.max(g.top, v.top),
        right: Math.min(g.right, v.right),
        bottom: Math.min(g.bottom, v.bottom),
      }
    },
  }
}
