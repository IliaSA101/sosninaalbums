// Зум и панорама холста (спека 008, design §6.1). Состояние — в мм разворота и только в памяти
// окна: после перезапуска разворот вписан. Окно (Viewport) задаёт Stage по своим размерам.
import { clampZoom, effH, fit, MIN_ZOOM, panBy, spaceTap, zoomAt, type Viewport, type ZoomState } from '../zoom/zoom'

class Canvas {
  s = $state<ZoomState>({ z: 1, cx: 0, cy: 0 })
  vp = $state<Viewport>({ w: 0, h: 0, k0: 0, sw: 0, ph: 0 })
  // Элемент окна холста (Stage ставит): мини-панель по его прямоугольнику узнаёт видимую область.
  win = $state.raw<HTMLElement | null>(null)
  // Действующее состояние: ресайз, крупный просмотр и галерея меняют окно, мм не трогаются.
  cur = $derived(clampZoom(this.s, this.vp))
  zoomed = $derived(this.cur.z > 1)

  // Идёт жест (перенос текста, кадр, места): колесо не меняет масштаб посреди жеста.
  busy = $state(false)
  // Левая кнопка зажата над холстом — любой жест мог начаться (ставит Stage).
  pressed = $state(false)

  // Пробел: зажат ли, когда нажат, с Shift ли, была ли панорама.
  space = $state(false)
  panning = $state(false)
  private spaceAt = 0
  private spaceShift = false
  private panned = false

  // Счётчик смен масштаба (z): бегунок (спека 016) по нему показывается сам. Сдвиг листа без смены z не считается.
  zoomSeq = $state(0)

  private set(next: ZoomState) {
    const was = this.cur.z
    this.s = next
    if (this.cur.z !== was) this.zoomSeq++
  }
  fit() {
    this.set(fit(this.vp.sw, this.vp.ph))
  }
  zoomAt(factor: number, x: number, y: number) {
    this.set(zoomAt(this.cur, this.vp, factor, x, y))
  }
  // Масштаб из бегунка: вокруг центра окна, как у Ctrl+=; левый край шкалы — «вписать».
  setZoom(z: number) {
    if (!(z > MIN_ZOOM)) return this.fit()
    this.zoomAt(z / this.cur.z, this.vp.w / 2, effH(this.vp) / 2)
  }
  panBy(dx: number, dy: number) {
    this.s = panBy(this.cur, this.vp, dx, dy)
  }

  spaceDown(shift: boolean) {
    if (this.space) return // автоповтор
    this.space = true
    this.spaceAt = performance.now()
    this.spaceShift = shift
    this.panned = false
  }
  // Отпустили Пробел. Возвращает шаг раскладки (±1) для короткого нажатия без панорамы, иначе 0.
  spaceUp(): number {
    if (!this.space) return 0
    this.space = false
    const tap = spaceTap(performance.now() - this.spaceAt, this.panned || this.panning)
    return tap ? (this.spaceShift ? -1 : 1) : 0
  }
  // Панорама мышью (колёсико или Пробел + левая кнопка).
  startPan() {
    this.panning = true
    if (this.space) this.panned = true
  }
  endPan() {
    this.panning = false
  }
  // Окно потеряло фокус: keyup и pointerup могут не прийти.
  reset() {
    this.space = false
    this.panning = false
    this.pressed = false
  }
}

export const canvas = new Canvas()
