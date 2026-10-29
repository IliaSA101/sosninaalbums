// Масштаб группы по осям рамки (спека 018, design §3.3): p → a + M(p − a), M = R(φ)·diag(kx, ky)·R(−φ).
// Зеркало Go (internal/geom/groupaxes.go, паритет testdata/parity/groupaxes.json): считает Go, это предпросмотр
// жеста. Перекоса нет: место сохраняет угол, его стороны — длины образов своих осей (как rescaleGrid в Go).
import { MIN_SIZE, MAX_SIZE } from '../../editor/text/textedit'
import { MIN_PLACE } from '../geom/grid'
import { clampPlaceIn, roundAway, round2 } from '../geom/rotbox'
import { groupBound, shiftIn } from './grouprot'
import { rotate } from '../geom/textbox'
import type { GxText } from './groupxf'
import type { RotRect } from '../core/types'

export interface GxAxesIn {
  sw: number
  ph: number
  places: RotRect[]
  texts: GxText[]
  kx: number
  ky: number
  phi: number // угол рамки, °
  ax: number // неподвижная точка
  ay: number
}

// Итог: действующие множители после зажима, сдвиг внутрь, места (шаг 0.01) и тексты (h — оценка по кеглю).
export interface GxAxesOut {
  kx: number
  ky: number
  dx: number
  dy: number
  places: RotRect[]
  texts: GxText[]
}

// Члены после масштаба (kx, ky) по осям рамки, без сдвига и округления.
function axesAt(inp: GxAxesIn, kx: number, ky: number): { places: RotRect[]; texts: GxText[] } {
  const tr = (x: number, y: number) => {
    const u = rotate(-inp.phi, x - inp.ax, y - inp.ay)
    const d = rotate(inp.phi, u.x * kx, u.y * ky)
    return { x: inp.ax + d.x, y: inp.ay + d.y }
  }
  // Множители своих осей объекта под углом rot: длины образов (1, 0) и (0, 1).
  const sides = (rot: number): [number, number] => {
    const r = ((rot - inp.phi) * Math.PI) / 180
    const s = Math.sin(r)
    const c = Math.cos(r)
    return [Math.hypot(kx * c, ky * s), Math.hypot(kx * s, ky * c)]
  }
  const places = inp.places.map((r) => {
    const c = tr(r.x + r.w / 2, r.y + r.h / 2)
    const [sx, sy] = sides(r.rot ?? 0)
    const w = r.w * sx
    const h = r.h * sy
    return { x: c.x - w / 2, y: c.y - h / 2, w, h, rot: r.rot ?? 0 }
  })
  const texts = inp.texts.map((t) => {
    const al = t.al ?? 0
    const [sx] = sides(t.rot) // кегль — по min(kx, ky), не по осям текста
    const u = rotate(t.rot, al * t.w, 0)
    const a = tr(t.x + u.x, t.y + u.y)
    const size = Math.min(Math.max(roundAway(t.size * Math.min(kx, ky) * 10) / 10, MIN_SIZE), MAX_SIZE)
    const w = t.w * sx
    const v = rotate(t.rot, al * w, 0)
    return { x: a.x - v.x, y: a.y - v.y, w, h: (t.h * size) / t.size, rot: t.rot, size, al }
  })
  return { places, texts }
}

// Множители не меньше, чем места остаются ≥ MIN_PLACE и кегль ≥ нижнего предела (консервативно: образ оси ≥
// min(kx, ky)). Не помещается в разворот — бисекция по отрезку (1, 1) → (kx, ky). Текст: едет точка
// выравнивания, кегль — по min(kx, ky) группы (не по осям самого текста).
export function groupAxes(inp: GxAxesIn): GxAxesOut {
  const out: GxAxesOut = { kx: 1, ky: 1, dx: 0, dy: 0, places: [], texts: [] }
  if (inp.places.length + inp.texts.length === 0) return out
  let kx = inp.kx
  let ky = inp.ky
  let hi = Infinity
  let lo = 0
  for (const t of inp.texts) {
    hi = Math.min(hi, MAX_SIZE / t.size)
    lo = Math.max(lo, MIN_SIZE / t.size)
  }
  // Меньший множитель зажат сверху по MaxSize/Size (иначе кегль упрётся в 200 и отстанет от группы);
  // больший кегль не трогает, при равных — оба.
  const small = Math.min(kx, ky)
  if (small > hi) {
    if (kx === small) kx = hi
    if (ky === small) ky = hi
  }
  for (const r of inp.places) lo = Math.max(lo, MIN_PLACE / Math.min(r.w, r.h))
  kx = Math.max(kx, lo)
  ky = Math.max(ky, lo)
  const fits = (x: number, y: number) => {
    const m = axesAt(inp, x, y)
    const b = groupBound(m.places, m.texts)
    return !b || (b.x1 - b.x0 <= inp.sw + 1e-9 && b.y1 - b.y0 <= inp.ph + 1e-9)
  }
  if (!fits(kx, ky)) {
    let t0 = 0
    let t1 = 1
    for (let i = 0; i < 30; i++) {
      const m = (t0 + t1) / 2
      if (fits(1 + m * (kx - 1), 1 + m * (ky - 1))) t0 = m
      else t1 = m
    }
    kx = 1 + t0 * (kx - 1)
    ky = 1 + t0 * (ky - 1)
  }
  const m = axesAt(inp, kx, ky)
  const b = groupBound(m.places, m.texts)
  const dx = b ? shiftIn(b.x0, b.x1, inp.sw) : 0
  const dy = b ? shiftIn(b.y0, b.y1, inp.ph) : 0
  return {
    kx,
    ky,
    dx,
    dy,
    places: m.places.map((r) => {
      const w = round2(r.w)
      const h = round2(r.h)
      const cx = r.x + r.w / 2 + dx
      const cy = r.y + r.h / 2 + dy
      return clampPlaceIn({ x: round2(cx - w / 2), y: round2(cy - h / 2), w, h, rot: r.rot }, inp.sw, inp.ph)
    }),
    texts: m.texts.map((t) => ({ ...t, x: round2(t.x + dx), y: round2(t.y + dy), w: round2(t.w) })),
  }
}
