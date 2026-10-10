// Жесты блока текста на развороте: перенос, ширина, поворот, кегль. Предпросмотр идёт без вызова Go,
// на отпускании — одна правка. Состояние общее для двух слоёв разворота: блоки лежат в стеке `.content`
// (TextLayer), а рамка выделения с ручками, редактор и подсказки — над ним (TextOverlay).
import { edit } from './edit.svelte'
import { placesel } from '../places/placesel.svelte'
import { canvas } from '../canvas/canvas.svelte'
import { axisLock } from '../../base/geom/gridedit'
import { snapLines, snapThreshold } from '../../base/geom/snap'
import { MAX_SIZE, MIN_SIZE, resizeBox } from './textedit'
import {
  boxOf,
  center,
  cursorDeg,
  dragAngle,
  moveRotated,
  resizeLocal,
  rotateAbout,
  scaleAnchor,
  scaleCorner,
  type Corner,
} from '../../base/geom/textbox'
import type { Settings, SpreadView, TextView } from '../../base/core/types'

export interface Gesture {
  id: string
  mode: 'move' | 'resize' | 'rotate' | 'scale'
  x: number
  y: number
  w: number
  h: number
  scale: number // множитель кегля: рамка и строки растут от (x, y); 1 — не угловой жест
  size: number // кегль после углового жеста, пт
  rot: number
  guideX: number | null
  guideY: number | null
  label: { text: string; x: number; y: number } | null // подпись угла или кегля у курсора, px разворота
}

export interface TextCtx {
  spread: SpreadView
  settings: Settings
  width: number // px разворота
  root: HTMLElement | undefined // `.spread`: от его рамки считаются экранные координаты
}

const NO_LINES = { v: [], h: [] }

export class TextGestures {
  gesture = $state<Gesture | null>(null)

  constructor(private ctx: () => TextCtx) {}

  get root() {
    return this.ctx().root
  }
  rect = () => this.ctx().root?.getBoundingClientRect()

  editing = (kind: 'text' | 'caption', id: string) => edit.target?.kind === kind && edit.target.id === id
  selected = (kind: 'text' | 'caption', id: string) => {
    const s = edit.sel
    return s?.kind === kind && s.id === id && s.spread === this.ctx().spread.id
  }
  // Текст — член группы (без ручек: рамку группы рисует GroupFrame).
  inGroup = (id: string) => placesel.texts.includes(id)

  // Положение блока с учётом жеста.
  eff(t: TextView) {
    // Жест группы (перенос, масштаб): рамка и множитель кегля приходят из GroupFrame через placesel.
    const gl = placesel.liveTexts?.get(t.id)
    if (gl) return { x: gl.x, y: gl.y, w: gl.w, h: gl.h, scale: gl.scale, rot: gl.rot ?? t.rot, mode: 'move' as const }
    const g = this.gesture?.id === t.id ? this.gesture : null
    return g
      ? { x: g.x, y: g.y, w: g.w, h: g.h, scale: g.scale, rot: g.rot, mode: g.mode }
      : { x: t.x, y: t.y, w: t.w, h: t.h, scale: 1, rot: t.rot, mode: null }
  }

  // Жест «ничего не изменилось»: поля, которые режим не трогает, остаются как у блока.
  private base = (t: TextView): Gesture => ({
    id: t.id,
    mode: 'move',
    x: t.x,
    y: t.y,
    w: t.w,
    h: t.h,
    scale: 1,
    size: t.style.size,
    rot: t.rot,
    guideX: null,
    guideY: null,
    label: null,
  })

  // Слушатели жеста на window. run вызывается на отпускании (если жест начался), cancel — на Esc,
  // click — на отпускании без движения (порог жеста). canvas.busy на время жеста: колесо не меняет
  // масштаб, пока блок в руках.
  private track(move: (ev: PointerEvent) => boolean, run: () => Promise<unknown>, click?: () => void) {
    canvas.busy = true
    let started = false
    const off = () => {
      window.removeEventListener('pointermove', onMove)
      window.removeEventListener('pointerup', up)
      window.removeEventListener('keydown', esc, true)
    }
    const onMove = (ev: PointerEvent) => {
      if (move(ev)) started = true
    }
    const up = async () => {
      off()
      try {
        // Предпросмотр держим до ответа Go: иначе блок на миг отскакивает назад.
        if (started) await run()
        else click?.()
      } finally {
        this.gesture = null
        canvas.busy = false
      }
    }
    const esc = (ev: KeyboardEvent) => {
      if (ev.key !== 'Escape') return
      ev.stopPropagation() // Esc отменяет только жест, выделение остаётся
      off()
      this.gesture = null
      canvas.busy = false
    }
    window.addEventListener('pointermove', onMove)
    window.addEventListener('pointerup', up)
    window.addEventListener('keydown', esc, true)
  }

  // Перенос блока и боковые ручки ширины.
  start(e: PointerEvent, t: TextView, mode: 'move' | 'l' | 'r', click?: () => void) {
    const { spread, settings, width } = this.ctx()
    const sw = settings.pageW * 2
    const ph = settings.pageH
    const k = width / sw
    const sx = e.clientX
    const sy = e.clientY
    const box = boxOf(t)
    const all = snapLines({ pageW: settings.pageW, pageH: ph, margin: settings.margin, rects: spread.rects })
    const thr = snapThreshold(width, settings.pageW)
    const base = this.base(t)
    let last: Gesture | null = null
    this.track(
      (ev) => {
        if (!last && Math.hypot(ev.clientX - sx, ev.clientY - sy) < 5) return false
        const { dx, dy, axis } = axisLock((ev.clientX - sx) / k, (ev.clientY - sy) / k, mode === 'move' && ev.shiftKey)
        // Общая «Привязка» выключена или зажат Alt — линий нет, прилипать не к чему.
        const ln = ev.altKey || !placesel.snap ? NO_LINES : all
        if (mode === 'move') {
          const r = moveRotated(box, dx, dy, ln, thr, sw, ph)
          // Shift держит ось: по другой прилипание не уводит.
          last = {
            ...base,
            mode: 'move',
            x: axis === 'y' ? t.x : r.x,
            y: axis === 'x' ? t.y : r.y,
            guideX: axis === 'y' ? null : r.guideX,
            guideY: axis === 'x' ? null : r.guideY,
          }
        } else if (t.rot === 0) {
          const r = resizeBox(box, mode, dx, ln, thr, sw)
          last = { ...base, mode: 'resize', x: r.x, w: r.w, guideX: r.guide }
        } else {
          // Повёрнутый: проекция на ось строки, без прилипания (design §6.4).
          const r = resizeLocal(box, mode, dx, dy, settings.pageW)
          last = { ...base, mode: 'resize', x: r.x, y: r.y, w: r.w }
        }
        this.gesture = last
        return true
      },
      async () => {
        if (last) await edit.setBox(last.id, last.x, last.y, last.w)
      },
      click,
    )
  }

  // Ручка поворота: центр блока на экране — от рамки разворота; блок крутится вокруг своего центра.
  rotate(e: PointerEvent, t: TextView) {
    const rect0 = this.rect()
    if (!rect0) return
    const { settings } = this.ctx()
    const sw = settings.pageW * 2
    const ph = settings.pageH
    const box = boxOf(t)
    const cm = center(box)
    const c = { x: rect0.left + (cm.x / sw) * rect0.width, y: rect0.top + (cm.y / ph) * rect0.height }
    const grab = cursorDeg(c, { x: e.clientX, y: e.clientY })
    const sx = e.clientX
    const sy = e.clientY
    let last: Gesture | null = null
    this.track(
      (ev) => {
        if (!last && Math.hypot(ev.clientX - sx, ev.clientY - sy) < 5) return false
        const r = this.rect() ?? rect0
        const deg = dragAngle(c, { x: ev.clientX, y: ev.clientY }, grab, t.rot, ev.shiftKey)
        const p = rotateAbout(box, deg)
        last = {
          ...this.base(t),
          mode: 'rotate',
          x: p.x,
          y: p.y,
          rot: deg,
          label: { text: `${deg}°`, x: ev.clientX - r.left, y: ev.clientY - r.top },
        }
        this.gesture = last
        return true
      },
      async () => {
        if (last) await edit.setAngle(last.id, last.rot)
      },
    )
  }

  // Угловая ручка: кегль и рамка растут вместе от (x, y) — так держит Go; подсказка «36 пт» у курсора,
  // на отпускании один SetTextScale.
  scale(e: PointerEvent, t: TextView, corner: Corner) {
    const { settings, width } = this.ctx()
    if (!this.ctx().root) return
    const k = width / (settings.pageW * 2)
    const box = boxOf(t)
    const size0 = t.style.size
    const sx = e.clientX
    const sy = e.clientY
    let last: Gesture | null = null
    this.track(
      (ev) => {
        if (!last && Math.hypot(ev.clientX - sx, ev.clientY - sy) < 5) return false
        const r = this.rect()
        const s = scaleCorner(box, corner, (ev.clientX - sx) / k, (ev.clientY - sy) / k, MIN_SIZE / size0, MAX_SIZE / size0)
        // Кегль округляется до 0.1, как в Go: рамка в предпросмотре совпадает с итогом.
        const size = Math.round(size0 * s.k * 10) / 10
        const m = size / size0
        last = {
          ...this.base(t),
          mode: 'scale',
          ...scaleAnchor(box, corner, m), // по округлённому кеглю: предпросмотр совпадает с итогом Go
          w: t.w * m,
          h: t.h * m,
          scale: m,
          size,
          label: { text: `${size} пт`, x: ev.clientX - (r?.left ?? 0), y: ev.clientY - (r?.top ?? 0) },
        }
        this.gesture = last
        return true
      },
      async () => {
        if (last && last.size !== size0) await edit.setScale(last.id, last.size, last.x, last.y)
      },
    )
  }
}
