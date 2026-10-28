// Прилипание блока текста при перетаскивании (спека 006, п. 11). Всё в мм разворота;
// порог задаётся в пикселях экрана и переводится в мм (snapThreshold).
import type { Handle, Rect } from '../core/types'

export interface SnapLines {
  v: number[] // вертикальные линии: x
  h: number[] // горизонтальные линии: y
}

export interface SnapParams {
  w: number // ширина листа, мм
  h: number // высота листа
  mid: number // шов, 0 — нет
  margin: number
  rects: Rect[] // ячейки фото разворота
  // Сторонки обложки (032) по X, мм: [левый край, правый]. Заданы — линии прилипания идут по ним (края, центры, поля),
  // а не по шву: у обложки mid — центр корешка, а страницы — задняя и лицевая.
  sides?: readonly (readonly [number, number])[]
}

export const SNAP_PX = 6

const uniq = (a: number[]) => [...new Set(a.map((n) => Math.round(n * 1000) / 1000))].sort((x, y) => x - y)

// Линии прилипания: края и центры страниц, поля, шов (= край левой и правой страницы),
// края и центры фото. Без шва (лист виньетки) страница одна — весь лист.
export function snapLines({ w, h: pageH, mid, margin, rects, sides }: SnapParams): SnapLines {
  let v: number[]
  if (sides?.length) v = [0, w, ...sides.flatMap(([a, b]) => [a, (a + b) / 2, b, a + margin, b - margin])]
  else v = mid > 0 ? [0, mid / 2, mid, (mid + w) / 2, w, margin, mid - margin, mid + margin, w - margin] : [0, w / 2, w, margin, w - margin]
  const h = [0, pageH / 2, pageH, margin, pageH - margin]
  for (const r of rects) {
    v.push(r.x, r.x + r.w / 2, r.x + r.w)
    h.push(r.y, r.y + r.h / 2, r.y + r.h)
  }
  return { v: uniq(v), h: uniq(h) }
}

// Порог в мм: px экрана × мм на пиксель. spreadPx — ширина листа на экране в пикселях, sheetW — в мм.
export function snapThreshold(spreadPx: number, sheetW: number, px = SNAP_PX): number {
  if (!(spreadPx > 0)) return 0
  return (px * sheetW) / spreadPx
}

export interface Snapped {
  d: number // поправка к исходной координате
  line: number | null // линия, к которой прилипли (для направляющей)
}

// Ближайшая линия к одной из точек (края и центр блока); побеждает меньший зазор.
export function snapAxis(points: number[], lines: number[], thr: number): Snapped {
  let best: Snapped = { d: 0, line: null }
  let bestAbs = Infinity
  for (const p of points) {
    for (const l of lines) {
      const d = l - p
      const a = Math.abs(d)
      if (a <= thr && a < bestAbs) {
        bestAbs = a
        best = { d, line: l }
      }
    }
  }
  return best
}

export interface BoxSnap {
  x: number
  y: number
  guideX: number | null // голубая вертикальная направляющая, мм
  guideY: number | null
}

// Прилипание рамки блока при переносе: левый край, центр и правый край — по x; верх, центр, низ — по y.
export function snapBox(box: Rect, lines: SnapLines, thr: number): BoxSnap {
  const sx = snapAxis([box.x, box.x + box.w / 2, box.x + box.w], lines.v, thr)
  const sy = snapAxis([box.y, box.y + box.h / 2, box.y + box.h], lines.h, thr)
  return { x: box.x + sx.d, y: box.y + sy.d, guideX: sx.line, guideY: sy.line }
}

// Прилипание боковой ручки ширины: edge — x двигаемого края, возвращается его новый x.
export function snapEdge(edge: number, lines: SnapLines, thr: number): { x: number; guide: number | null } {
  const s = snapAxis([edge], lines.v, thr)
  return { x: edge + s.d, guide: s.line }
}

// Блок не выходит за разворот (Go всё равно зажмёт, но предпросмотр не должен «вылетать»).
export function clampBox(box: Rect, spreadW: number, pageH: number): Rect {
  const w = Math.min(box.w, spreadW)
  const h = Math.min(box.h, pageH)
  return { x: Math.min(Math.max(box.x, 0), spreadW - w), y: Math.min(Math.max(box.y, 0), pageH - h), w, h }
}

// ---------- места сетки (спека 007) ----------
// Линии мест — те же snapLines (края и центры других мест передаются как rects); сверх них — «тот же
// промежуток»: расстояние до соседа, равное settings.gap.

export interface GapTargets {
  left: number[] // куда может встать левый край, чтобы до соседа слева было ровно gap
  right: number[] // правый край — до соседа справа
  top: number[]
  bottom: number[]
}

export interface PlaceSnapParams extends SnapParams {
  gap: number // settings.gap, мм
}

const overlap = (a0: number, a1: number, b0: number, b1: number) => a0 < b1 && b0 < a1

// Цели «тот же промежуток» для рамки box относительно соседей others: только у тех, кто с ней
// пересекается по другой оси (иначе промежуток между ними не имеет смысла).
export function gapTargets(box: Rect, others: Rect[], gap: number): GapTargets {
  const t: GapTargets = { left: [], right: [], top: [], bottom: [] }
  for (const o of others) {
    if (overlap(o.y, o.y + o.h, box.y, box.y + box.h)) {
      t.left.push(o.x + o.w + gap)
      t.right.push(o.x - gap)
    }
    if (overlap(o.x, o.x + o.w, box.x, box.x + box.w)) {
      t.top.push(o.y + o.h + gap)
      t.bottom.push(o.y - gap)
    }
  }
  return t
}

interface Cand {
  point: number // координата края или центра рамки
  lines: number[] // к чему он может прилипнуть
  gap: boolean
}

interface Hit {
  d: number
  line: number | null
  gap: boolean // прилипли к «тому же промежутку», а не к линии
}

// Лучший из кандидатов: меньший зазор; при равенстве побеждает обычная линия (она идёт раньше).
function bestOf(cands: Cand[], thr: number): Hit {
  let best: Hit = { d: 0, line: null, gap: false }
  let bestAbs = Infinity
  for (const c of cands) {
    for (const l of c.lines) {
      const d = l - c.point
      const a = Math.abs(d)
      if (a <= thr && a < bestAbs) {
        bestAbs = a
        best = { d, line: l, gap: c.gap }
      }
    }
  }
  return best
}

export interface PlaceSnap extends BoxSnap {
  gapX: boolean // по x прилипли к «тому же промежутку»
  gapY: boolean
}

// Перенос места: края и центр — к линиям (в т. ч. краям и центрам других мест), края — к «тому же
// промежутку». others — остальные места, без переносимых.
export function snapPlaceMove(box: Rect, others: Rect[], p: PlaceSnapParams, thr: number): PlaceSnap {
  const lines = snapLines({ ...p, rects: others })
  const gaps = gapTargets(box, others, p.gap)
  const x1 = box.x + box.w
  const y1 = box.y + box.h
  const sx = bestOf(
    [
      { point: box.x, lines: lines.v, gap: false },
      { point: box.x + box.w / 2, lines: lines.v, gap: false },
      { point: x1, lines: lines.v, gap: false },
      { point: box.x, lines: gaps.left, gap: true },
      { point: x1, lines: gaps.right, gap: true },
    ],
    thr,
  )
  const sy = bestOf(
    [
      { point: box.y, lines: lines.h, gap: false },
      { point: box.y + box.h / 2, lines: lines.h, gap: false },
      { point: y1, lines: lines.h, gap: false },
      { point: box.y, lines: gaps.top, gap: true },
      { point: y1, lines: gaps.bottom, gap: true },
    ],
    thr,
  )
  return {
    x: box.x + sx.d,
    y: box.y + sy.d,
    guideX: sx.line,
    guideY: sy.line,
    gapX: sx.gap,
    gapY: sy.gap,
  }
}

export interface PlaceResizeSnap {
  box: Rect
  guideX: number | null
  guideY: number | null
}

// Размер места: прилипают только двигаемые ручкой края (box — уже после resizeBox). Если после прилипания
// сторона стала бы меньше min, этот край остаётся как был. С замком пропорций не вызывать: прилипание
// одного края сломало бы пропорции.
export function snapPlaceResize(box: Rect, handle: Handle, others: Rect[], p: PlaceSnapParams, thr: number, min: number): PlaceResizeSnap {
  const lines = snapLines({ ...p, rects: others })
  const gaps = gapTargets(box, others, p.gap)
  let { x, y, w, h } = box
  let guideX: number | null = null
  let guideY: number | null = null

  if (handle.includes('w')) {
    const s = bestOf(
      [
        { point: x, lines: lines.v, gap: false },
        { point: x, lines: gaps.left, gap: true },
      ],
      thr,
    )
    if (s.line !== null && w - s.d >= min) {
      x += s.d
      w -= s.d
      guideX = s.line
    }
  } else if (handle.includes('e')) {
    const s = bestOf(
      [
        { point: x + w, lines: lines.v, gap: false },
        { point: x + w, lines: gaps.right, gap: true },
      ],
      thr,
    )
    if (s.line !== null && w + s.d >= min) {
      w += s.d
      guideX = s.line
    }
  }
  if (handle.includes('n')) {
    const s = bestOf(
      [
        { point: y, lines: lines.h, gap: false },
        { point: y, lines: gaps.top, gap: true },
      ],
      thr,
    )
    if (s.line !== null && h - s.d >= min) {
      y += s.d
      h -= s.d
      guideY = s.line
    }
  } else if (handle.includes('s')) {
    const s = bestOf(
      [
        { point: y + h, lines: lines.h, gap: false },
        { point: y + h, lines: gaps.bottom, gap: true },
      ],
      thr,
    )
    if (s.line !== null && h + s.d >= min) {
      h += s.d
      guideY = s.line
    }
  }
  return { box: { x, y, w, h }, guideX, guideY }
}
