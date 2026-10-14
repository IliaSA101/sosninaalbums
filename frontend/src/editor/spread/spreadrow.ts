// Ряд разворотов на холсте (спека 012): чистая логика без Svelte. Позиция ряда `pos` — в слотах
// (дробная), а не в px: ресайз окна, крупный просмотр и галерея раскладок её не сбивают.
// Слоты 0..n-1 — развороты, слот n — «новый» (точка остановки).
import { centerScroll } from '../../panels/timeline/timeline'
import type { WheelLike } from '../zoom/zoom'

export const GAP = 24 // зазор между листами, px
export const MAX_SLOTS = 13 // сколько слотов максимум в DOM (узкий Stage при открытой галерее)
export const IDLE = 140 // мс без событий колеса — начинается доводка
export const SNAP_MIN = 0.2 // сдвиг от стартового слота, после которого доводка идёт дальше
export const FLING = 0.4 // px/мс: бросок быстрее — к следующему слоту по направлению
export const FAR = 2 // дальше этого (в слотах) переход подъезжает одним шагом
const REACH = 3 // щелчки колеса копят цель не дальше стольких слотов от вида
const RUBBER = 0.35 // коэффициент резинки за краем
const RUBBER_MAX = 0.3 // и её предел, слотов
const MOUSE_STEP = 40 // целый шаг не меньше этого — щелчок колеса мыши, а не тачпад

const clamp = (v: number, lo: number, hi: number) => Math.min(hi, Math.max(lo, v))
const fin = (v: number) => (Number.isFinite(v) ? v : 0)

// Шаг ряда: ширина вписанного разворота (целая) плюс зазор.
export const pitch = (W: number) => W + GAP

// Масштаб ряда, px на мм: вписывает обычный разворот (base) в свободное место. Один на весь ряд: обложка (032)
// и развороты рисуются в одном масштабе, а зазор между краями соседних листов всегда GAP (см. slotCenters).
export function rowScale(availW: number, availH: number, base: { w: number; h: number }): number {
  if (!(base.w > 0) || !(base.h > 0)) return 0
  return Math.max(0, Math.min(availW / base.w, availH / base.h))
}

export interface SlotBox {
  left: number
  top: number
  width: number
  height: number
}

// Центры слотов по x (px) от центра слота 0. Листы разной ширины (обложка с заворотом шире разворота, слот «новый»)
// стоят так, что между соседними краями ровно GAP: шаг считается по полуширинам, а не по ширине обычного разворота.
export function slotCenters(widths: readonly number[], gap = GAP): number[] {
  const out: number[] = []
  let c = 0
  for (let i = 0; i < widths.length; i++) {
    if (i > 0) c += (widths[i - 1] + widths[i]) / 2 + gap
    out.push(c)
  }
  return out
}

// Центр в дробной позиции pos (между целыми — линейно). За краями ряд продолжается по крайнему шагу:
// резинка (012 §5.2) тянет pos в [-0,3, 0) и (max, max+0,3], и ряд должен там двигаться. Шаг при длине 1 — 0.
export function centerAt(centers: readonly number[], pos: number): number {
  const n = centers.length
  if (n === 0) return 0
  const p = fin(pos)
  if (p < 0) return centers[0] + (n > 1 ? p * (centers[1] - centers[0]) : 0)
  const last = n - 1
  if (p > last) return centers[last] + (n > 1 ? (p - last) * (centers[last] - centers[last - 1]) : 0)
  const i = Math.floor(p)
  const b = centers[Math.min(i + 1, last)]
  return centers[i] + (b - centers[i]) * (p - i)
}

// Позиция ряда после сдвига на px (px > 0 — вперёд): обратная к centerAt, поэтому лист идёт под пальцем 1:1
// и у обложки шире разворота. Меньше двух слотов — шаг step.
export function movePos(centers: readonly number[], pos: number, px: number, step: number): number {
  const n = centers.length
  if (n < 2 || !(centers[n - 1] > centers[0])) return fin(pos) + fin(px) / Math.max(1, step)
  const x = centerAt(centers, pos) + fin(px)
  const first = centers[1] - centers[0]
  if (x <= centers[0]) return first > 0 ? (x - centers[0]) / first : 0
  const last = n - 1
  const tail = centers[last] - centers[last - 1]
  if (x >= centers[last]) return last + (tail > 0 ? (x - centers[last]) / tail : 0)
  let i = 0
  while (i + 1 < last && centers[i + 1] <= x) i++
  const d = centers[i + 1] - centers[i]
  return i + (d > 0 ? (x - centers[i]) / d : 0)
}

// Слот i ряда: центр сдвинут от центра текущего (cur, его место — fit) на разницу центров слотов, размер — масштаб × мм листа.
export function slotBox(
  fit: SlotBox,
  i: number,
  cur: number,
  centers: readonly number[],
  k: number,
  size: { w: number; h: number },
): SlotBox {
  if (i === cur) return fit
  const width = k * size.w
  const height = k * size.h
  const cx = fit.left + fit.width / 2 + (centers[i] - centers[cur])
  const cy = fit.top + fit.height / 2
  return { left: Math.round(cx - width / 2), top: Math.round(cy - height / 2), width, height }
}

// Последний слот: n при n ≥ 1 (слот «новый»), иначе ряда нет.
export const rowMax = (n: number) => (n >= 1 ? n : 0)

// Ближайший слот к позиции (для подсветки ленты и «Разворот N»).
export const nearSlot = (pos: number, max: number) => clamp(Math.round(clamp(pos, 0, max)), 0, max)

// Левый край слота i в окне; left — левый край текущего листа в покое (pos = cur). При целом pos всё целое: разворот в покое не размыт.
export const slotLeft = (i: number, pos: number, left: number, centers: readonly number[]) => left + centers[i] - centerAt(centers, pos)

// Какие слоты рисовать: видимые плюс по одному с каждой стороны, не больше MAX_SLOTS;
// текущий (cur) входит всегда — у него один и тот же экземпляр Spread при движении ряда.
// В зуме — только текущий. Результат по возрастанию.
export function renderRange(pos: number, n: number, max: number, w: number, W: number, P: number, zoomed: boolean, cur: number): number[] {
  if (n < 1) return []
  if (zoomed) return [cur]
  const half = (w + W) / (2 * P)
  // видимые: pos − half < i < pos + half; плюс по одному запасному с каждой стороны
  let lo = Math.max(0, Math.floor(pos - half))
  let hi = Math.min(max, Math.ceil(pos + half))
  if (hi - lo + 1 > MAX_SLOTS) {
    const c = clamp(Math.round(pos), 0, max)
    lo = Math.max(0, Math.min(c - (MAX_SLOTS - 1) / 2, max - (MAX_SLOTS - 1)))
    hi = Math.min(max, lo + MAX_SLOTS - 1)
  }
  const out: number[] = []
  for (let i = lo; i <= hi; i++) out.push(i)
  if (cur >= 0 && cur <= max && !out.includes(cur)) {
    out.push(cur)
    out.sort((a, b) => a - b)
    while (out.length > MAX_SLOTS) {
      // выкидываем самый дальний от вида, но не текущий
      const far = out.filter((i) => i !== cur).reduce((a, b) => (Math.abs(b - pos) > Math.abs(a - pos) ? b : a))
      out.splice(out.indexOf(far), 1)
    }
  }
  return out
}

// Куда доводить ряд после жеста. start — слот в начале жеста, v — слотов/мс (знак: вперёд по индексам).
export function snapTarget(start: number, pos: number, v: number, max: number, P: number): number {
  let t: number
  if (Math.abs(v) * P >= FLING) {
    t = v > 0 ? Math.floor(pos) + 1 : Math.ceil(pos) - 1
  } else {
    const d = pos - start
    if (Math.abs(d) < SNAP_MIN - 1e-9) t = start
    else {
      // ушли на |d| слотов: на слот — следующий, дальше — округление со смещением 0,3
      const steps = Math.max(1, Math.floor(Math.abs(d) + 0.8 + 1e-9))
      t = start + Math.sign(d) * steps
    }
  }
  return clamp(t, 0, max)
}

// Резинка за краями [0, max]: смещение ×0,35, не больше 0,3 слота.
export function rubber(raw: number, max: number): number {
  if (raw < 0) return -Math.min(RUBBER_MAX, -raw * RUBBER)
  if (raw > max) return max + Math.min(RUBBER_MAX, (raw - max) * RUBBER)
  return raw
}

// Дальний переход: вид сразу ставится на слот до цели, дальше — обычный один шаг.
export function jumpFrom(pos: number, target: number): number {
  return Math.abs(target - pos) > FAR ? target - Math.sign(target - pos) : pos
}

export type RowWheel = { kind: 'step'; dir: -1 | 1 } | { kind: 'px'; px: number }

// Колесо при 100 % (Ctrl уже ушёл в зум): щелчок мыши (целый одиночный шаг ≥ 40 по любой оси) и строки —
// шаг на слот; тачпад — px по главной оси (px > 0 — вперёд). null — событие пустое.
// Shift+колесо мыши: Chromium шлёт deltaX = ±100, иначе такой щелчок сдвигал бы на 0,1 слота.
export function rowWheel(e: WheelLike): RowWheel | null {
  const dx = fin(e.deltaX)
  const dy = fin(e.deltaY)
  const main = Math.abs(dx) > Math.abs(dy) ? dx : dy
  if (main === 0) return null
  const dir = main > 0 ? 1 : -1
  if (e.deltaMode !== 0) return { kind: 'step', dir }
  const click = (a: number, b: number) => a === 0 && Number.isInteger(b) && Math.abs(b) >= MOUSE_STEP
  return click(dx, dy) || click(dy, dx) ? { kind: 'step', dir } : { kind: 'px', px: main }
}

// Цель шага от текущей цели: копится, но не дальше REACH слотов от вида и в [0, max].
export function stepTarget(target: number, d: number, pos: number, max: number): number {
  return clamp(clamp(target + d, pos - REACH, pos + REACH), 0, max)
}

// Прокрутка ленты альбома: центры групп (последний — «+»), между целыми — линейно по pos.
export function rowScroll(centers: number[], pos: number, boxW: number): number {
  if (centers.length === 0) return 0
  // лента за резинкой не уходит: позиция зажата в ряд
  return centerScroll(centerAt(centers, clamp(fin(pos), 0, centers.length - 1)), boxW, 0)
}

// Длительность анимации по расстоянию в слотах.
export const dur = (delta: number) => clamp(160 + 220 * Math.abs(fin(delta)), 160, 450)

// Ease-out cubic (как --move).
export const easeOut = (t: number) => 1 - Math.pow(1 - clamp(t, 0, 1), 3)
