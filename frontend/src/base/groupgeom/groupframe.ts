// Рамка группы с углом (спека 018, design §2, §3.1, §3.4): описанный прямоугольник в осях φ, угол рамки,
// боковые ручки, поля карточки → аргументы Go. Угла группы в проекте нет: φ — состояние интерфейса.
// Чистая логика без Svelte; расчёт мест после жеста — grouprot.ts и groupaxes.ts (зеркала Go).
import { groupAxes, type GxAxesIn } from './groupaxes'
import { anchorOf, cornerK, commonAngle, groupBox, type GroupCorner, type GxText } from './groupxf'
import { placeBox } from '../geom/rotbox'
import { corners, normDeg, rotate, type Pt } from '../geom/textbox'
import type { RotRect } from '../core/types'

// Рамка с обязательным углом (RotRect.rot необязателен).
export type Frame = RotRect & { rot: number }

const FIELD_EPS = 0.05 // половина шага показа: «ввёл то, что видел»

// Описанный прямоугольник мест и текстов в осях φ: неповёрнутый прямоугольник вокруг центра, как место.
// φ = 0 — groupBox дословно (спека 014); пусто — null.
export function groupOBB(places: RotRect[], texts: GxText[], phi: number): Frame | null {
  if (places.length + texts.length === 0) return null
  if (phi === 0) {
    const b = groupBox(places, texts)
    return b ? { ...b, rot: 0 } : null
  }
  const pts: Pt[] = [...places.flatMap((r) => corners(placeBox(r))), ...texts.flatMap((t) => corners(t))]
  const loc = pts.map((p) => rotate(-phi, p.x, p.y))
  const u0 = Math.min(...loc.map((p) => p.x))
  const u1 = Math.max(...loc.map((p) => p.x))
  const v0 = Math.min(...loc.map((p) => p.y))
  const v1 = Math.max(...loc.map((p) => p.y))
  const c = rotate(phi, (u0 + u1) / 2, (v0 + v1) / 2)
  const w = u1 - u0
  const h = v1 - v0
  return { x: c.x - w / 2, y: c.y - h / 2, w, h, rot: phi }
}

// Угол в (−45, 45], на котором рисуется рамка: рамка под 90° и под 0° — один и тот же прямоугольник,
// поэтому ⟳ всегда сверху.
export function reduce90(phi: number): number {
  const r = ((((phi + 45) % 90) + 90) % 90) - 45
  return r <= -45 ? 45 : r
}

// Последний поворот группы: ключ выделения, угол рамки и углы членов сразу после него.
export interface Turn {
  key: string
  deg: number
  rots: Map<string, number>
}

// Угол рамки φ: поворот действует, пока выделение то же и углы членов те же, что сразу после него; иначе
// общий угол членов, иначе 0°. rots — текущие углы членов (ID → °).
export function frameAngle(turn: Turn | null, key: string, rots: Map<string, number>): number {
  if (turn && turn.key === key && rots.size > 0 && [...rots].every(([id, r]) => Math.abs((turn.rots.get(id) ?? NaN) - r) < 1e-6))
    return turn.deg
  return commonAngle([...rots.values()]) ?? 0
}

// У членов разные углы (подсказка поля «Угол»). Поворот группы это не отменяет: после выпрямления 0°/15° так и
// остаются разными, а φ показывает рамку.
export function mixedAngles(rots: Map<string, number>): boolean {
  return rots.size > 0 && commonAngle([...rots.values()]) === null
}

export type GroupSide = 'n' | 'e' | 's' | 'w'

const ctr = (f: Frame): Pt => ({ x: f.x + f.w / 2, y: f.y + f.h / 2 })
// Точка рамки по локальным координатам от её центра (u — вдоль ширины, v — вдоль высоты).
const at = (f: Frame, u: number, v: number): Pt => {
  const c = ctr(f)
  const d = rotate(f.rot ?? 0, u, v)
  return { x: c.x + d.x, y: c.y + d.y }
}

// Неподвижная точка боковой ручки — середина противоположной стороны.
export function sideAnchor(f: Frame, side: GroupSide): Pt {
  return side === 'e' ? at(f, -f.w / 2, 0) : side === 'w' ? at(f, f.w / 2, 0) : side === 's' ? at(f, 0, -f.h / 2) : at(f, 0, f.h / 2)
}

const MIN_K = 0.001

// Множители по осям рамки от сдвига ручки (dx, dy) в осях разворота: тянется только своя ось.
export function sideK(f: Frame, side: GroupSide, dx: number, dy: number): { kx: number; ky: number } {
  const d = rotate(-f.rot, dx, dy)
  if (side === 'e' || side === 'w') return { kx: f.w > 0 ? Math.max(MIN_K, (f.w + (side === 'e' ? d.x : -d.x)) / f.w) : 1, ky: 1 }
  return { kx: 1, ky: f.h > 0 ? Math.max(MIN_K, (f.h + (side === 's' ? d.y : -d.y)) / f.h) : 1 }
}

// Рамка в локальных координатах (центр в нуле): к ней применяются anchorOf и cornerK из 014.
const local = (f: Frame) => ({ x: -f.w / 2, y: -f.h / 2, w: f.w, h: f.h })

// Неподвижный угол при масштабе за угол: противоположный, у повёрнутой рамки — от её повёрнутого угла.
export function cornerAnchor(f: Frame, corner: GroupCorner): Pt {
  const a = anchorOf(local(f), corner)
  return at(f, a.x, a.y)
}

// Пропорциональный k по сдвигу угла (dx, dy) в осях разворота.
export function cornerKRot(f: Frame, corner: GroupCorner, dx: number, dy: number): number {
  const d = rotate(-f.rot, dx, dy)
  return cornerK(local(f), corner, d.x, d.y)
}

export type FrameField = 'x' | 'y' | 'w' | 'h'

export interface MoveArgs {
  k: 1
  ax: 0
  ay: 0
  dx: number
  dy: number
}

// X / Y: левый верхний угол неповёрнутой рамки; правка — перенос (TransformGroup, k = 1).
export function moveField(f: Frame, field: 'x' | 'y', v: number): MoveArgs | null {
  if (!Number.isFinite(v) || Math.abs(v - f[field]) < FIELD_EPS) return null
  return { k: 1, ax: 0, ay: 0, dx: field === 'x' ? v - f.x : 0, dy: field === 'y' ? v - f.y : 0 }
}

export interface AxesArgs {
  kx: number
  ky: number
  phi: number
  ax: number
  ay: number
}

export type AxesBase = Omit<GxAxesIn, 'kx' | 'ky' | 'phi' | 'ax' | 'ay'>

const aligned = (rot: number, phi: number, mod: number) => {
  const d = Math.abs((((rot - phi) % mod) + mod) % mod)
  return d < 1e-6 || mod - d < 1e-6
}

// Ш / В: растяжка по одной оси от левой (верхней) стороны. Все члены соосны — ровно v / текущее; иначе подбор
// множителя по зеркалу groupAxes (ширина рамки растёт с ним): удвоение до перехода через v, затем бисекция.
// Недостижимо — упор (Go зажмёт так же). Мусор, v ≤ 0 и разница меньше половины шага — null.
export function sizeField(f: Frame, field: 'w' | 'h', v: number, base: AxesBase): AxesArgs | null {
  const cur = f[field]
  if (!Number.isFinite(v) || v <= 0 || cur <= 0 || Math.abs(v - cur) < FIELD_EPS) return null
  const phi = f.rot
  const a = field === 'w' ? at(f, -f.w / 2, 0) : at(f, 0, -f.h / 2)
  const mk = (k: number): AxesArgs => ({ kx: field === 'w' ? k : 1, ky: field === 'h' ? k : 1, phi, ax: a.x, ay: a.y })
  // Тексты вдоль оси растут ровно только по ширине; по высоте их кегль и высота по ky не идут.
  const exact =
    base.places.every((r) => aligned(r.rot ?? 0, phi, 90)) && base.texts.every((t) => (field === 'w' ? aligned(t.rot, phi, 180) : false))
  const k0 = v / cur
  if (exact) return mk(k0)
  const size = (k: number): number => {
    const o = groupAxes({ ...base, ...mk(k) })
    const bb = groupOBB(o.places, o.texts, phi)
    return bb ? bb[field] : 0
  }
  let lo = k0
  let hi = k0
  if (size(k0) < v) {
    hi = k0 * 2
    for (let i = 0; i < 12 && size(hi) < v; i++) {
      lo = hi
      hi *= 2
    }
  } else {
    lo = k0 / 2
    for (let i = 0; i < 12 && size(lo) > v; i++) {
      hi = lo
      lo /= 2
    }
  }
  for (let i = 0; i < 30; i++) {
    const m = (lo + hi) / 2
    if (size(m) < v) lo = m
    else hi = m
  }
  return mk((lo + hi) / 2)
}

export interface RotArgs {
  deg: number
  cx: number
  cy: number
}

// «Угол»: значение v поворачивает группу на v − φ вокруг центра рамки. Мусор и разница меньше половины шага — null.
export function angleField(f: Frame, phi: number, v: number): RotArgs | null {
  if (!Number.isFinite(v) || Math.abs(v - phi) < FIELD_EPS) return null
  const deg = normDeg(Math.round(v * 10) / 10 - phi)
  if (deg === 0) return null
  const c = ctr(f)
  return { deg, cx: c.x, cy: c.y }
}
