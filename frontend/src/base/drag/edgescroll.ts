// Автопрокрутка у края при переносе (095 §А): лента разворотов и панель деталей сами едут, когда фото или
// разворот подтащили к их краю. Чистая часть — скорость, шаг, задержка входа; ниже — один общий цикл кадров
// по зарегистрированным скроллерам. drag не импортируется (нет цикла): он зовёт feed/stop и ставит onScroll.

export type Axis = 'x' | 'y'
export interface Box {
  left: number
  top: number
  width: number
  height: number
}

export const EDGE_MAX = 48 // зона у края, px (не больше)
export const EDGE_PART = 0.15 // и не больше этой доли размера скроллера
export const EDGE_VMAX = 18 // px за кадр 60 Гц у самого края
export const EDGE_DELAY = 150 // мс в зоне до начала: быстрый проход ленту не трогает
export const FRAME_MS = 1000 / 60
const DT_MAX = 50 // после паузы вкладки не прыгаем

export const edgeZone = (size: number): number => Math.min(EDGE_MAX, size * EDGE_PART)

// px за кадр 60 Гц со знаком: < 0 — к началу, > 0 — к концу, 0 — вне зоны. Поперёк — весь прямоугольник скроллера;
// вдоль — глубина в зоне квадратично; за краем не дальше зоны — полная скорость, дальше — ушёл в другую панель.
export function edgeSpeed(box: Box, axis: Axis, x: number, y: number): number {
  const [pos, start, size, cross, cStart, cSize] =
    axis === 'x' ? [x, box.left, box.width, y, box.top, box.height] : [y, box.top, box.height, x, box.left, box.width]
  if (!(size > 0) || cross < cStart || cross > cStart + cSize) return 0
  const z = edgeZone(size)
  const toStart = pos - start
  const toEnd = start + size - pos
  const dist = Math.min(toStart, toEnd)
  if (dist >= z || dist < -z) return 0
  const d = dist < 0 ? 1 : (z - dist) / z
  const v = EDGE_VMAX * d * d
  return toStart <= toEnd ? -v : v
}

// Новая прокрутка через dt мс при скорости speed (px за кадр 60 Гц), зажата в [0, max].
export function edgeStep(speed: number, dt: number, pos: number, max: number): number {
  const t = Math.min(Math.max(dt, 0), DT_MAX)
  return Math.min(max, Math.max(0, pos + (speed * t) / FRAME_MS))
}

// Задержка входа: true — курсор в зоне не меньше EDGE_DELAY; выход из зоны сбрасывает.
export class EdgeDwell {
  private since = -1
  feed(inZone: boolean, now: number): boolean {
    if (!inZone) {
      this.since = -1
      return false
    }
    if (this.since < 0) this.since = now
    return now - this.since >= EDGE_DELAY
  }
  reset() {
    this.since = -1
  }
}

interface Scroller {
  axis: Axis
  dwell: EdgeDwell
}

const nodes = new Map<HTMLElement, Scroller>()
let live = false // идёт перенос: прокрутка скроллера пересчитывает цель
let px = 0
let py = 0
let raf = 0
let last = 0

const clock = () => performance.now()

// Скорость узла с учётом того, можно ли ещё ехать в ту сторону. Закрытая панель (inert) не едет.
function speedOf(node: HTMLElement, s: Scroller): { v: number; pos: number; max: number } {
  const x = s.axis === 'x'
  const pos = x ? node.scrollLeft : node.scrollTop
  const max = x ? node.scrollWidth - node.clientWidth : node.scrollHeight - node.clientHeight
  if (max <= 0 || node.closest('[inert]')) return { v: 0, pos, max }
  const v = edgeSpeed(node.getBoundingClientRect(), s.axis, px, py)
  // Упёрлись в конец прокрутки — стоп: цикл не крутится вхолостую.
  if ((v < 0 && pos <= 0) || (v > 0 && pos >= max)) return { v: 0, pos, max }
  return { v, pos, max }
}

// Один проход по скроллерам: true — хоть один в зоне (ждёт задержку или едет), цикл нужен дальше.
function pass(now: number, dt: number): boolean {
  let need = false
  for (const [node, s] of nodes) {
    const { v, pos, max } = speedOf(node, s)
    const ready = s.dwell.feed(v !== 0, now)
    if (v === 0) continue
    need = true
    if (!ready || dt <= 0) continue
    const next = edgeStep(v, dt, pos, max)
    if (s.axis === 'x') node.scrollLeft = next
    else node.scrollTop = next
  }
  return need
}

function frame() {
  raf = 0
  if (!live) return
  const now = clock()
  const dt = now - last
  last = now
  if (pass(now, dt)) raf = requestAnimationFrame(frame)
}

export const edges = {
  // Прокрутка скроллера во время переноса (шаг автопрокрутки, колесо, лента за рядом) — пересчитать цель.
  onScroll: () => {},
  // Каждый pointermove переноса: курсор неподвижен — цикл идёт по последней точке.
  feed(x: number, y: number) {
    live = true
    px = x
    py = y
    if (raf) return
    const now = clock()
    if (pass(now, 0)) {
      last = now
      raf = requestAnimationFrame(frame)
    }
  },
  // Конец переноса любым путём (drag.reset): стоп сразу.
  stop() {
    live = false
    if (raf) cancelAnimationFrame(raf)
    raf = 0
    for (const s of nodes.values()) s.dwell.reset()
  },
  // Для тестов: идёт ли цикл.
  get running() {
    return raf !== 0
  },
}

// use:edgeScroll={{ axis }} — скроллер, который едет у края при переносе. opts.step (ступенчатый режим для зон «‹ ›»)
// — задел, в 0.22 не реализован.
export function edgeScroll(node: HTMLElement, opts: { axis: Axis; step?: number }) {
  const s: Scroller = { axis: opts.axis, dwell: new EdgeDwell() }
  nodes.set(node, s)
  const scroll = () => {
    if (live) edges.onScroll()
  }
  node.addEventListener('scroll', scroll, { passive: true })
  return {
    update(o: { axis: Axis; step?: number }) {
      s.axis = o.axis
    },
    destroy() {
      nodes.delete(node)
      node.removeEventListener('scroll', scroll)
    },
  }
}
