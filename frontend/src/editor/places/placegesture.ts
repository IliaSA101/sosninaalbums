// Жесты правки мест на развороте без режима (спека 008, modeless.md): перенос и размер.
// Общие для SelFrame (рамка выделенного) и PlaceLayer (тело пустого места). Предпросмотр считает
// gridedit.ts и показывает рамка выделения (placesel.live); на отпускании — один SetPlaces по ref
// (у «Авто» первая такая правка закрепляет разворот в Go).
import { primaryPress } from '../../base/core/platform'
import { Api, store } from '../../base/core/store.svelte'
import { gridAct } from '../../panels/layouts/gridact.svelte'
import { placesel } from './placesel.svelte'
import { canvas } from '../canvas/canvas.svelte'
import { additive } from './gesture'
import { axisLock, moveSel, resizeSel, type SnapCtx } from '../../base/geom/gridedit'
import { roundBox } from '../../base/geom/grid'
import { snapThreshold } from '../../base/geom/snap'
import { coverSides } from '../cover/cover'
import { aabbOf, moveRotated, resizeRotated } from '../../base/geom/rotbox'
import { bdAngle } from '../../base/geom/bdbox'
import { cursorDeg } from '../../base/geom/textbox'
import type { Handle, Rect, RotRect, SpreadView } from '../../base/core/types'

const THRESHOLD = 3

// Клик сразу после жеста (pointerup над другим элементом) не должен снимать выделение.
let justDone = false
let doneTimer = 0
export const gestureClickAllowed = () => !justDone

// Обычный щелчок по пустому полю (страница или серое поле вокруг) снимает выделение; с Shift/Ctrl —
// ничего (gestures.md); после жеста над рамкой не считается. Общий для Spread и Stage.
export function fieldClick(e: MouseEvent) {
  if (!additive(e) && gestureClickAllowed()) store.selectedCell = null
}

let running = false

// ms > 0 — дольше обычного: после выбора пункта меню второй щелчок быстрого двойного попал бы
// в поле под закрывшимся меню и снял выделение.
export function markDone(ms = 0) {
  justDone = true
  clearTimeout(doneTimer)
  doneTimer = window.setTimeout(() => (justDone = false), ms)
}

// Рамки остальных мест (кроме переносимых) — к ним прилипаем; у повёрнутого — описанный прямоугольник.
function othersOf(s: SpreadView, skip: Set<string>): Rect[] {
  if (s.grid) return (s.places ?? []).filter((p) => !skip.has(p.id)).map(aabbOf)
  return s.photos.flatMap((id, i) => (skip.has(id) || !s.rects[i] ? [] : [{ ...s.rects[i] }]))
}

// Каркас жеста: порог 3 px, Esc отменяет (и не снимает выделение), на отпускании — commit.
// root — элемент разворота: по его ширине px переводятся в мм. step считает предпросмотр,
// show его рисует (null — убрать), commit пишет итог одной правкой.
export function drive<T>(
  e: PointerEvent,
  root: HTMLElement,
  step: (dx: number, dy: number, ev: PointerEvent) => T,
  show: (r: T | null) => void,
  commit: (r: T) => Promise<unknown>,
  onClick?: () => void,
) {
  if (!primaryPress(e) || running || canvas.space || canvas.panning) return
  const sw = store.spread?.w ?? 0
  if (!(sw > 0)) return
  const mmPx = sw / root.getBoundingClientRect().width
  const sx = e.clientX
  const sy = e.clientY
  let moved = false
  let last: T | null = null
  running = true
  canvas.busy = true

  const cleanup = () => {
    window.removeEventListener('pointermove', move)
    window.removeEventListener('pointerup', up)
    window.removeEventListener('keydown', esc, true)
  }
  const finish = () => {
    running = false
    canvas.busy = false
    show(null)
    placesel.guides = { x: null, y: null }
  }
  const move = (ev: PointerEvent) => {
    if (!moved && Math.hypot(ev.clientX - sx, ev.clientY - sy) < THRESHOLD) return
    moved = true
    last = step((ev.clientX - sx) * mmPx, (ev.clientY - sy) * mmPx, ev)
    show(last)
  }
  const up = async () => {
    cleanup()
    markDone()
    if (!moved || !last) {
      finish()
      onClick?.()
      return
    }
    placesel.guides = { x: null, y: null }
    try {
      await commit(last)
    } finally {
      finish()
    }
  }
  const esc = (ev: KeyboardEvent) => {
    if (ev.key !== 'Escape') return
    ev.stopPropagation()
    cleanup()
    finish()
    // Кнопку ещё держат: её отпускание даст click по общему предку — на выделение он влиять не должен.
    window.addEventListener('pointerup', () => markDone(), { once: true })
  }
  window.addEventListener('pointermove', move)
  window.addEventListener('pointerup', up)
  window.addEventListener('keydown', esc, true)
}

type Step = (dx: number, dy: number, ev: PointerEvent) => Map<string, RotRect>

// Жест над местами: итог — один SetPlaces по ref (у «Авто» первая такая правка закрепляет разворот в Go).
function gesture(e: PointerEvent, root: HTMLElement, sid: string, step: Step, onClick?: () => void) {
  drive(
    e,
    root,
    step,
    (r) => (placesel.live = r ? { spread: sid, boxes: r } : null),
    (last) => {
      const boxes = [...last].map(([id, b]) => ({ id, ...roundBox(b) }))
      return gridAct.placeEdit(sid, Api.SetPlaces(sid, boxes, ''))
    },
    onClick,
  )
}

// Контекст прилипания; Alt или выключенная «Привязка» — без него.
export function snapCtx(ev: PointerEvent, root: HTMLElement, s: SpreadView, skip: Set<string>): SnapCtx | null {
  if (ev.altKey || !placesel.snap) return null
  const st = store.view.settings
  return {
    others: othersOf(s, skip),
    params: { w: s.w, h: s.h, mid: s.mid, margin: st.margin, gap: st.gap, rects: [], sides: coverSides(s, store.view.cover) },
    thr: snapThreshold(root.getBoundingClientRect().width, s.w),
  }
}

// Перенос выделенных. solo — ref, к которому свернуть группу, если по нему щёлкнули без движения.
export function startMove(e: PointerEvent, root: HTMLElement, solo?: string) {
  const s = store.spread
  const items = placesel.selected
  if (!s || items.length === 0) return
  const bounds = { w: s.w, h: s.h }
  const bases: Rect[] = items.map((i) => ({ x: i.x, y: i.y, w: i.w, h: i.h }))
  const skip = new Set(items.map((i) => i.id))
  const turned = items.length === 1 && items[0].rot !== 0 ? { ...bases[0], rot: items[0].rot } : null
  gesture(
    e,
    root,
    s.id,
    (dx, dy, ev) => {
      const a = axisLock(dx, dy, ev.shiftKey) // Shift во время протяжки держит ось
      if (turned) {
        // Повёрнутое: прилипает описанный прямоугольник, зажим — правило переноса Go (вылет не растёт).
        const m = moveRotated(turned, a.dx, a.dy, snapCtx(ev, root, s, skip), a.axis, bounds.w, bounds.h)
        placesel.guides = { x: m.guideX, y: m.guideY }
        return new Map([[items[0].id, m.box]])
      }
      const r = moveSel(bases, a.dx, a.dy, bounds, snapCtx(ev, root, s, skip), a.axis)
      placesel.guides = { x: r.guideX, y: r.guideY }
      return new Map(r.boxes.map((b, i) => [items[i].id, b]))
    },
    () => {
      if (solo && items.length > 1) placesel.select([solo])
    },
  )
}

// Размер образца (первого выделенного) ручкой. Замок или Shift — с пропорциями.
export function startResize(e: PointerEvent, root: HTMLElement, handle: Handle) {
  const s = store.spread
  const sample = placesel.selected[0]
  if (!s || !sample) return
  const bounds = { w: s.w, h: s.h }
  const base: Rect = { x: sample.x, y: sample.y, w: sample.w, h: sample.h }
  const skip = new Set([sample.id])
  gesture(e, root, s.id, (dx, dy, ev) => {
    if (sample.rot !== 0) {
      // Повёрнутое: размер в осях места, без привязки (линии не по его осям).
      placesel.guides = { x: null, y: null }
      return new Map([
        [sample.id, resizeRotated({ ...base, rot: sample.rot }, handle, dx, dy, placesel.lock || ev.shiftKey, bounds.w, bounds.h)],
      ])
    }
    const r = resizeSel(base, handle, dx, dy, placesel.lock || ev.shiftKey, bounds, snapCtx(ev, root, s, skip))
    placesel.guides = { x: r.guideX, y: r.guideY }
    return new Map([[sample.id, r.box]])
  })
}

// ⟳: угол места следит за курсором вокруг центра (Shift — 15°, Alt — без прилипания); на отпускании —
// один SetPlaceAngle. Место остаётся на месте, крутится только рамка (фото встаёт после ответа Go).
export function startRotate(e: PointerEvent, root: HTMLElement) {
  const s = store.spread
  const sample = placesel.selected[0]
  if (!s || !sample) return
  const r = root.getBoundingClientRect()
  const k = s.w / r.width
  const c = { x: sample.x + sample.w / 2, y: sample.y + sample.h / 2 }
  const p0 = { x: (e.clientX - r.left) * k, y: (e.clientY - r.top) * k }
  const grab = cursorDeg(c, p0)
  // Результат — объект: угол 0° ложен, а drive по ложному считает, что сдвига не было.
  drive(
    e,
    root,
    (dx, dy, ev) => ({ deg: bdAngle(c, { x: p0.x + dx, y: p0.y + dy }, grab, sample.rot, ev.shiftKey, ev.altKey) }),
    (a) =>
      (placesel.live = a
        ? { spread: s.id, boxes: new Map([[sample.id, { x: sample.x, y: sample.y, w: sample.w, h: sample.h, rot: a.deg }]]) }
        : null),
    (a) => gridAct.placeEdit(s.id, Api.SetPlaceAngle(s.id, sample.id, a.deg)),
  )
}

// Двойной щелчок по ⟳ — угол 0°.
export function zeroAngle() {
  const s = store.spread
  const sample = placesel.selected[0]
  if (s && sample && sample.rot !== 0) gridAct.placeEdit(s.id, Api.SetPlaceAngle(s.id, sample.id, 0))
}
