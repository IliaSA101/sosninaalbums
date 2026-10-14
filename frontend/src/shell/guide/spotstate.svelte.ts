// Подсветка места (спека 033): кольцо поверх элемента и подпись. Состояние для Spot.svelte; выбор
// элемента и геометрия — в spot.ts (под тестом). Кольцо не класс на элементе: overflow:hidden у ленты
// и холста обрезал бы обводку.
import { SPOT_HINT_MS, SPOT_MS, isCandidate, pickTarget, revealShift, ringBox, type Box } from './spot'

export const reducedMotion = (): boolean => typeof matchMedia === 'function' && matchMedia('(prefers-reduced-motion: reduce)').matches

// Кадр или 50 мс: в фоновой вкладке и в окне без кадров rAF стоит, а ждать результата поиска надо по времени.
const frame = () =>
  new Promise<void>((res) => {
    const t = setTimeout(res, 50)
    requestAnimationFrame(() => {
      clearTimeout(t)
      res()
    })
  })

// Подходит ли элемент: виден, не нулевой, не под inert.
function usable(el: HTMLElement): boolean {
  const r = el.getBoundingClientRect()
  return isCandidate({
    rects: el.getClientRects().length,
    w: r.width,
    h: r.height,
    inert: !!el.closest('[inert]'),
    hidden: getComputedStyle(el).visibility === 'hidden',
  })
}

// Ищем элементы по селектору до ms миллисекунд: панель деталей выезжает, ряд доезжает, меню рисуется после tick.
export async function locate(selector: string, ms = 600): Promise<HTMLElement[]> {
  const end = performance.now() + ms
  for (;;) {
    const found = [...document.querySelectorAll<HTMLElement>(selector)].filter(usable)
    if (found.length > 0 || performance.now() >= end) return found
    await frame()
  }
}

// Подвести элемент в вид: сдвигаем каждого прокручиваемого предка по своей оси. Не scrollIntoView: он двигал бы
// и панели с overflow:hidden, а плавный режим в WebView не работает. Предков с overflow:hidden не трогаем.
export function revealInView(el: HTMLElement): void {
  for (let p = el.parentElement; p; p = p.parentElement) {
    const cs = getComputedStyle(p)
    const y = /(auto|scroll)/.test(cs.overflowY) && p.scrollHeight > p.clientHeight
    const x = /(auto|scroll)/.test(cs.overflowX) && p.scrollWidth > p.clientWidth
    if (!x && !y) continue
    const a = p.getBoundingClientRect()
    const b = el.getBoundingClientRect()
    if (y) p.scrollTop += revealShift(b.top, b.bottom, a.top, a.bottom)
    if (x) p.scrollLeft += revealShift(b.left, b.right, a.left, a.right)
  }
}

// Из найденных — тот, что виднее всего; предки прокручиваются, чтобы он оказался в поле зрения.
export function choose(els: HTMLElement[]): HTMLElement | null {
  const rects = els.map((e) => e.getBoundingClientRect())
  const t = pickTarget(rects, { w: window.innerWidth, h: window.innerHeight })
  if (!t) return null
  const el = els[t.index]
  revealInView(el)
  return el
}

interface Cur {
  el: HTMLElement
  label: string
  hint: string
  reduced: boolean
}

class Spot {
  cur = $state<Cur | null>(null)
  // Кольцо пересчитывается каждый кадр: панель выезжает, ряд доезжает.
  box = $state<Box | null>(null)
  private raf = 0
  private timer: ReturnType<typeof setTimeout> | undefined
  private off: (() => void) | undefined
  private onEnd: (() => void) | undefined
  // Контейнеры с data-guide-reveal, что прячут кнопки до наведения (LayoutBar): на время поиска и показа
  // получают data-spot, и кнопки видны. Иначе круг: скрытую кнопку locate отбросит, а атрибут ставится на найденную.
  private held: HTMLElement[] = []

  // Раскрыть контейнеры, в которых есть искомое; снимается cancel().
  reveal(selector: string) {
    this.cancel()
    this.hold(document.querySelectorAll<HTMLElement>('[data-guide-reveal]'), selector)
  }

  private hold(list: Iterable<HTMLElement>, selector?: string) {
    for (const c of list) {
      if (selector && !c.querySelector(selector)) continue
      c.setAttribute('data-spot', '')
      this.held.push(c)
    }
  }

  flash(el: HTMLElement, label: string, hint = '', onEnd?: () => void) {
    this.cancel()
    this.cur = { el, label, hint, reduced: reducedMotion() }
    this.onEnd = onEnd
    el.setAttribute('data-spot', '') // кнопки, видимые только при наведении, на время показа не прячутся
    const box = el.closest<HTMLElement>('[data-guide-reveal]')
    if (box) this.hold([box])
    this.timer = setTimeout(() => this.cancel(), hint ? SPOT_HINT_MS : SPOT_MS)
    const tick = () => {
      const c = this.cur
      if (!c) return
      if (c.el.isConnected) {
        const r = c.el.getBoundingClientRect()
        this.box = ringBox({ left: r.left, top: r.top, width: r.width, height: r.height })
      } else this.box = null
      this.raf = requestAnimationFrame(tick)
    }
    tick()
    // Щелчок или клавиша гасят подсветку раньше; слушаем со следующего кадра, чтобы не поймать запустивший щелчок.
    void frame().then(() => {
      if (this.cur?.el !== el) return
      const stop = () => this.cancel()
      window.addEventListener('pointerdown', stop, true)
      window.addEventListener('keydown', stop, true)
      this.off = () => {
        window.removeEventListener('pointerdown', stop, true)
        window.removeEventListener('keydown', stop, true)
      }
    })
  }

  cancel() {
    cancelAnimationFrame(this.raf)
    clearTimeout(this.timer)
    this.off?.()
    this.off = undefined
    this.cur?.el.removeAttribute('data-spot')
    for (const c of this.held) c.removeAttribute('data-spot')
    this.held = []
    this.cur = null
    this.box = null
    const end = this.onEnd
    this.onEnd = undefined
    end?.()
  }
}

export const spot = new Spot()
