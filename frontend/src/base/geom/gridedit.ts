// Правка мест без режима (спека 007/008): чистые шаги предпросмотра (перенос, размер, поля X/Y/Ш/В)
// и выделение по ref. Всё в мм разворота; итог зажимает и пишет Go (SetPlaces), здесь — только то,
// что видно до отпускания.
//
// ref — ID места или ID фото этого разворота (у «Авто» мест нет). Выделение хранит ref: фото — ID фото
// (и в «Авто», и в сетке — после закрепления не пересчитывается), пустое место — ID места.
import { MIN_PLACE, moveBoxes, resizeBox, round2, type Bounds } from './grid'
import { clampBox, snapPlaceMove, snapPlaceResize, type PlaceSnapParams } from './snap'
import type { Handle, Rect, SpreadView } from '../core/types'

// Контекст прилипания: остальные места разворота и параметры страницы.
export interface SnapCtx {
  others: Rect[]
  params: PlaceSnapParams
  thr: number // порог, мм
}

export interface MoveResult {
  boxes: Rect[]
  guideX: number | null
  guideY: number | null
}

const EPS = 0.01

export type Axis = 'x' | 'y' | null

// Shift во время переноса держит ось: остаётся больший сдвиг, меньший обнуляется. Ничья — по X.
// axis — вдоль какой оси идёт сдвиг (null — свободно): по другой оси прилипание не должно уводить.
export function axisLock(dx: number, dy: number, shift: boolean): { dx: number; dy: number; axis: Axis } {
  if (!shift) return { dx, dy, axis: null }
  return Math.abs(dx) >= Math.abs(dy) ? { dx, dy: 0, axis: 'x' } : { dx: 0, dy, axis: 'y' }
}

// Перенос выделенных мест на (dx, dy). Прилипает образец (первый), остальные едут с ним.
// axis — удержанная ось (axisLock): по другой оси прилипания нет.
export function moveSel(bases: Rect[], dx: number, dy: number, bounds: Bounds, snap: SnapCtx | null, axis: Axis = null): MoveResult {
  const raw = moveBoxes(bases, dx, dy, bounds)
  if (!snap || raw.length === 0) return { boxes: raw, guideX: null, guideY: null }
  const s = snapPlaceMove(raw[0], snap.others, snap.params, snap.thr)
  const sx = axis === 'y' ? raw[0].x : s.x
  const sy = axis === 'x' ? raw[0].y : s.y
  const boxes = moveBoxes(bases, dx + (sx - raw[0].x), dy + (sy - raw[0].y), bounds)
  // Зажим у края мог увести место от линии: направляющую тогда не рисуем.
  return {
    boxes,
    guideX: axis !== 'y' && s.guideX !== null && Math.abs(boxes[0].x - s.x) < EPS ? s.guideX : null,
    guideY: axis !== 'x' && s.guideY !== null && Math.abs(boxes[0].y - s.y) < EPS ? s.guideY : null,
  }
}

export interface ResizeResult {
  box: Rect
  guideX: number | null
  guideY: number | null
}

// Размер одного места ручкой. Замок (или Shift) — пропорции, и тогда без прилипания: оно их сломало бы.
export function resizeSel(
  base: Rect,
  handle: Handle,
  dx: number,
  dy: number,
  lock: boolean,
  bounds: Bounds,
  snap: SnapCtx | null,
): ResizeResult {
  const box = resizeBox(base, handle, dx, dy, lock, MIN_PLACE, bounds)
  if (lock || !snap) return { box, guideX: null, guideY: null }
  const s = snapPlaceResize(box, handle, snap.others, snap.params, snap.thr, MIN_PLACE)
  return { box: s.box, guideX: s.guideX, guideY: s.guideY }
}

export type Field = 'x' | 'y' | 'w' | 'h'

// Поле X/Y/Ш/В задало значение v (мм). С замком второй размер масштабируется, левый верхний угол держится.
export function fieldBox(base: Rect, field: Field, v: number, lock: boolean, bounds: Bounds): Rect {
  const b = { ...base }
  if (field === 'x' || field === 'y') {
    b[field] = v
  } else {
    const size = Math.max(v, MIN_PLACE)
    const k = size / base[field]
    b[field] = size
    if (lock && base[field] > 0) {
      const other = field === 'w' ? 'h' : 'w'
      b[other] = Math.max(base[other] * k, MIN_PLACE)
    }
  }
  return roundRect(clampBox(b, bounds.w, bounds.h))
}

const roundRect = (b: Rect): Rect => ({ x: round2(b.x), y: round2(b.y), w: round2(b.w), h: round2(b.h) })

// Отличаются ли рамки больше, чем на округление Go.
export const sameBox = (a: Rect, b: Rect) =>
  Math.abs(a.x - b.x) < EPS && Math.abs(a.y - b.y) < EPS && Math.abs(a.w - b.w) < EPS && Math.abs(a.h - b.h) < EPS

// Выделение по ref: клик — один; Shift — добавить или убрать. Первый — образец.
export function pickPlace(sel: string[], ref: string, additive: boolean): string[] {
  if (!additive) return [ref]
  return sel.includes(ref) ? sel.filter((s) => s !== ref) : [...sel, ref]
}

// Выделенное в мм разворота: рамка + что отдавать Go. id — ID места (сетка) или ID фото (у «Авто»):
// Go принимает оба (до G14 у «Авто» правка по ref холостая).
export interface SelBox extends Rect {
  ref: string
  id: string
  photo: string // "" — пустое место
  fit: '' | 'contain'
  rot: number // угол вокруг центра (спека 010); в «Авто» 0
}

// Рамка ref: сетка — место по ID, затем по фото; «Авто» — ячейка фото. Чужой ref — null.
export function refBox(s: SpreadView | undefined, ref: string): SelBox | null {
  if (!s || !ref) return null
  if (s.grid) {
    const p = s.places?.find((x) => x.id === ref) ?? s.places?.find((x) => x.photo === ref)
    return p ? { x: p.x, y: p.y, w: p.w, h: p.h, ref, id: p.id, photo: p.photo, fit: p.fit, rot: p.rot ?? 0 } : null
  }
  const i = s.photos.indexOf(ref)
  const r = i >= 0 ? s.rects[i] : undefined
  return r ? { x: r.x, y: r.y, w: r.w, h: r.h, ref, id: ref, photo: ref, fit: '', rot: 0 } : null
}

// Рамки выделенных ref в порядке выделения; исчезнувшие и повторные (фото и его место) отбрасываются.
export function refsBoxes(s: SpreadView | undefined, refs: string[]): SelBox[] {
  const out: SelBox[] = []
  for (const ref of refs) {
    const b = refBox(s, ref)
    if (b && !out.some((o) => o.id === b.id)) out.push(b)
  }
  return out
}

// Образец «Одинакового размера» Go берёт первым в списке: последнее щёлкнутое выходит вперёд,
// иначе порядок выделения (рамкой — порядок мест).
export function sampleFirst(ids: string[], sample: string | null | undefined): string[] {
  if (!sample || ids[0] === sample || !ids.includes(sample)) return ids
  return [sample, ...ids.filter((x) => x !== sample)]
}

// Пропорции места совпадают с фото (± 0.5 %): «Под фото» не нужна. Размер фото неизвестен — тоже
// нечего предлагать.
export function canFitPhoto(box: Rect, aspect: number): boolean {
  if (!(aspect > 0) || !(box.w > 0) || !(box.h > 0)) return true
  return Math.abs(box.w / box.h / aspect - 1) <= 0.005
}

// Значение для поля: мм с одним знаком.
export const fmtMM = (n: number) => String(Math.round(n * 10) / 10)

// Сессия стрелок: [A-Za-z0-9-], ≤ 32 знаков (требование Go).
export const gridSession = (t = Date.now()) => `grid-${t.toString(36)}`
