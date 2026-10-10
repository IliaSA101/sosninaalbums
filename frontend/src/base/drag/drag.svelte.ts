// Перетаскивание на событиях указателя: так полностью управляем видом
// «призрака» и подсветкой мест, куда можно отпустить.
import type { Payload, Target } from '../core/types'
import { store } from '../core/store.svelte'
import { dropOn } from './dropact'
import { backdropTarget, cellSide, halfSide, isNoopMove, placeTarget } from './hit'

const THRESHOLD = 5

class Drag {
  active = $state(false)
  payload = $state<Payload | null>(null)
  x = $state(0)
  y = $state(0)
  target = $state<Target | null>(null)
  // Ключ подсвечиваемого места: компоненты сравнивают его со своим.
  over = $derived(keyOf(this.target))

  private suppressClick = false
  // Бросок начался: верх убирает то, что мешает целиться (предпросмотр галереи). Подключает shell/boot/wire.ts.
  onStart: () => void = () => {}

  // start вешается на pointerdown источника. make вызывается, только
  // если мышь действительно сдвинулась — обычный клик остаётся кликом.
  start(e: PointerEvent, make: () => Payload) {
    if (e.button !== 0) return
    const sx = e.clientX
    const sy = e.clientY
    const move = (ev: PointerEvent) => {
      if (!this.active) {
        if (Math.hypot(ev.clientX - sx, ev.clientY - sy) < THRESHOLD) return
        this.payload = make()
        this.active = true
        document.body.classList.add('dragging')
        this.onStart()
      }
      this.x = ev.clientX
      this.y = ev.clientY
      this.target = hit(ev.clientX, ev.clientY, this.payload!)
    }
    const up = () => {
      window.removeEventListener('pointermove', move)
      window.removeEventListener('pointerup', up)
      window.removeEventListener('keydown', esc)
      if (!this.active) return
      const p = this.payload!
      const t = this.target
      this.reset()
      this.suppressClick = true
      setTimeout(() => (this.suppressClick = false), 0)
      if (t) dropOn(p, t)
    }
    const esc = (ev: KeyboardEvent) => {
      if (ev.key === 'Escape') {
        window.removeEventListener('pointermove', move)
        window.removeEventListener('pointerup', up)
        window.removeEventListener('keydown', esc)
        this.reset()
      }
    }
    window.addEventListener('pointermove', move)
    window.addEventListener('pointerup', up)
    window.addEventListener('keydown', esc)
  }

  // Цель под последней точкой указателя: ряд разворотов поехал, а мышь стоит (спека 012).
  rehit() {
    if (this.active && this.payload) this.target = hit(this.x, this.y, this.payload)
  }

  // Клик сразу после перетаскивания не должен ничего выделять.
  clickAllowed() {
    return !this.suppressClick
  }

  private reset() {
    this.active = false
    this.payload = null
    this.target = null
    document.body.classList.remove('dragging')
  }
}

// Цель под курсором. Блок текста лежит поверх мест сетки (data-through): бросок на него попадает
// в место под ним, а не в разворот. Под текстом нет места — цель по-прежнему ближайшая сверху.
function dropEl(x: number, y: number): HTMLElement | null {
  const stack = document.elementsFromPoint(x, y)
  const top = stack[0]?.closest<HTMLElement>('[data-drop]') ?? null
  if (!stack[0]?.closest('[data-through]')) return top
  return (stack.find((e) => (e as HTMLElement).dataset?.drop === 'place') as HTMLElement | undefined) ?? top
}

type Data = DOMStringMap
type PhotoPayload = Extract<Payload, { kind: 'photos' }>

function cellTarget(d: Data, p: Payload, fx: number): Target | null {
  if (p.kind !== 'photos') return null
  // Обмен — только для фото из разворота; из ленты центр = «вставить после».
  const side = cellSide(fx, !!p.fromSpread)
  if (p.ids.length === 1 && p.ids[0] === d.photo) return null
  return { kind: 'cell', spread: d.spread!, index: +d.index!, photo: d.photo!, side }
}

function chipTarget(d: Data, p: PhotoPayload, fx: number): Target | null {
  const side = halfSide(fx)
  const index = +d.index!
  if (p.ids.length === 1 && p.fromSpread === d.spread) {
    // Бросок на себя или на соседнюю позицию ничего не меняет — не подсвечиваем.
    const from = store.view.spreads.find((s) => s.id === d.spread)?.photos.indexOf(p.ids[0]) ?? -1
    if (isNoopMove(from, index, side)) return null
  }
  return { kind: 'chip', spread: d.spread!, index, photo: d.photo!, side }
}

function hit(x: number, y: number, p: Payload): Target | null {
  const el = dropEl(x, y)
  if (!el) return null
  const d = el.dataset
  const r = el.getBoundingClientRect()
  const fx = (x - r.left) / r.width
  switch (d.drop) {
    case 'cell':
      return cellTarget(d, p, fx)
    case 'spread':
      return p.kind === 'photos' ? { kind: 'spread', spread: d.spread! } : null
    case 'chip':
      // Группу (разворот) тащат над её миниатюрами — цель та же, что у подложки группы.
      if (p.kind === 'spread') return mini(el.closest<HTMLElement>('[data-drop="mini"]'), x)
      return chipTarget(d, p, fx)
    case 'mini':
      return mini(el, x)
    case 'new':
      return p.kind === 'photos' ? { kind: 'new', at: +d.at! } : null
    case 'library':
      return p.kind === 'photos' && p.fromSpread ? { kind: 'library' } : null
    case 'place':
      return placeTarget(d, p)
    case 'backdrop':
      return backdropTarget(d, p)
  }
  return null
}

function mini(el: HTMLElement | null, x: number): Target | null {
  if (!el) return null
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
      return `chip:${t.photo}:${t.side}`
    case 'new':
      return `new:${t.at}`
    case 'library':
      return 'library'
    case 'place':
      return `place:${t.place}`
    case 'backdrop':
      return t.many ? '' : `backdrop:${t.scope}`
  }
}

export const drag = new Drag()
