// Общий зум и сдвиги окна сравнения (138): чистые функции поверх loupe.ts (лупа окна 128), без Svelte.
// Зум относительный: z — во сколько раз кадр крупнее своего «вписано». У кадров одной камеры это то же,
// что общий масштаб, у разных пропорций — одна и та же доля кадра. Точка (cx, cy) — доли кадра.
import {
  FIT,
  fitScale,
  MAX_SCALE,
  ORIG_MAX_PX,
  ORIG_WHY_TEXT,
  origMax,
  pan,
  place,
  sameShape,
  toggle,
  zoomAt,
  type Loupe,
  type OrigWhy,
  type Pt,
  type Size,
} from '../../editor/photo/loupe'

export interface Sync {
  z: number // 0 — вписано
  cx: number
  cy: number
}
export interface Off {
  dx: number // свой сдвиг ячейки (Shift+перетаскивание), доли кадра
  dy: number
}
// Ячейка, с которой работает жест: размеры её превью и окна, предел зума, свой сдвиг.
export interface Tile {
  nat: Size
  box: Size
  max: number
  off: Off
}
export interface Photo {
  w: number
  h: number
  color?: string
}

export const FIT_SYNC: Sync = { z: 0, cx: 0.5, cy: 0.5 }
export const NO_OFF: Off = { dx: 0, dy: 0 }
// Свой сдвиг дальше половины кадра не уводит: иначе он «копился» бы там, где его уже не видно.
const OFF_LIMIT = 0.5
const EPS = 1e-6

const clamp = (v: number, lo: number, hi: number): number => Math.min(hi, Math.max(lo, v))

export const isFit = (s: Sync): boolean => s.z <= 0

// Лупа ячейки по общему состоянию: масштаб z·вписано в [вписано, max], центр — общая точка плюс свой сдвиг.
export function tileLoupe(s: Sync, o: Off, nat: Size, box: Size, max: number): Loupe {
  const fit = fitScale(nat, box)
  if (fit <= 0 || isFit(s)) return FIT
  const scale = clamp(s.z * fit, fit, Math.max(fit, max))
  if (scale <= fit + EPS) return FIT
  return { s: scale, cx: clamp(s.cx + o.dx, 0, 1), cy: clamp(s.cy + o.dy, 0, 1) }
}

const loupeOf = (s: Sync, a: Tile): Loupe => tileLoupe(s, a.off, a.nat, a.box, a.max)

// Лупа с центром, как его видно в окне (край кадра не отходит от края окна): жест начинается от настоящего места.
function visible(l: Loupe, nat: Size, box: Size): Loupe {
  if (l.s <= 0) return l
  const p = place(l, nat, box)
  return { s: l.s, cx: (box.w / 2 - p.left) / p.w, cy: (box.h / 2 - p.top) / p.h }
}

// Обратно в общее состояние: z от «вписано» активной ячейки, общая точка — без её своего сдвига.
function fromLoupe(l: Loupe, a: Tile): Sync {
  const fit = fitScale(a.nat, a.box)
  if (l.s <= 0 || fit <= 0) return FIT_SYNC
  return { z: l.s / fit, cx: l.cx - a.off.dx, cy: l.cy - a.off.dy }
}

// Зум в k раз; точка под курсором активной ячейки стоит на месте (колесо, щипок, Mod+= / Mod+−).
export function zoomSync(s: Sync, k: number, act: Tile, at: Pt | null): Sync {
  if (fitScale(act.nat, act.box) <= 0) return s
  return fromLoupe(zoomAt(visible(loupeOf(s, act), act.nat, act.box), k, act.nat, act.box, at, act.max), act)
}

// Вписано ↔ 1:1 активной ячейки в точке курсора (z = 1 / вписано активной).
export function toggleSync(s: Sync, act: Tile, at: Pt | null): Sync {
  if (fitScale(act.nat, act.box) <= 0) return s
  return fromLoupe(toggle(visible(loupeOf(s, act), act.nat, act.box), act.nat, act.box, at, MAX_SCALE), act)
}

// Перетаскивание увеличенного кадра: общая точка едет за курсором активной ячейки, у всех сразу. start — состояние на нажатии.
export function panSync(start: Sync, dx: number, dy: number, act: Tile): Sync {
  if (isFit(start)) return start
  const l = visible(loupeOf(start, act), act.nat, act.box)
  return fromLoupe(pan(l, dx, dy, act.nat, act.box), act)
}

// Shift+перетаскивание: сдвигается только эта ячейка. Возвращает новый свой сдвиг (act.off — на нажатии).
export function panOne(sync: Sync, dx: number, dy: number, act: Tile): Off {
  const l = loupeOf(sync, act)
  if (l.s <= 0) return act.off
  const moved = pan(visible(l, act.nat, act.box), dx, dy, act.nat, act.box)
  return { dx: clamp(moved.cx - sync.cx, -OFF_LIMIT, OFF_LIMIT), dy: clamp(moved.cy - sync.cy, -OFF_LIMIT, OFF_LIMIT) }
}

// Листание ячейки (← →): пропорции нового кадра те же (±1 %) — зум и точка остаются, иначе вписаны все.
export function carrySync(s: Sync, prev: Size, next: Size): Sync {
  if (isFit(s)) return s
  return sameShape(prev, next) ? s : FIT_SYNC
}

// Оригиналы выше 1:1 грузятся всем или никому: все кадры sRGB без профиля и вместе не больше 100 Мп (0.29-138-5).
// '' — можно всем; иначе причина для значка зума: цветовой профиль первого такого кадра или 'big'.
export function origBudget(photos: readonly (Photo | undefined)[]): OrigWhy {
  let sum = 0
  for (const p of photos) {
    if (!p) continue
    if (p.color === 'adobe' || p.color === 'p3' || p.color === 'icc' || p.color === 'other') return p.color
    sum += p.w * p.h
  }
  return sum > ORIG_MAX_PX ? 'big' : ''
}

export const BUDGET_TEXT = 'Крупнее превью не показывается: кадры вместе больше 100 Мп'

// Подсказка значка зума при отказе от оригиналов (Т9); пусто — оригиналы можно.
export const budgetText = (why: OrigWhy): string => (why === '' ? '' : why === 'big' ? BUDGET_TEXT : ORIG_WHY_TEXT[why])

// Предел зума ячейки: по оригиналу, если бюджет позволяет всем; иначе 1:1 по превью.
export const tileMax = (why: OrigWhy, p: Photo | undefined, nat: Size): number => (why === '' ? origMax(p, nat) : MAX_SCALE)
