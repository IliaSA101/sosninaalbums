// Подсветка места (спека 033, design §5): какой из подходящих элементов мигает и где подпись.
// Чистая геометрия в px окна, без DOM.
import { articleText } from './articletext'

export interface Box {
  left: number
  top: number
  width: number
  height: number
}
export interface Win {
  w: number
  h: number
}

// Подсветка живёт ≈ 4 с (5 пульсов по 800 мс); с подписью-подсказкой ≈ 6 с: её надо успеть прочесть
// после смены уровня и переезда панели (аудит QA 2026-10-14).
export const SPOT_MS = 4000
export const SPOT_HINT_MS = 6000
// Кольцо тура (113) держится, пока открыт шаг: первые 1,5 с замер каждый кадр, дальше раз в 250 мс.
export const HOLD_FAST_MS = 1500
export const HOLD_SLOW_MS = 250
export const PAD = 4 // кольцо шире элемента
export const GAP = 8 // подпись от элемента и от края окна

// Сведения об элементе, снятые из DOM: решение «годится ли» принимается здесь, под тестом.
export interface Probe {
  rects: number // getClientRects().length: 0 — display:none или не в дереве
  w: number
  h: number
  inert: boolean // сам или предок под [inert]
  hidden: boolean // visibility: hidden
}

export const isCandidate = (p: Probe): boolean => p.rects > 0 && p.w > 0 && p.h > 0 && !p.inert && !p.hidden

// Площадь части прямоугольника, что видна в окне.
export function visibleArea(r: Box, win: Win): number {
  const w = Math.min(r.left + r.width, win.w) - Math.max(r.left, 0)
  const h = Math.min(r.top + r.height, win.h) - Math.max(r.top, 0)
  return w > 0 && h > 0 ? w * h : 0
}

// Из нескольких кандидатов (один якорь в нескольких местах: карточка у холста и в панели деталей) выбираем
// видимый с наибольшей площадью в окне. Все вне окна — берём самый большой (needScroll). Прокручиваются
// предки в любом случае (revealInView): элемент в окне может быть обрезан краем своей панели.
export function pickTarget(rects: Box[], win: Win): { index: number; needScroll: boolean } | null {
  let best = -1
  let bestArea = 0
  rects.forEach((r, i) => {
    const a = visibleArea(r, win)
    if (a > bestArea) {
      best = i
      bestArea = a
    }
  })
  if (best >= 0) return { index: best, needScroll: false }
  let big = -1
  let bigArea = 0
  rects.forEach((r, i) => {
    if (r.width * r.height > bigArea) {
      big = i
      bigArea = r.width * r.height
    }
  })
  return big >= 0 ? { index: big, needScroll: true } : null
}

export const RING_EDGE = 2 // кольцо от края окна: вырез и обводка не уходят за экран

// Кольцо вокруг элемента; с win — не шире окна с отступом RING_EDGE (элемент вплотную к краю: лента фото у низа окна).
export function ringBox(r: Box, win?: Win): Box {
  const l = r.left - PAD
  const t = r.top - PAD
  const rt = r.left + r.width + PAD
  const b = r.top + r.height + PAD
  if (!win) return { left: l, top: t, width: rt - l, height: b - t }
  const left = Math.max(l, RING_EDGE)
  const top = Math.max(t, RING_EDGE)
  return {
    left,
    top,
    width: Math.max(0, Math.min(rt, win.w - RING_EDGE) - left),
    height: Math.max(0, Math.min(b, win.h - RING_EDGE) - top),
  }
}

const clamp = (v: number, lo: number, hi: number): number => Math.max(lo, Math.min(v, Math.max(lo, hi)))

// Подпись: снизу от кольца, иначе сверху, справа, слева; без места нигде — снизу. Не выходит за окно.
export function placeLabel(ring: Box, size: { w: number; h: number }, win: Win): { left: number; top: number } {
  const midX = ring.left + ring.width / 2 - size.w / 2
  const midY = ring.top + ring.height / 2 - size.h / 2
  const below = ring.top + ring.height + GAP
  const above = ring.top - size.h - GAP
  const right = ring.left + ring.width + GAP
  const left = ring.left - size.w - GAP
  let x: number
  let y: number
  if (below + size.h <= win.h - GAP) [x, y] = [midX, below]
  else if (above >= GAP) [x, y] = [midX, above]
  else if (right + size.w <= win.w - GAP) [x, y] = [right, midY]
  else if (left >= GAP) [x, y] = [left, midY]
  else [x, y] = [midX, below]
  return { left: clamp(x, GAP, win.w - size.w - GAP), top: clamp(y, GAP, win.h - size.h - GAP) }
}

// На сколько сдвинуть прокручиваемого предка [vlo, vhi], чтобы элемент [lo, hi] оказался в поле зрения
// с отступом pad; элемент крупнее поля зрения выравнивается по началу. Координаты — окна.
export function revealShift(lo: number, hi: number, vlo: number, vhi: number, pad = GAP): number {
  if (hi - lo > vhi - vlo - 2 * pad) return lo - vlo - pad
  if (lo < vlo + pad) return lo - vlo - pad
  if (hi > vhi - pad) return hi - vhi + pad
  return 0
}

// Запасной тост, когда мигать нечем: «Название: где». Всегда непустой, даже у статьи без where и how.
export function missToast(a: { title: string; where?: string; how?: string }, hint = ''): string {
  return articleText(`«${a.title}»: ${hint || a.where || a.how || 'сейчас этого места на экране нет'}`)
}

// Тост про смену уровня и запасной — одним сообщением: второй notify затёр бы первый.
export const joinToast = (level: string | undefined, miss: string): string => (level ? `${level}. ${miss}` : miss)
