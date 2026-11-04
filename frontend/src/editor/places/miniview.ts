// Замер окна мини-панели (спека 009, 108): видимая область холста в осях разворота и угол листа на экране.
// Вынесено из MiniPanel.svelte; DOM читается здесь, решения — в minipanel.ts и minicol.ts.
import { COL_W, chromeInset, visibleArea, type MiniView } from './minipanel'

// Угол листа на экране и px экрана на px листа: панель стоит position: fixed от него.
export type Org = { x: number; y: number; k: number }

export const sameOrg = (a?: Org, b?: Org): boolean => a === b || (!!a && !!b && a.x === b.x && a.y === b.y && a.k === b.k)

// Низ ленты разворотов в осях листа, если она стоит сразу под окном холста (±2 px); иначе ленты нет.
function spillOf(win: DOMRect, k: number, top: number): number | undefined {
  const tl = document.querySelector('[data-guide="timeline"]')?.getBoundingClientRect()
  if (!tl || !(tl.height > 0) || Math.abs(tl.top - (win.top + win.height)) > 2) return undefined
  return (tl.top + tl.height - top) / k
}

// Видимая область (без линеек, ленты «Добавить» и плавающей карточки) и угол листа на экране; лента разворотов под
// холстом — в v.spill. sheet — лист (родитель якоря), win — окно холста, width — ширина листа, px раскладки.
export function measureMini(sheet: Element | null | undefined, win: Element | null | undefined, width: number): { v?: MiniView; o?: Org } {
  const sr = sheet?.getBoundingClientRect()
  const wr = win?.getBoundingClientRect()
  if (!sr || !wr) return {}
  const parts = [...(win?.parentElement?.querySelectorAll<HTMLElement>('[data-mini-edge]') ?? [])].map((n) => ({
    edge: n.dataset.miniEdge as 'top' | 'left' | 'right',
    box: n.getBoundingClientRect(),
  }))
  const v = visibleArea(sr, wr, width, chromeInset(wr, parts))
  const k = width > 0 ? sr.width / width : 0
  if (!v || !(k > 0)) return { v }
  const sp = spillOf(wr, k, sr.top)
  return { v: sp === undefined ? v : { ...v, spill: sp }, o: { x: sr.left, y: sr.top, k } }
}

// Панель целиком вне окна холста (и ленты под ним): столбцу проверяется ещё и x. h — её высота, px.
export function panelOutside(place: { top: number; left: number }, h: number, col: boolean, seen: MiniView): boolean {
  return (
    place.top < seen.top - 0.5 ||
    place.top + h > Math.max(seen.bottom, seen.spill ?? seen.bottom) + 0.5 ||
    (col && (place.left < seen.left - 0.5 || place.left + COL_W > seen.right + 0.5))
  )
}
