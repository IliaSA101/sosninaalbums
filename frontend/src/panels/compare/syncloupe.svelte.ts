// Общая лупа окна сравнения (138): жесты мыши, колесо и щипок поверх чистых функций synczoom.ts.
// Образец — PreviewLoupe окна 128; отличие — один общий зум на все ячейки и свой сдвиг у каждой (Shift).
import { primaryPress } from '../../base/core/platform'
import { isZoomWheel, wheelFactor } from '../../editor/zoom/zoom'
import { pinchOn, pinching } from '../../editor/zoom/pinch'
import { fitScale, isClick, MAX_SCALE, place, zoomLabel, type Loupe, type Pt, type Size } from '../../editor/photo/loupe'
import {
  carrySync,
  FIT_SYNC,
  isFit,
  NO_OFF,
  panOne,
  panSync,
  tileLoupe,
  toggleSync,
  zoomSync,
  type Off,
  type Sync,
  type Tile,
} from './synczoom'

const EPS = 1e-6

// Что окно сообщает лупе: кадры ячеек, активная, окно каждой ячейки (без подписи) и сведения о кадре.
export interface Src {
  ids: string[]
  active: number
  boxes: Size[]
  nat(id: string): Size // размер для расчёта: загруженный, до загрузки — по данным фото (только для рамки)
  ready(id: string): boolean // картинка загружена и не заглушка: до этого щелчок и зум ничего не делают
  max(id: string): number // предел зума ячейки: 1 — 1:1 по превью, выше — оригинал
  pick(i: number): void // сделать ячейку активной
}

interface Press {
  i: number
  x: number
  y: number
  start: Sync
  off0: Off
  shift: boolean
  moved: boolean
}

export class SyncLoupe {
  sync = $state<Sync>(FIT_SYNC)
  offs = $state.raw<Record<string, Off>>({}) // свой сдвиг ячейки по id кадра
  pressed = $state(-1) // ячейка, которую тащат
  private mouse: { i: number; pt: Pt } | null = null // последняя точка курсора в ячейке; нет — зум в центре
  private press: Press | null = null

  constructor(private src: () => Src) {}

  private off(id: string): Off {
    return this.offs[id] ?? NO_OFF
  }
  private tile(i: number): Tile | null {
    const s = this.src()
    const id = s.ids[i]
    const box = s.boxes[i]
    if (id === undefined || !box) return null
    return { nat: s.nat(id), box, max: s.max(id), off: this.off(id) }
  }
  private ok(i: number): boolean {
    const s = this.src()
    return s.ready(s.ids[i])
  }

  // Лупа ячейки по общему состоянию.
  loupeAt(i: number): Loupe {
    const t = this.tile(i)
    return t ? tileLoupe(this.sync, t.off, t.nat, t.box, t.max) : { s: 0, cx: 0.5, cy: 0.5 }
  }
  // Где стоит кадр в ячейке, px.
  at(i: number): { left: number; top: number; w: number; h: number } {
    const t = this.tile(i)
    return t ? place(this.loupeAt(i), t.nat, t.box) : { left: 0, top: 0, w: 0, h: 0 }
  }
  // Масштаб ячейки (px экрана на px превью): выше 1:1 нужен оригинал.
  scale(i: number): number {
    const t = this.tile(i)
    return t ? place(this.loupeAt(i), t.nat, t.box).w / (t.nat.w || 1) : 0
  }
  zoomed(i: number): boolean {
    const t = this.tile(i)
    const f = t ? fitScale(t.nat, t.box) : 0
    return f > 0 && this.loupeAt(i).s > f + EPS
  }
  // Есть куда увеличивать (кадр, который и так не меньше 1:1, остаётся вписанным).
  canZoom(i: number): boolean {
    const t = this.tile(i)
    const f = t ? fitScale(t.nat, t.box) : 0
    return f > 0 && f < MAX_SCALE
  }
  // Значок зума активной ячейки: «Вписано · 48 %», «1:1», «130 %».
  get label(): string {
    const a = this.src().active
    const t = this.tile(a)
    return t && this.ok(a) ? zoomLabel(this.loupeAt(a), t.nat, t.box) : ''
  }

  reset() {
    this.sync = FIT_SYNC
    this.offs = {}
    this.mouse = null
    this.press = null
    this.pressed = -1
  }
  // «Вписать» все: свои сдвиги снимаются вместе с зумом.
  fit() {
    this.sync = FIT_SYNC
    this.offs = {}
  }
  forget(id: string) {
    if (!(id in this.offs)) return
    const { [id]: _drop, ...rest } = this.offs
    void _drop
    this.offs = rest
  }
  // Листание ячейки (← →): пропорции те же — зум остаётся, иначе вписаны все; свой сдвиг нового кадра снят.
  carry(id: string, prev: Size, next: Size) {
    this.forget(id)
    this.sync = carrySync(this.sync, prev, next)
    if (isFit(this.sync)) this.offs = {}
  }

  private set(s: Sync) {
    this.sync = s
    if (isFit(s)) this.offs = {}
  }
  private actAt(i: number, at: Pt | null): Pt | null {
    return at ?? (this.mouse && this.mouse.i === i ? this.mouse.pt : null)
  }
  // Z (1:1 у всех): активная ячейка, точка — курсор в ней.
  toggleAt(at?: Pt | null) {
    const a = this.src().active
    const t = this.tile(a)
    if (t && this.ok(a)) this.set(toggleSync(this.sync, t, this.actAt(a, at ?? null)))
  }
  // Mod+= / Mod+−: активная ячейка.
  zoomBy(k: number, i = this.src().active, at?: Pt | null) {
    const t = this.tile(i)
    if (t && this.ok(i)) this.set(zoomSync(this.sync, k, t, this.actAt(i, at ?? null)))
  }

  private pt(e: { clientX: number; clientY: number }, el: Element): Pt {
    const r = el.getBoundingClientRect()
    return { x: e.clientX - r.left, y: e.clientY - r.top }
  }

  // Нажатие на кадр ячейки i: она становится активной; щелчок или перетаскивание решается при отпускании.
  down(i: number, e: PointerEvent): boolean {
    if (!primaryPress(e)) return false
    this.src().pick(i)
    const t = this.tile(i)
    if (!t || !this.ok(i)) return false
    const el = e.currentTarget as HTMLElement
    this.mouse = { i, pt: this.pt(e, el) }
    this.press = { i, x: e.clientX, y: e.clientY, start: this.sync, off0: t.off, shift: e.shiftKey, moved: false }
    el.setPointerCapture?.(e.pointerId)
    return true
  }
  move(i: number, e: PointerEvent) {
    this.mouse = { i, pt: this.pt(e, e.currentTarget as Element) }
    const p = this.press
    if (!p || p.i !== i) return
    const dx = e.clientX - p.x
    const dy = e.clientY - p.y
    if (!p.moved && isClick(dx, dy)) return
    p.moved = true
    if (isFit(p.start)) return // вписанный кадр не двигается
    const t = this.tile(i)
    if (!t) return
    this.pressed = i
    if (p.shift) this.offs = { ...this.offs, [this.src().ids[i]]: panOne(p.start, dx, dy, { ...t, off: p.off0 }) }
    else this.sync = panSync(p.start, dx, dy, { ...t, off: p.off0 })
  }
  up(i: number, e: PointerEvent) {
    const p = this.press
    if (!p || p.i !== i) return
    this.press = null
    this.pressed = -1
    if (!p.moved) this.toggleAt(this.pt(e, e.currentTarget as Element))
  }
  cancel() {
    this.press = null
    this.pressed = -1
  }

  // Ячейка под курсором: колесо зумит у курсора, а не в активной.
  private under(e: { target: EventTarget | null }): { i: number; el: Element } | null {
    const el = (e.target as Element | null)?.closest?.('[data-cview]')
    const i = el ? Number(el.getAttribute('data-cview')) : NaN
    return el && Number.isFinite(i) ? { i, el } : null
  }
  // Ctrl+колесо — зум у курсора у всех; без Ctrl ничего. Посреди щипка Mac колесо не зумит (двойной зум).
  wheel(e: WheelEvent) {
    e.preventDefault()
    if (!isZoomWheel(e) || pinching()) return
    const u = this.under(e)
    if (u) this.zoomBy(wheelFactor(e.deltaY, e.deltaMode), u.i, this.pt(e, u.el))
  }
  // Щипок Mac (134): множитель жеста как есть, у ячейки под точкой жеста (нет такой — активная, в центре).
  pinch(f: number, x: number, y: number) {
    const el = document.elementFromPoint?.(x, y)
    const u = el ? this.under({ target: el }) : null
    if (u) this.zoomBy(f, u.i, this.pt({ clientX: x, clientY: y }, u.el))
    else this.zoomBy(f)
  }
  // Колесо и щипок на узле окна; passive: false — иначе Ctrl+колесо масштабирует страницу. Возвращает снятие.
  bind(node: HTMLElement): () => void {
    const wheel = (e: WheelEvent) => this.wheel(e)
    node.addEventListener('wheel', wheel, { passive: false })
    const off = pinchOn(node, (f, x, y) => this.pinch(f, x, y))
    return () => {
      node.removeEventListener('wheel', wheel)
      off()
    }
  }
}
