// Жесты группы (спеки 009 U6, 018): перенос за полосу края и за любого члена, масштаб за угол, растяжка боковой
// ручкой по оси рамки, поворот ⟳. Предпросмотр считают зеркала Go (groupxf, groupaxes, grouprot) и кладут в
// placesel.live / liveTexts / liveTurn; на отпускании — один вызов Go. У «Авто» с фото Go сам закрепляет
// разворот той же правкой, тост даёт gridAct.placeEdit.
import { Api, store } from '../../base/core/store.svelte'
import { gridAct } from '../../panels/layouts/gridact.svelte'
import { placesel } from './placesel.svelte'
import { drive, snapCtx, startMove } from './placegesture'
import { axisLock } from '../../base/geom/gridedit'
import { snapPlaceMove } from '../../base/geom/snap'
import { groupBox, groupXf, type GroupCorner, type GxIn, type GxOut, type GxText } from '../../base/groupgeom/groupxf'
import { groupAxes, type GxAxesOut } from '../../base/groupgeom/groupaxes'
import { groupRot } from '../../base/groupgeom/grouprot'
import {
  angleField,
  cornerAnchor,
  cornerKRot,
  moveField,
  sideAnchor,
  sideK,
  sizeField,
  type FrameField,
  type GroupSide,
} from '../../base/groupgeom/groupframe'
import { bdAngle } from '../../base/geom/bdbox'
import { ALIGN_X, alignOf, cursorDeg, normDeg } from '../../base/geom/textbox'
import type { Rect, RotRect, SpreadView } from '../../base/core/types'

// ID членов группы на развороте для Go: места (ID места или фото) и выделенные тексты.
export function groupIds(s: SpreadView): string[] {
  return [...placesel.selected.map((i) => i.id), ...s.texts.filter((t) => placesel.texts.includes(t.id)).map((t) => t.id)]
}

type Base = Omit<GxIn, 'k' | 'ax' | 'ay' | 'dx' | 'dy'>

// Члены группы на развороте: места (с рамками и углами) и тексты (с долей выравнивания для groupAxes).
export function groupMembers(s: SpreadView): { in: Base; ids: string[]; textIds: string[] } {
  const items = placesel.selected
  const texts = s.texts.filter((t) => placesel.texts.includes(t.id))
  const st = store.view.settings
  return {
    in: {
      sw: st.pageW * 2,
      ph: st.pageH,
      places: items.map((i) => ({ x: i.x, y: i.y, w: i.w, h: i.h, rot: i.rot })),
      texts: texts.map((t) => ({ x: t.x, y: t.y, w: t.w, h: t.h, rot: t.rot, size: t.style.size, al: ALIGN_X[alignOf(t.style.align)] })),
    },
    ids: groupIds(s),
    textIds: texts.map((t) => t.id),
  }
}

// Итог шага жеста: новые места и тексты (предпросмотр), живой угол рамки (поворот) и вызов Go на отпускании.
interface Step {
  places: RotRect[]
  texts: GxText[]
  deg?: number
  call: (sid: string, ids: string[]) => Promise<unknown>
}

// Предпросмотр: рамки мест, тексты группы, угол рамки.
function show(s: SpreadView, textIds: string[], placeIds: string[], r: Step | null) {
  if (!r) {
    placesel.live = null
    placesel.liveTexts = null
    placesel.liveTurn = null
    return
  }
  placesel.live = { spread: s.id, boxes: new Map(placeIds.map((id, i) => [id, r.places[i]])) }
  placesel.liveTurn = r.deg ?? null
  placesel.liveTexts = new Map(
    textIds.map((id, i) => {
      const t = r.texts[i]
      const orig = s.texts.find((x) => x.id === id)
      return [id, { x: t.x, y: t.y, w: t.w, h: t.h, rot: t.rot, scale: orig && orig.style.size > 0 ? t.size / orig.style.size : 1 }]
    }),
  )
}

// Перенос или пропорциональный масштаб: один TransformGroup (k и d уже зажаты — Go зажмёт так же).
function xfStep(out: GxOut, ax: number, ay: number): Step {
  return {
    places: out.places,
    texts: out.texts,
    call: (sid, ids) => gridAct.placeEdit(sid, Api.TransformGroup(sid, ids, out.k, ax, ay, out.dx, out.dy, '')),
  }
}

// Растяжка по осям рамки: один ScaleGroupAxes.
function axesStep(out: GxAxesOut, phi: number, ax: number, ay: number): Step {
  return {
    places: out.places,
    texts: out.texts,
    call: (sid, ids) => gridAct.placeEdit(sid, Api.ScaleGroupAxes(sid, ids, out.kx, out.ky, phi, ax, ay)),
  }
}

function run(
  e: PointerEvent,
  root: HTMLElement,
  calc: (m: ReturnType<typeof groupMembers>, dx: number, dy: number, ev: PointerEvent) => Step,
  onClick?: () => void,
) {
  const s = store.spread
  if (!s || !placesel.isGroup) return
  const m = groupMembers(s)
  const pids = placesel.selected.map((i) => i.id)
  // Ctrl+Z (или чужая правка) посреди жеста меняет проект: предпросмотр считан с прежнего, не применять.
  const rev = store.view.rev
  drive(
    e,
    root,
    (dx, dy, ev) => calc(m, dx, dy, ev),
    (r) => show(s, m.textIds, pids, r),
    async (r) => {
      if (store.view.rev === rev) await r.call(s.id, m.ids)
    },
    onClick,
  )
}

// Перенос группы. Прилипает описанный прямоугольник группы к остальным местам и краям (Alt — без);
// solo — что сделать, если по члену щёлкнули без движения (оставить только его).
export function startGroupMove(e: PointerEvent, root: HTMLElement, solo?: () => void) {
  const s = store.spread
  if (!s) return
  const skip = new Set(placesel.selected.map((i) => i.id))
  run(
    e,
    root,
    (m, dx0, dy0, ev) => {
      const bb = groupBox(m.in.places, m.in.texts)
      const { dx, dy, axis } = axisLock(dx0, dy0, ev.shiftKey) // Shift во время протяжки держит ось
      let sx = dx
      let sy = dy
      placesel.guides = { x: null, y: null }
      const ctx = bb ? snapCtx(ev, root, s, skip) : null
      if (bb && ctx) {
        // Куда встала бы рамка без прилипания: зажим у краёв делает groupXf, линии считаем по сырой.
        const raw: Rect = { ...bb, x: bb.x + dx, y: bb.y + dy }
        const sn = snapPlaceMove(raw, ctx.others, ctx.params, ctx.thr)
        if (axis !== 'y') sx += sn.x - raw.x
        if (axis !== 'x') sy += sn.y - raw.y
        placesel.guides = { x: axis !== 'y' ? sn.guideX : null, y: axis !== 'x' ? sn.guideY : null }
      }
      return xfStep(groupXf({ ...m.in, k: 1, ax: 0, ay: 0, dx: sx, dy: sy }), 0, 0)
    },
    solo,
  )
}

// Масштаб группы за угол: пропорционально, от противоположного угла рамки (у повёрнутой — её повёрнутого
// угла); кегль текстов тоже. Зажим в разворот (hold повёрнутых) делает groupXf.
export function startGroupScale(e: PointerEvent, root: HTMLElement, corner: GroupCorner) {
  const f = placesel.frame
  if (!f) return
  const a = cornerAnchor(f, corner)
  run(e, root, (m, dx, dy) => xfStep(groupXf({ ...m.in, k: cornerKRot(f, corner, dx, dy), ax: a.x, ay: a.y, dx: 0, dy: 0 }), a.x, a.y))
}

// Боковая ручка (спека 018 п. 5): тянет группу по одной оси рамки, противоположная сторона стоит. С Shift —
// пропорционально от середины противоположной стороны (TransformGroup).
export function startGroupSide(e: PointerEvent, root: HTMLElement, side: GroupSide) {
  const f = placesel.frame
  if (!f) return
  const a = sideAnchor(f, side)
  run(e, root, (m, dx, dy, ev) => {
    const k = sideK(f, side, dx, dy)
    if (ev.shiftKey) {
      const kk = side === 'e' || side === 'w' ? k.kx : k.ky
      return xfStep(groupXf({ ...m.in, k: kk, ax: a.x, ay: a.y, dx: 0, dy: 0 }), a.x, a.y)
    }
    return axesStep(groupAxes({ ...m.in, kx: k.kx, ky: k.ky, phi: f.rot, ax: a.x, ay: a.y }), f.rot, a.x, a.y)
  })
}

// Поворот группы на theta° вокруг (cx, cy): один RotateGroup, после него запоминается угол рамки (phi + theta).
// phi — угол рамки до жеста (во время жеста placesel.phi живой). Холостой угол — без вызова.
export async function rotateGroup(theta: number, cx: number, cy: number, phi = placesel.phi): Promise<boolean> {
  const s = store.spread
  if (!s || theta === 0 || !placesel.isGroup) return false
  // Ключ выделения и rev — до вызова: за ожидание выделение может смениться, а при панике Go вид вернётся прежним.
  const key = placesel.key
  const rev = store.view.rev
  if (!(await gridAct.placeEdit(s.id, Api.RotateGroup(s.id, groupIds(s), theta, cx, cy)))) return false
  if (store.view.rev === rev) return false
  placesel.noteTurn(phi + theta, key)
  return true
}

// ⟳: угол рамки следит за курсором вокруг центра рамки (Shift — 15°, Alt — без прилипания, иначе липнет к
// 0/±90/180); предпросмотр — groupRot, на отпускании — один RotateGroup на разность углов.
export function startGroupRotate(e: PointerEvent, root: HTMLElement) {
  const f = placesel.frame
  if (!f) return
  const r = root.getBoundingClientRect()
  const k = (store.view.settings.pageW * 2) / r.width
  const c = { x: f.x + f.w / 2, y: f.y + f.h / 2 }
  const p0 = { x: (e.clientX - r.left) * k, y: (e.clientY - r.top) * k }
  const grab = cursorDeg(c, p0)
  const phi0 = placesel.phi
  run(e, root, (m, dx, dy, ev) => {
    const theta = normDeg(bdAngle(c, { x: p0.x + dx, y: p0.y + dy }, grab, phi0, ev.shiftKey, ev.altKey) - phi0)
    const out = groupRot({ ...m.in, deg: theta, cx: c.x, cy: c.y })
    return { places: out.places, texts: out.texts, deg: normDeg(phi0 + theta), call: () => rotateGroup(theta, c.x, c.y, phi0) }
  })
}

// Поле карточки группы (X/Y/Ш/В — мм, «Угол» — °): значение абсолютное. Правки идут по очереди, а рамку и φ берём
// внутри очереди по свежему виду: удержанная стрелка не накапливает поворот и масштаб. Мусор — без вызова.
export function setGroupField(f: FrameField | 'a', v: number): Promise<void> {
  const sid = store.spread?.id
  if (!sid) return Promise.resolve()
  return placesel.queue(async () => {
    const sp = store.spread
    const fr = placesel.frame
    if (!sp || sp.id !== sid || !fr || !placesel.isGroup) return
    if (f === 'a') {
      const a = angleField(fr, placesel.phi, v)
      if (a) await rotateGroup(a.deg, a.cx, a.cy)
    } else if (f === 'x' || f === 'y') {
      const a = moveField(fr, f, v)
      if (a) await gridAct.placeEdit(sid, Api.TransformGroup(sid, groupIds(sp), a.k, a.ax, a.ay, a.dx, a.dy, ''))
    } else {
      const a = sizeField(fr, f, v, groupMembers(sp).in)
      if (a) await gridAct.placeEdit(sid, Api.ScaleGroupAxes(sid, groupIds(sp), a.kx, a.ky, a.phi, a.ax, a.ay))
    }
  })
}

// Двойной щелчок по ⟳ — выпрямить группу (поворот на −φ).
export function straightenGroup() {
  const f = placesel.frame
  if (!f) return
  void rotateGroup(normDeg(-placesel.phi), f.x + f.w / 2, f.y + f.h / 2)
}

// Перенос выделенного за член: группа — целиком (с текстами), одно место — как раньше.
// solo вызывается, если по члену группы щёлкнули без движения.
export function moveSelected(e: PointerEvent, root: HTMLElement, solo?: () => void) {
  if (placesel.isGroup) startGroupMove(e, root, solo)
  else startMove(e, root)
}
