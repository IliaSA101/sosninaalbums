// Ctrl/⌘/Alt+колесо и щипок Mac (134) над плитками окна оформления — их размер (133), как над лентой фото (053).
// Накопитель и шаг — общие с лентой (library/thumbzoom).
import { tick } from 'svelte'
import { thumbWheelAcc, isThumbWheel } from '../library/thumbzoom'
import { pinchOn, pinching } from '../../editor/zoom/pinch'
import type { DecorKind } from '../decor/decor'
import { setTile, tileOf, tileRange } from './tilesize'

// Плитка под точкой (x, y) и где точка в ней: после смены размера та же плитка встаёт под тот же курсор, как у ленты фото.
// Курсор в щели между плитками или за краем — null (прокрутка сохранит долю высоты).
function anchorAt(node: HTMLElement, x: number, y: number): { el: Element; cy: number; fy: number } | null {
  const nr = node.getBoundingClientRect()
  const inside = (r: DOMRect) => x >= r.left && x < r.right && y >= r.top && y < r.bottom
  if (!inside(nr)) return null
  for (const el of Array.from(node.children)) {
    const r = el.getBoundingClientRect()
    if (inside(r)) return { el, cy: y - nr.top, fy: r.height > 0 ? (y - r.top) / r.height : 0 }
  }
  return null
}

// Действие на контейнере плиток. Дельты копятся до кадра; прокрутка держит плитку под курсором,
// без неё — долю высоты, чтобы протяжка не уносила от просматриваемого места.
export function tileWheel(node: HTMLElement, kind: DecorKind) {
  let k = kind
  const mk = () => thumbWheelAcc(tileRange(k).lo, tileRange(k).hi)
  let acc = mk()
  let raf = 0
  let wx = 0
  let wy = 0

  async function apply() {
    raf = 0
    const n = acc.take()
    if (n === tileOf(k)) return
    const a = anchorAt(node, wx, wy)
    const ratio = node.scrollHeight > 0 ? node.scrollTop / node.scrollHeight : 0
    setTile(k, n)
    await tick() // высота уже новая, иначе браузер подрежет прокрутку
    if (a?.el.isConnected) {
      const r = a.el.getBoundingClientRect()
      node.scrollTop += r.top + a.fy * r.height - (node.getBoundingClientRect().top + a.cy)
    } else node.scrollTop = ratio * node.scrollHeight
  }
  function order(first: boolean, x: number, y: number) {
    wx = x
    wy = y
    if (first && !raf) raf = requestAnimationFrame(() => void apply())
  }
  function wheel(e: WheelEvent) {
    if (!isThumbWheel(e)) return
    e.preventDefault() // иначе WebView2 масштабирует страницу
    if (pinching()) return // посреди щипка Ctrl-колесо (старый WebKit) не меняет размер второй раз
    order(acc.push(tileOf(k), e), e.clientX, e.clientY)
  }
  // passive: false — иначе preventDefault у колеса не сработает.
  node.addEventListener('wheel', wheel, { passive: false })
  const unpinch = pinchOn(node, (f, x, y) => order(acc.pushScale(tileOf(k), f), x, y))
  return {
    update(next: DecorKind) {
      if (next === k) return
      k = next
      // Кадр от прежнего вида взял бы пустой накопитель и сжал размер нового вида.
      if (raf) cancelAnimationFrame(raf)
      raf = 0
      acc = mk()
    },
    destroy() {
      node.removeEventListener('wheel', wheel)
      unpinch()
      if (raf) cancelAnimationFrame(raf)
      raf = 0
      acc.reset()
    },
  }
}
