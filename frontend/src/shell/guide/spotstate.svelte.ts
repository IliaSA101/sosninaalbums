// Подсветка места (спека 033): кольцо поверх элемента и подпись. Состояние для Spot.svelte; выбор
// элемента и геометрия — в spot.ts (под тестом). Кольцо не класс на элементе: overflow:hidden у ленты
// и холста обрезал бы обводку.
import { HOLD_FAST_MS, HOLD_SLOW_MS, SPOT_HINT_MS, SPOT_MS, isCandidate, pickTarget, revealShift, ringBox, type Box } from './spot'
import { spotId } from './show'
import type { SpotKey } from '../guidetext/articles'
import { cardGroups, isGroupHost, isGroupId } from '../../base/prefs/cardgroups.svelte'
import { modes } from '../../base/prefs/view.svelte'

export const reducedMotion = (): boolean => typeof matchMedia === 'function' && matchMedia('(prefers-reduced-motion: reduce)').matches

// Кадр или 50 мс: в фоновой вкладке и в окне без кадров rAF стоит, а ждать результата поиска надо по времени.
// signal — отмена: ожидание кончается сразу, таймер и кадр снимаются (тур закрыт — поиск не живёт дальше).
const frame = (signal?: AbortSignal) =>
  new Promise<void>((res) => {
    if (signal?.aborted) return res()
    let raf = 0
    const done = () => {
      clearTimeout(t)
      cancelAnimationFrame(raf)
      signal?.removeEventListener('abort', done)
      res()
    }
    const t = setTimeout(done, 50)
    raf = requestAnimationFrame(done)
    signal?.addEventListener('abort', done)
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

// Группа карточки (спека 124 §4.4): искомое лежит в свёрнутой группе — раскрыть её и запомнить открытой (как «Показать»
// у раздела панели деталей). Без этого locate отбросит элемент под inert. true — раскрыли хоть одну.
export function unfoldFor(selector: string): boolean {
  let any = false
  for (const el of document.querySelectorAll<HTMLElement>(selector)) {
    const g = el.closest<HTMLElement>('[data-group]')
    const id = g?.dataset.group
    const host = g?.dataset.groupHost
    if (isGroupId(id) && isGroupHost(host) && cardGroups.open(host, id)) any = true
  }
  return any
}

// Ищем элементы по селектору до ms миллисекунд: панель деталей выезжает, ряд доезжает, меню рисуется после tick.
// unfold — раскрывать свёрнутые группы карточек: зовём в каждом круге, не один раз до поиска, чтобы не зависеть от того,
// успел ли элемент смонтироваться. unfolded (если дан) получает true, когда что-то раскрыто.
// signal — отмена: поиск возвращает [] и больше не трогает документ.
export async function locate(selector: string, ms = 600, unfold?: { done: boolean }, signal?: AbortSignal): Promise<HTMLElement[]> {
  const end = performance.now() + ms
  for (;;) {
    if (signal?.aborted) return []
    if (unfold && unfoldFor(selector)) unfold.done = true
    const found = [...document.querySelectorAll<HTMLElement>(selector)].filter(usable)
    if (found.length > 0 || performance.now() >= end) return found
    await frame(signal)
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

// Раскрытая группа или раздел панели растёт 0,45 с (--move), панель выезжает; choose() прокрутил предков один раз, пока
// высота была старой: нижний элемент остался бы за краем (F1 «Показать» при закрытой панели, спека 123 QA). Повторяем
// подводку каждый кадр 1,2 с — образец Inspector.svelte (зона «Фото фона»). Свою прокрутку к разделу панель деталей
// (modes.reveal) на это время отдаёт: она целит в верх раздела и тянула бы поле обратно. Колесо и щелчок пользователя обрывают.
const FOLLOW_MS = 1200
function follow(el: HTMLElement): void {
  const t0 = performance.now()
  let raf = 0
  const stop = () => {
    cancelAnimationFrame(raf)
    window.removeEventListener('wheel', stop, true)
    window.removeEventListener('gesturestart', stop, true)
    window.removeEventListener('pointerdown', stop, true)
  }
  const step = () => {
    if (!el.isConnected) return stop()
    revealInView(el)
    if (performance.now() - t0 < FOLLOW_MS) raf = requestAnimationFrame(step)
    else stop()
  }
  modes.reveal = null
  window.addEventListener('wheel', stop, { capture: true, passive: true })
  window.addEventListener('gesturestart', stop, { capture: true, passive: true }) // щипок Mac (134) — тоже ручное движение
  window.addEventListener('pointerdown', stop, true)
  raf = requestAnimationFrame(step)
}

// Основное место, иначе «⋯» полосы фото (кнопка ушла в меню). Общий поиск «Показать» справочника и тура (113):
// hint — подпись, когда мигает не сама функция (planHint — подсказка плана для найденного основного места).
// signal — отмена (тур закрыт или ищет заново): результат null, документ после отмены не читается.
export async function seek(key: SpotKey, planHint = '', signal?: AbortSignal): Promise<{ el: HTMLElement; hint: string } | null> {
  const id = spotId(key)
  const isMenu = key.startsWith('menu:')
  const sel = isMenu ? `[role="menu"] [data-item="${id}"]` : `[data-guide~="${id}"]`
  if (signal?.aborted) return null
  if (!isMenu) spot.reveal(sel) // кнопки, видимые только при наведении, раскрываются до поиска
  const unfold = { done: false }
  let els = await locate(sel, 600, isMenu ? undefined : unfold, signal)
  let hint = planHint
  if (els.length === 0 && !isMenu) {
    // Кнопка ушла в «⋯» полосы фото: мигает «⋯».
    els = await locate(`[data-guide-hold~="${id}"]`, 100, undefined, signal)
    hint = els.length > 0 ? 'В меню «⋯»' : ''
  }
  if (signal?.aborted) return null
  const el = choose(els)
  // Подводим и без раскрытия группы: панель деталей и раздел «Настроек» могли открыться этим же «Показать».
  if (el && !isMenu) follow(el)
  return el ? { el, hint } : null
}

// hold — кольцо тура (113): без таймера, щелчок и клавиша его не гасят, только cancel().
// onLost — элемент снят из документа (его заменила другая кнопка с тем же data-guide): тур ищет место заново. Зовётся один раз.
export interface FlashOpts {
  hold?: boolean
  onLost?: () => void
}

interface Cur {
  el: HTMLElement
  label: string
  hint: string
  reduced: boolean
  hold: boolean
}

class Spot {
  cur = $state<Cur | null>(null)
  // Кольцо пересчитывается каждый кадр: панель выезжает, ряд доезжает.
  box = $state<Box | null>(null)
  private raf = 0
  private timer: ReturnType<typeof setTimeout> | undefined
  private slow: ReturnType<typeof setTimeout> | undefined
  private off: (() => void) | undefined
  // Ожидание кадра перед подпиской на щелчок/клавишу: cancel() снимает его, иначе таймер переживёт показ.
  private wait: AbortController | undefined
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

  flash(el: HTMLElement, label: string, hint = '', onEnd?: () => void, opts: FlashOpts = {}) {
    this.cancel()
    const hold = !!opts.hold
    this.cur = { el, label, hint, reduced: reducedMotion(), hold }
    this.onEnd = onEnd
    el.setAttribute('data-spot', '') // кнопки, видимые только при наведении, на время показа не прячутся
    const box = el.closest<HTMLElement>('[data-guide-reveal]')
    if (box) this.hold([box])
    if (!hold) this.timer = setTimeout(() => this.cancel(), hint ? SPOT_HINT_MS : SPOT_MS)
    let lost = false
    const measure = () => {
      const c = this.cur
      if (!c) return
      if (c.el.isConnected) {
        const r = c.el.getBoundingClientRect()
        this.box = ringBox({ left: r.left, top: r.top, width: r.width, height: r.height }, { w: window.innerWidth, h: window.innerHeight })
      } else {
        this.box = null
        if (hold && opts.onLost && !lost) {
          lost = true
          setTimeout(() => this.cur?.el === el && opts.onLost?.(), 0)
        }
      }
    }
    const started = performance.now()
    // Обычный показ короткий — кадр за кадром. Держащееся кольцо (тур) первые 1,5 с так же (панель выезжает, ряд доезжает),
    // дальше раз в 250 мс и при изменении размера окна: минута тура не должна жечь кадры.
    const tick = () => {
      if (!this.cur) return
      measure()
      if (!hold || performance.now() - started < HOLD_FAST_MS) this.raf = requestAnimationFrame(tick)
      else this.slow = setTimeout(tick, HOLD_SLOW_MS)
    }
    tick()
    if (hold) {
      const again = () => measure()
      window.addEventListener('resize', again)
      this.off = () => window.removeEventListener('resize', again)
      return
    }
    // Щелчок или клавиша гасят подсветку раньше; слушаем со следующего кадра, чтобы не поймать запустивший щелчок.
    const wait = (this.wait = new AbortController())
    void frame(wait.signal).then(() => {
      if (wait.signal.aborted || this.cur?.el !== el) return
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
    clearTimeout(this.slow)
    this.wait?.abort()
    this.wait = undefined
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
