// Поворот группы (спека 018, design §3.2): каждый член поворачивается вокруг общего центра, группа жёсткая.
// Зеркало Go (internal/geom/grouprot.go, паритет testdata/parity/grouprot.json): считает Go, это предпросмотр
// жеста, пока не отпущена кнопка. Угол не упирается: что вылезло, обрезается краем. Всё в мм разворота.
import { aabb, normDeg, rotate } from '../geom/textbox'
import { ceil01, clampF, clampPlaceIn, floor01, holdOf, round2 } from '../geom/rotbox'
import type { GxText } from './groupxf'
import type { RotRect } from '../core/types'

export interface GxRotIn {
  sw: number
  ph: number
  places: RotRect[]
  texts: GxText[]
  deg: number
  cx: number
  cy: number
}

// Итог: угол (normDeg входа), общий сдвиг внутрь, места и тексты (шаг 0.01, каждый уже зажат).
export interface GxRotOut {
  deg: number
  dx: number
  dy: number
  places: RotRect[]
  texts: GxText[]
}

// Описанный прямоугольник группы для зажима: hold мест и aabb текстов; пусто — null.
export function groupBound(places: RotRect[], texts: GxText[]): { x0: number; y0: number; x1: number; y1: number } | null {
  const rs = [...places.map(holdOf), ...texts.map((t) => aabb(t))]
  if (rs.length === 0) return null
  return {
    x0: Math.min(...rs.map((r) => r.x)),
    y0: Math.min(...rs.map((r) => r.y)),
    x1: Math.max(...rs.map((r) => r.x + r.w)),
    y1: Math.max(...rs.map((r) => r.y + r.h)),
  }
}

// Сдвиг по оси, при котором [v0, v1] внутри [0, side]: помещается — наименьший, нет — вылет поровну с двух сторон.
export function shiftIn(v0: number, v1: number, side: number): number {
  const lo = -v0
  const hi = side - v1
  return lo > hi ? (lo + hi) / 2 : clampF(0, lo, hi)
}

// FitAxis из Go: угол повёрнутого блока по одной оси внутри [lo, hi] на сетке 0.01; шире разворота — к левому краю.
function fitAxis(x: number, lo: number, hi: number): number {
  const eps = 1e-6
  if (!Number.isFinite(x) || !Number.isFinite(lo) || !Number.isFinite(hi)) return 0
  const inn = ceil01(lo)
  const out = floor01(hi)
  if (inn > out) return Math.abs(x - inn) > eps ? inn : x
  if (x < lo - eps) return inn
  if (x > hi + eps) return out
  return x
}

// TextBox.FitIn из Go: угол P, при котором блок целиком внутри разворота.
export function fitTextIn(t: GxText, sw: number, ph: number): { x: number; y: number } {
  if (t.rot !== 0) {
    const a = aabb(t)
    const ox = a.x - t.x
    const oy = a.y - t.y
    return { x: fitAxis(t.x, -ox, sw - a.w - ox), y: fitAxis(t.y, -oy, ph - a.h - oy) }
  }
  return { x: clampF(t.x, 0, sw - t.w), y: clampF(t.y, 0, Math.max(0, ph - t.h)) }
}

// Поворот группы на deg вокруг (cx, cy): у места центр идёт по дуге, у текста — угол рамки P. Потом группа
// целиком сдвигается внутрь по hold мест и aabb текстов; не помещается — по центру вылета, крайние поджимаются.
export function groupRot(inp: GxRotIn): GxRotOut {
  const th = normDeg(inp.deg)
  if (th === 0) return { deg: 0, dx: 0, dy: 0, places: [...inp.places], texts: [...inp.texts] }
  const turn = (x: number, y: number) => {
    const d = rotate(th, x - inp.cx, y - inp.cy)
    return { x: inp.cx + d.x, y: inp.cy + d.y }
  }
  const pls: RotRect[] = inp.places.map((r) => {
    const c = turn(r.x + r.w / 2, r.y + r.h / 2)
    return { x: c.x - r.w / 2, y: c.y - r.h / 2, w: r.w, h: r.h, rot: normDeg((r.rot ?? 0) + th) }
  })
  const txs: GxText[] = inp.texts.map((t) => ({ ...t, ...turn(t.x, t.y), rot: normDeg(t.rot + th) }))
  const b = groupBound(pls, txs)
  const dx = b ? shiftIn(b.x0, b.x1, inp.sw) : 0
  const dy = b ? shiftIn(b.y0, b.y1, inp.ph) : 0
  return {
    deg: th,
    dx,
    dy,
    places: pls.map((r) => clampPlaceIn({ ...r, x: r.x + dx, y: r.y + dy }, inp.sw, inp.ph)),
    texts: txs.map((t) => ({ ...t, ...fitTextIn({ ...t, x: round2(t.x + dx), y: round2(t.y + dy) }, inp.sw, inp.ph) })),
  }
}
