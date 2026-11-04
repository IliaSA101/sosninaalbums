// Повёрнутое место (спека 010, design-rotation §2.1): геометрия предпросмотра жестов. Зеркало Go
// (internal/geom/rotbox.go; паритет testdata/parity/rotplace.json): считает Go, здесь — то,
// что видно, пока кнопка не отпущена. Место — неповёрнутый прямоугольник (x, y, w, h) и угол вокруг
// его центра; всё в мм разворота.
import { MIN_PLACE } from './grid'
import { snapPlaceMove } from './snap'
import type { Axis, Field, SnapCtx } from './gridedit'
import { aabb, normDeg, rotate, type TextBox } from './textbox'
import type { Handle, Rect, RotRect } from '../core/types'

// math.Round из Go: от нуля (Math.round в JS у отрицательных .5 расходится).
export const roundAway = (v: number) => Math.sign(v) * Math.round(Math.abs(v))
export const round2 = (v: number) => roundAway(v * 100) / 100
export const clampF = (v: number, lo: number, hi: number) => Math.max(lo, Math.min(hi, v))
const floor2 = (v: number) => Math.floor(v * 100 + 1e-6) / 100
export const ceil01 = (v: number) => Math.ceil(v * 100 - 1e-4) / 100
export const floor01 = (v: number) => Math.floor(v * 100 + 1e-4) / 100
const EPS = 0.01

export const angleOf = (r: RotRect): number => r.rot ?? 0

// Место как повёрнутая рамка: угол до поворота = центр + rotate(rot, −w/2, −h/2).
export function placeBox(r: RotRect): TextBox {
  const o = rotate(angleOf(r), -r.w / 2, -r.h / 2)
  return { x: r.x + r.w / 2 + o.x, y: r.y + r.h / 2 + o.y, w: r.w, h: r.h, rot: angleOf(r) }
}

// aabb — описанный прямоугольник: привязка, линейки, мини-панель, DpiMark, выделение рамкой.
export const aabbOf = (r: RotRect): Rect => (angleOf(r) === 0 ? { x: r.x, y: r.y, w: r.w, h: r.h } : aabb(placeBox(r)))

// hold — удержание: без поворота само место, иначе квадрат min(w, h) вокруг центра (вписанный круг).
export function holdOf(r: RotRect): Rect {
  if (angleOf(r) === 0) return { x: r.x, y: r.y, w: r.w, h: r.h }
  const d = Math.min(r.w, r.h)
  return { x: r.x + (r.w - d) / 2, y: r.y + (r.h - d) / 2, w: d, h: d }
}

// v (на сетке 0.01) в [lo, hi], границы округлены внутрь; между ними нет сотых — lo вверх. Ноль без знака.
function clampIn01(v: number, lo: number, hi: number): number {
  let r = ceil01(lo)
  const out = floor01(hi)
  if (r <= out) r = clampF(v, r, out)
  return r === 0 ? 0 : r
}

// clampPlace в Go: размеры, угол (normDeg), без поворота — внутрь разворота, с поворотом — hold внутри.
export function clampPlaceIn(pl: RotRect, sw: number, ph: number): RotRect {
  const w = clampF(round2(pl.w), MIN_PLACE, floor2(sw))
  const h = clampF(round2(pl.h), MIN_PLACE, floor2(ph))
  const rot = normDeg(angleOf(pl))
  if (rot === 0) {
    return { x: clampF(round2(pl.x), 0, floor2(sw - w)), y: clampF(round2(pl.y), 0, floor2(ph - h)), w, h, rot }
  }
  const r = Math.min(w, h) / 2
  return {
    x: clampIn01(round2(pl.x), r - w / 2, sw - r - w / 2),
    y: clampIn01(round2(pl.y), r - h / 2, ph - r - h / 2),
    w,
    h,
    rot,
  }
}

// clampMoveIn в Go: перенос повёрнутого места (SetPlaces) — вылет aabb ни с одной стороны не растёт
// относительно prev, сверх того hold. Размер выросший сильнее вылета — только hold. Без поворота — clampPlaceIn.
export function clampMoveIn(pl: RotRect, prev: RotRect, sw: number, ph: number): RotRect {
  const w = clampF(round2(pl.w), MIN_PLACE, floor2(sw))
  const h = clampF(round2(pl.h), MIN_PLACE, floor2(ph))
  const rot = normDeg(angleOf(pl))
  if (rot === 0) return clampPlaceIn({ ...pl, w, h, rot }, sw, ph)
  const was = aabbOf(prev)
  const half = aabbOf({ x: pl.x, y: pl.y, w, h, rot }) // от положения зависит только сдвиг, размер — нет
  const r = Math.min(w, h) / 2
  const axis = (v: number, size: number, a: number, o0: number, o1: number, side: number): number => {
    let lo = Math.max(a - o0, r) - size / 2
    let hi = Math.min(side + o1 - a, side - r) - size / 2
    if (ceil01(lo) > floor01(hi)) {
      lo = r - size / 2
      hi = side - r - size / 2
    }
    return clampIn01(round2(v), lo, hi)
  }
  return {
    x: axis(pl.x, w, half.w / 2, Math.max(0, -was.x), Math.max(0, was.x + was.w - sw), sw),
    y: axis(pl.y, h, half.h / 2, Math.max(0, -was.y), Math.max(0, was.y + was.h - ph), ph),
    w,
    h,
    rot,
  }
}

export interface RotMove {
  box: RotRect
  guideX: number | null
  guideY: number | null
}

// Перенос повёрнутого места на (dx, dy): описанный прямоугольник прилипает (snap = null — без привязки),
// axis — удержанная ось (по другой прилипания нет); итог — правило переноса Go (clampMoveIn).
export function moveRotated(base: RotRect, dx: number, dy: number, snap: SnapCtx | null, axis: Axis, sw: number, ph: number): RotMove {
  const a = aabbOf(base)
  let mx = dx
  let my = dy
  let gx: number | null = null
  let gy: number | null = null
  if (snap) {
    const raw = { ...a, x: a.x + dx, y: a.y + dy }
    const s = snapPlaceMove(raw, snap.others, snap.params, snap.thr)
    if (axis !== 'y') {
      mx += s.x - raw.x
      gx = s.guideX
    }
    if (axis !== 'x') {
      my += s.y - raw.y
      gy = s.guideY
    }
  }
  const box = clampMoveIn({ ...base, x: base.x + mx, y: base.y + my }, base, sw, ph)
  const got = aabbOf(box)
  // Зажим мог увести место от линии: направляющую тогда не рисуем.
  return {
    box,
    guideX: gx !== null && Math.abs(got.x - a.x - mx) < EPS ? gx : null,
    guideY: gy !== null && Math.abs(got.y - a.y - my) < EPS ? gy : null,
  }
}

const has = (h: Handle, c: string) => h.includes(c)

// Размер повёрнутого места ручкой в осях места: противоположная сторона (угол) стоит, центр едет на
// rotate(rot, d/2). lock — пропорции (у боковой ручки второй размер меняется вокруг центра). Без привязки.
export function resizeRotated(base: RotRect, handle: Handle, dx: number, dy: number, lock: boolean, sw: number, ph: number): RotRect {
  const rot = angleOf(base)
  const d = rotate(-rot, dx, dy)
  const kx = has(handle, 'e') ? 1 : has(handle, 'w') ? -1 : 0 // в какую сторону растёт ширина
  const ky = has(handle, 's') ? 1 : has(handle, 'n') ? -1 : 0
  let w = base.w + kx * d.x
  let h = base.h + ky * d.y
  if (lock && base.w > 0 && base.h > 0) {
    const sx = w / base.w
    const sy = h / base.h
    let s = kx && ky ? (Math.abs(sx - 1) >= Math.abs(sy - 1) ? sx : sy) : kx ? sx : sy
    const lower = Math.max(MIN_PLACE / base.w, MIN_PLACE / base.h)
    s = Math.max(lower, Math.min(s, Math.min(sw / base.w, ph / base.h)))
    w = base.w * s
    h = base.h * s
  }
  w = clampF(w, MIN_PLACE, sw)
  h = clampF(h, MIN_PLACE, ph)
  const o = rotate(rot, (kx * (w - base.w)) / 2, (ky * (h - base.h)) / 2)
  const cx = base.x + base.w / 2 + o.x
  const cy = base.y + base.h / 2 + o.y
  return clampMoveIn({ x: cx - w / 2, y: cy - h / 2, w, h, rot }, base, sw, ph)
}

// Поле X/Y/Ш/В у повёрнутого: как fieldBox (левый верхний угол неповёрнутого держится, с замком второй размер
// масштабируется), но зажим — Go-овский для переноса, а не «внутрь разворота».
export function fieldRotated(base: RotRect, field: Field, v: number, lock: boolean, sw: number, ph: number): RotRect {
  const b: RotRect = { ...base }
  if (field === 'x' || field === 'y') b[field] = v
  else {
    const size = Math.max(v, MIN_PLACE)
    const k = size / base[field]
    b[field] = size
    if (lock && base[field] > 0) {
      const other = field === 'w' ? 'h' : 'w'
      b[other] = Math.max(base[other] * k, MIN_PLACE)
    }
  }
  return clampMoveIn(b, base, sw, ph)
}

// Курсор ручки на повёрнутой рамке: направление ручки (n = 0°, по часовой) + угол места → ближайший из четырёх.
const DIR: Record<Handle, number> = { n: 0, ne: 45, e: 90, se: 135, s: 180, sw: 225, w: 270, nw: 315 }
const CURSORS = ['ns-resize', 'nesw-resize', 'ew-resize', 'nwse-resize']
export function resizeCursor(h: Handle, rot: number): string {
  const deg = (((DIR[h] + rot) % 180) + 180) % 180
  return CURSORS[Math.round(deg / 45) % 4]
}

// Метка (DpiMark), которая стоит у верхнего левого угла описанного прямоугольника и не крутится с местом:
// положение в осях ячейки (r — её рамка, rot — угол, ab — описанный прямоугольник), отступ off px — на экране.
export function uprightAt(r: Rect, rot: number, ab: Rect, off: number): { left: string; top: string; rot: number } {
  const u = rotate(-rot, ab.x - (r.x + r.w / 2), ab.y - (r.y + r.h / 2))
  const o = rotate(-rot, off, off)
  const pc = (v: number, of: number) => (of > 0 ? (v / of) * 100 : 0)
  return {
    left: `calc(${50 + pc(u.x, r.w)}% + ${o.x}px)`,
    top: `calc(${50 + pc(u.y, r.h)}% + ${o.y}px)`,
    rot: -rot,
  }
}

// Вынос ⟳ над верхней стороной на dist px не помещается в разворот — тогда его прячут внутрь рамки.
export function rotHandleIn(r: RotRect, dist: number, sw: number, ph: number): boolean {
  const o = rotate(angleOf(r), 0, -(r.h / 2) - dist)
  const x = r.x + r.w / 2 + o.x
  const y = r.y + r.h / 2 + o.y
  return x < 0 || x > sw || y < 0 || y > ph
}

// Центр ⟳ в мм разворота: на off мм от верхней стороны наружу (внутрь — со знаком минус), по её середине.
export function rotHandleAt(r: RotRect, off: number): { x: number; y: number } {
  const o = rotate(angleOf(r), 0, -(r.h / 2) - off)
  return { x: r.x + r.w / 2 + o.x, y: r.y + r.h / 2 + o.y }
}
