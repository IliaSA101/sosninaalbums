// Состояние лупы окна крупного просмотра (128): жесты мыши и колесо поверх чистых функций loupe.ts.
// Живёт столько же, сколько окно: экземпляр создаёт PhotoPreview.svelte, на закрытии reset().
import { primaryPress } from '../../base/core/platform'
import { isZoomWheel, wheelFactor } from '../zoom/zoom'
import { pinchOn, pinching } from '../zoom/pinch'
import { carry, fitScale, FIT, isClick, MAX_SCALE, pan, place, toggle, zoomAt, zoomLabel, type Loupe, type Pt, type Size } from './loupe'

const NONE: Size = { w: 0, h: 0 }

// Что известно окну о показанном кадре: size — размер загруженной картинки с учётом четверти поворота (0 — не загружена),
// guess — размер по данным фото (до загрузки, только для рамки), off — картинки нет (заглушка «нет файла»),
// max — предел зума (1 — 1:1 по превью, выше — оригинал, ступень 2).
export interface Src {
  size: Size
  guess: Size
  off: boolean
  max: number
}

export class PreviewLoupe {
  l = $state<Loupe>(FIT)
  box = $state<Size>({ w: 0, h: 0 }) // окно под кадром, px (bind:clientWidth/Height)
  stage = $state<HTMLElement>()
  panning = $state(false)
  private last: Size = NONE // размер прошлого загруженного кадра: для переноса при листании
  private mouse: Pt | null = null // последняя точка курсора в окне; нет — зум в центре
  private press: { x: number; y: number; start: Loupe; moved: boolean } | null = null

  constructor(private src: () => Src) {}

  // Размер для расчёта: загруженный; пока грузится соседний кадр — прошлый (картинка на экране прежняя); до первой — по фото.
  private dim(): Size {
    const { size, guess } = this.src()
    return size.w > 0 && size.h > 0 ? size : this.last.w > 0 ? this.last : guess
  }
  // Размер настоящий и картинка есть: до этого щелчок и зум ничего не делают.
  private ready(): boolean {
    const { size, off } = this.src()
    return !off && size.w > 0 && size.h > 0
  }

  get at(): { left: number; top: number; w: number; h: number } {
    return place(this.l, this.dim(), this.box)
  }
  get zoomed(): boolean {
    const f = fitScale(this.dim(), this.box)
    return f > 0 && this.l.s > f + 1e-6
  }
  // Есть куда увеличивать (кадр, который и так не меньше 1:1, остаётся вписанным).
  get canZoom(): boolean {
    const f = fitScale(this.dim(), this.box)
    return f > 0 && f < MAX_SCALE
  }
  get label(): string {
    return this.ready() ? zoomLabel(this.l, this.dim(), this.box) : ''
  }

  reset() {
    this.l = FIT
    this.last = NONE
    this.mouse = null
    this.press = null
    this.panning = false
  }

  // Картинка загрузилась: при листании зум и точка остаются, если пропорции те же.
  loaded(next: Size) {
    this.l = carry(this.l, this.last, next)
    this.last = next
  }

  fit() {
    this.l = FIT
  }
  toggleAt(at?: Pt | null) {
    if (this.ready()) this.l = toggle(this.l, this.dim(), this.box, at ?? this.mouse, MAX_SCALE)
  }
  zoomBy(k: number, at?: Pt | null) {
    if (this.ready()) this.l = zoomAt(this.l, k, this.dim(), this.box, at ?? this.mouse, this.src().max)
  }
  // Предел снизился (оригинал не пришёл, другое фото): растянутое превью возвращается к пределу, центр остаётся.
  clampTo(max: number) {
    if (this.l.s > max + 1e-6 && this.ready()) this.l = zoomAt(this.l, max / this.l.s, this.dim(), this.box, null, max)
  }

  private pt(e: { clientX: number; clientY: number }): Pt {
    const r = this.stage?.getBoundingClientRect()
    return { x: e.clientX - (r?.left ?? 0), y: e.clientY - (r?.top ?? 0) }
  }

  // Нажатие на кадр: щелчок или перетаскивание решается при отпускании. true — жест начат.
  down(e: PointerEvent): boolean {
    if (!primaryPress(e) || !this.ready()) return false
    this.mouse = this.pt(e)
    this.press = { x: e.clientX, y: e.clientY, start: this.l, moved: false }
    this.stage?.setPointerCapture?.(e.pointerId)
    return true
  }
  move(e: PointerEvent) {
    this.mouse = this.pt(e)
    const p = this.press
    if (!p) return
    const dx = e.clientX - p.x
    const dy = e.clientY - p.y
    if (!p.moved && isClick(dx, dy)) return
    p.moved = true
    if (p.start.s <= 0) return // вписанный кадр не двигается
    this.panning = true
    this.l = pan(p.start, dx, dy, this.dim(), this.box)
  }
  up(e: PointerEvent) {
    const p = this.press
    if (!p) return
    this.press = null
    this.panning = false
    if (!p.moved) this.toggleAt(this.pt(e))
  }
  cancel() {
    this.press = null
    this.panning = false
  }

  // Ctrl+колесо — зум у курсора; без Ctrl ничего (тачпад иначе проскакивал бы кадры). Посреди щипка Mac колесо не зумит (двойной зум).
  wheel(e: WheelEvent) {
    e.preventDefault()
    if (!isZoomWheel(e) || pinching()) return
    this.zoomBy(wheelFactor(e.deltaY, e.deltaMode), this.pt(e))
  }
  // Щипок Mac (134): множитель жеста как есть, у точки жеста.
  pinch(f: number, x: number, y: number) {
    this.zoomBy(f, this.pt({ clientX: x, clientY: y }))
  }
  // Колесо и щипок на узле окна; passive: false — иначе preventDefault не сработает и Ctrl+колесо масштабирует страницу.
  // Возвращает снятие. Тот же вызов — в просмотре окон оформления (133).
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
