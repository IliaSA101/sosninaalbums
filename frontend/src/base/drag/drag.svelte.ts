// Перетаскивание на событиях указателя: так полностью управляем видом
// «призрака» и подсветкой мест, куда можно отпустить.
import { primaryPress } from '../core/platform'
import type { Payload, Target } from '../core/types'
import { store } from '../core/store.svelte'
import { dropOn } from './dropact'
import { edges } from './edgescroll'
import { wheelHint } from './wheelhint.svelte'
import { backdropTarget, binTarget, cellSide, coverTarget, gapOf, halfSide, photoNoop, pickDrop, placeTarget, sheetFrac } from './hit'

const THRESHOLD = 5

// Свой расчёт цели у источника (096: живая перестановка разворотов) вместо общего hit по DOM под курсором.
export interface DragSession {
  hit(x: number, y: number): Target | null
  drop(t: Target | null): void // отпускание: цель есть — приземление, нет — назад
  settle(ok: boolean): void // ответ Go на бросок пришёл (ok) или правка не удалась
  cancel(): void // Esc и любой конец переноса без отпускания
}

class Drag {
  active = $state(false)
  payload = $state<Payload | null>(null)
  x = $state(0)
  y = $state(0)
  target = $state<Target | null>(null)
  // Сессия источника: место показывает он сам, общие подсветки (черты, «+») молчат.
  private session = $state.raw<DragSession | null>(null)
  // Ключ подсвечиваемого места: компоненты сравнивают его со своим.
  over = $derived(this.session ? '' : keyOf(this.target))

  private suppressClick = false
  // Снять слушатели текущего жеста (092 U1): один на все пути конца — бросок, Esc, потеря окна, новый жест.
  private end: (() => void) | null = null
  // Бросок начался: верх убирает то, что мешает целиться (предпросмотр галереи). Подключает shell/boot/wire.ts.
  onStart: () => void = () => {}

  constructor() {
    // Скроллер поехал (автопрокрутка у края, колесо, лента за рядом) — цель под неподвижным курсором другая (095).
    edges.onScroll = () => this.rehit()
  }

  // start вешается на pointerdown источника. make читает данные источника сразу, при нажатии (derived_inert,
  // 075-9: к первому сдвигу плитка ленты уже размонтирована). Побочное — в begin, он идёт только когда
  // мышь действительно сдвинулась: обычный клик остаётся кликом. session — свой расчёт цели (096), создаётся
  // на пороге; sx, sy — точка нажатия.
  start(e: PointerEvent, make: () => Payload, begin?: () => void, session?: (sx: number, sy: number) => DragSession) {
    if (!primaryPress(e)) return
    // Прежний жест потерял отпускание и висит (092 §2): снять без броска, иначе его up бросит старое фото.
    this.cancel()
    const sx = e.clientX
    const sy = e.clientY
    const snap = make()
    const move = (ev: PointerEvent) => {
      // Кнопку отпустили вне окна: pointerup сюда не пришёл.
      if (ev.buttons === 0) return this.cancel()
      if (!this.active) {
        if (Math.hypot(ev.clientX - sx, ev.clientY - sy) < THRESHOLD) return
        begin?.()
        this.payload = snap
        this.session = session?.(sx, sy) ?? null
        this.active = true
        document.body.classList.add('dragging')
        wheelHint.begin(snap.kind === 'photos')
        this.onStart()
      }
      this.x = ev.clientX
      this.y = ev.clientY
      this.target = this.aim(ev.clientX, ev.clientY, this.payload!)
      edges.feed(ev.clientX, ev.clientY)
    }
    const up = () => {
      off()
      if (!this.active) return
      const p = this.payload!
      const t = this.target
      // Сессия получает отпускание, а не отмену: reset её уже не видит.
      const s = this.session
      this.session = null
      this.reset()
      this.suppressClick = true
      setTimeout(() => (this.suppressClick = false), 0)
      s?.drop(t)
      if (!t) return
      const done = dropOn(p, t)
      if (s)
        done.then(
          (ok) => s.settle(ok),
          () => s.settle(false),
        )
    }
    const esc = (ev: KeyboardEvent) => {
      if (ev.key !== 'Escape') return
      ev.stopPropagation() // Esc отменяет только перенос: режимы и меню под ним не берут
      ev.preventDefault()
      this.cancel()
    }
    const lost = () => this.cancel()
    const hidden = () => document.visibilityState === 'hidden' && this.cancel()
    const wheel = () => this.active && wheelHint.wheel()
    const off = () => {
      window.removeEventListener('pointermove', move)
      window.removeEventListener('pointerup', up)
      window.removeEventListener('pointercancel', lost)
      window.removeEventListener('blur', lost)
      window.removeEventListener('keydown', esc, true)
      window.removeEventListener('wheel', wheel, true)
      document.removeEventListener('visibilitychange', hidden)
      if (this.end === off) this.end = null
    }
    this.end = off
    window.addEventListener('pointermove', move)
    window.addEventListener('pointerup', up)
    window.addEventListener('pointercancel', lost)
    window.addEventListener('blur', lost)
    window.addEventListener('keydown', esc, true) // захват: раньше слушателей режимов и меню
    window.addEventListener('wheel', wheel, { capture: true, passive: true })
    document.addEventListener('visibilitychange', hidden)
  }

  // Отмена без броска (Esc, pointercancel, потеря окна, скрытие, новый жест): щелчок после неё не гасится.
  cancel() {
    this.end?.()
    if (this.active) this.reset()
  }

  // Цель под последней точкой указателя: ряд разворотов поехал, а мышь стоит (спека 012).
  rehit() {
    if (this.active && this.payload) this.target = this.aim(this.x, this.y, this.payload)
  }

  private aim(x: number, y: number, p: Payload): Target | null {
    return this.session ? this.session.hit(x, y) : hit(x, y, p)
  }

  // Клик сразу после перетаскивания не должен ничего выделять.
  clickAllowed() {
    return !this.suppressClick
  }

  // Единственная точка конца переноса (095 §5): автопрокрутка и подсказка колеса гаснут здесь.
  private reset() {
    const s = this.session
    this.session = null
    s?.cancel()
    edges.stop()
    wheelHint.end()
    this.active = false
    this.payload = null
    this.target = null
    document.body.classList.remove('dragging')
  }
}

// Цель под курсором (выбор по стопке — pickDrop в hit.ts).
function dropEl(x: number, y: number): HTMLElement | null {
  return pickDrop(document.elementsFromPoint(x, y))
}

type Data = DOMStringMap
type PhotoPayload = Extract<Payload, { kind: 'photos' }>

// Нынешний индекс единственного фото на развороте spread (-1 — тащат не оттуда или не одно).
function fromIndex(p: PhotoPayload, spread: string): number {
  return p.ids.length === 1 && p.fromSpread === spread ? (store.sheet(spread)?.photos.indexOf(p.ids[0]) ?? -1) : -1
}

function cellTarget(d: Data, p: Payload, fx: number): Target | null {
  if (p.kind !== 'photos') return null
  // Обмен — только для фото из разворота; из ленты центр = «вставить после».
  const side = cellSide(fx, !!p.fromSpread)
  if (p.ids.length === 1 && p.ids[0] === d.photo) return null
  const index = +d.index!
  // До/после соседа на своём листе — фото останется где было (095 сц. 7). На холсте щели не сводятся: соседи
  // по порядку не обязательно рядом (сц. 8).
  if (side !== 'center' && photoNoop('cell', p, d.spread!, fromIndex(p, d.spread!), gapOf(index, side))) return null
  return { kind: 'cell', spread: d.spread!, index, photo: d.photo!, side }
}

// Миниатюра фото в ленте: правая половина j и левая половина j+1 — одна щель (095 сц. 6), цель всегда «до щели».
function chipTarget(d: Data, p: PhotoPayload, fx: number): Target | null {
  const at = gapOf(+d.index!, halfSide(fx))
  if (photoNoop('chip', p, d.spread!, fromIndex(p, d.spread!), at)) return null
  return { kind: 'chip', spread: d.spread!, index: at, photo: d.photo!, side: 'before' }
}

function hit(x: number, y: number, p: Payload): Target | null {
  const el = dropEl(x, y)
  if (!el) return null
  // Разворот в ленте и во «Всех разворотах» переносит сессия живой перестановки (096, reorder.svelte.ts): цель с щелью
  // даёт она, а не элемент под курсором. Источника разворота без сессии нет — здесь цели для него не ищем.
  if (p.kind === 'spread') return null
  const d = el.dataset
  const r = el.getBoundingClientRect()
  const fx = (x - r.left) / r.width
  switch (d.drop) {
    case 'cell':
      return cellTarget(d, p, fx)
    case 'spread':
      if (photoNoop('spread', p, d.spread!)) return null // свой лист: бросок ничего не меняет
      // Доля ширины листа выбирает страницу для новых мест (042); у соседа в ряду точки нет — там «свободнее».
      return 'peer' in d ? { kind: 'spread', spread: d.spread! } : { kind: 'spread', spread: d.spread!, at: sheetFrac(fx) }
    case 'chip':
      return chipTarget(d, p, fx)
    case 'mini':
      return photoNoop('mini', p, d.spread!) ? null : mini(el, x)
    case 'new':
      return { kind: 'new', at: +d.at! }
    case 'library':
      return p.fromSpread ? { kind: 'library' } : null
  }
  return zoneTarget(d, p)
}

// Цели-зоны карточки и плитки: место сетки, фон области, плитка обложки.
function zoneTarget(d: Data, p: Payload): Target | null {
  switch (d.drop) {
    case 'place':
      return placeTarget(d, p)
    case 'backdrop':
      return backdropTarget(d, p)
    case 'cover':
      return coverTarget(p)
    case 'bin':
      return binTarget(p)
  }
  return null
}

function mini(el: HTMLElement, x: number): Target {
  const r = el.getBoundingClientRect()
  const side = halfSide((x - r.left) / r.width)
  return { kind: 'mini', spread: el.dataset.spread!, index: +el.dataset.index!, side }
}

function keyOf(t: Target | null): string {
  if (!t) return ''
  switch (t.kind) {
    case 'cell':
      return `cell:${t.photo}:${t.side}`
    case 'spread':
      return `spread:${t.spread}`
    case 'mini':
      return `mini:${t.spread}:${t.side}`
    case 'chip':
      return `chip:${t.spread}:${t.index}`
    case 'gap':
      return '' // щель разворота показывает сессия перестановки (096): общей подсветки нет
    case 'new':
      return `new:${t.at}`
    case 'library':
      return 'library'
    case 'place':
      return `place:${t.place}`
    case 'backdrop':
      return t.many ? '' : `backdrop:${t.scope}`
    case 'cover':
      return t.many ? '' : 'cover'
    case 'bin':
      return 'bin'
  }
}

export const drag = new Drag()
