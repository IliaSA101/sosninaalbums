<script lang="ts">
  import { primaryPress } from '../../base/core/platform'
  import type { Frame, FrameView, Rect, SpreadView } from '../../base/core/types'
  import { store, Api } from '../../base/core/store.svelte'
  import { panFrame, sinCosDeg, zoomFrame } from '../../base/geom/frame'
  import { outlineOf } from '../../base/geom/shape'
  import { framelive } from './framelive.svelte'
  import { canvas } from '../canvas/canvas.svelte'
  import { markDone } from '../places/placegesture'
  import { tiltmode } from '../tilt/tiltmode.svelte'
  import { miniSpot } from '../places/minispot.svelte'
  import { NO_ROOM, type ArcRoom } from '../places/minipanel'
  import TiltDial from '../tilt/TiltDial.svelte'

  // Управление кадром на выделенной ячейке: ✥ (сдвиг кадра) и ползунок масштаба. Тело ячейки занято
  // переносом фото или места, поэтому кадр двигает только ✥. Правка уходит в Go при отпускании.
  // Лежит в CellOverlay (вместе с оболочкой `[data-cov]` в осях места), предпросмотр — через framelive.
  // room (063): включён режим наклона и дуге есть место — вместо ползунка масштаба сверху встаёт дуга.
  let {
    id,
    spread,
    r,
    fv,
    frame,
    cur,
    movable,
    zoomable,
    room = NO_ROOM,
  }: {
    id: string
    spread: SpreadView
    r: Rect // ячейка, мм разворота
    fv: FrameView | undefined
    frame: Frame // кадр из Go
    cur: Frame // кадр с учётом предпросмотра (для ползунка)
    movable: boolean
    zoomable: boolean
    room?: ArcRoom
  } = $props()

  const panning = $derived(framelive.panning(id))
  const shaped = $derived(!!outlineOf(fv?.shape, fv?.radius))
  const dial = $derived(room.mode !== 'none')
  // ✥ прячется только на время протяжки: сдвига кадра или кружка дуги
  const hideMove = $derived(panning || (dial && tiltmode.dialing))
  // Ползунок масштаба: над мини-панелью внутри у нижнего края (063) и у нижнего края видимой части, если низ рамки за экраном (070)
  const lift = $derived(miniSpot.photo === id ? Math.max(16, miniSpot.lift) : 16)

  function startPan(e: PointerEvent) {
    if (!primaryPress(e) || !fv) return
    // Оболочка в осях места: по ширине её описанного прямоугольника px переводятся в мм.
    const cell = (e.currentTarget as HTMLElement).closest('[data-cov]')
    if (!cell) return
    canvas.busy = true
    const base = frame
    const baseImg = fv.img
    // Место может быть повёрнуто (cellRot): getBoundingClientRect даёт описанный прямоугольник, поэтому
    // масштаб — по нему и по размеру места, а сдвиг курсора возвращается в оси места (поворот на −cellRot).
    const [sn, cs] = sinCosDeg(fv.cellRot)
    const mm = (r.w * Math.abs(cs) + r.h * Math.abs(sn)) / cell.getBoundingClientRect().width // мм на пиксель экрана
    const sx = e.clientX
    const sy = e.clientY
    let moved = false
    const stop = () => {
      window.removeEventListener('pointermove', move)
      window.removeEventListener('pointerup', up)
      window.removeEventListener('keydown', esc, true)
      canvas.busy = false
    }
    const move = (ev: PointerEvent) => {
      if (!moved && Math.hypot(ev.clientX - sx, ev.clientY - sy) < 3) return
      moved = true
      const dx = (ev.clientX - sx) * mm
      const dy = (ev.clientY - sy) * mm
      // Дальше panFrame переводит сдвиг в оси фото (−r).
      framelive.set(id, panFrame(base, r, baseImg, dx * cs + dy * sn, -dx * sn + dy * cs, outlineOf(fv.shape, fv.radius)), false, true)
    }
    const up = async () => {
      stop()
      const f = moved ? framelive.of(id) : null
      if (!f) return
      markDone() // click после сдвига не должен снимать выделение
      framelive.set(id, f) // кадр остаётся до ответа Go, сетка третей гаснет
      await store.run(Api.SetFrame(id, f.s, f.x, f.y))
      framelive.clear()
    }
    const esc = (ev: KeyboardEvent) => {
      if (ev.key !== 'Escape') return
      ev.stopPropagation() // Esc отменяет только сдвиг: выделение ячейки остаётся и режим наклона тоже
      ev.preventDefault()
      stop()
      framelive.clear()
    }
    window.addEventListener('pointermove', move)
    window.addEventListener('pointerup', up)
    window.addEventListener('keydown', esc, true) // фаза захвата: раньше хоткея «снять выделение»
  }

  // Ползунок масштаба: предпросмотр через framelive, SetFrame — на отпускании.
  const zoomValue = (e: Event) => +(e.currentTarget as HTMLInputElement).value / 100
  async function zoomCommit(e: Event) {
    const f = zoomFrame(frame, zoomValue(e))
    await store.run(Api.SetFrame(id, f.s, f.x, f.y))
    framelive.clear()
  }
</script>

{#if movable && !hideMove}
  <button
    class="handle"
    data-guide="frame"
    title="Сдвинуть кадр"
    aria-label="Сдвинуть кадр"
    onpointerdown={(e) => {
      e.stopPropagation()
      startPan(e)
    }}
    onclick={(e) => e.stopPropagation()}>✥</button
  >
{/if}
{#if dial && fv}
  <TiltDial {id} {spread} {fv} {frame} {room} {shaped} />
{:else if zoomable && !panning}
  <input
    class="zoom"
    class:shaped
    style:bottom={shaped ? `max(calc(16px + 8%), ${lift}px)` : `${lift}px`}
    data-guide="frame"
    type="range"
    min="100"
    max="400"
    step="1"
    value={Math.round(cur.s * 100)}
    aria-label="Масштаб кадра"
    title="Масштаб кадра"
    oninput={(e) => framelive.set(id, zoomFrame(frame, zoomValue(e)))}
    onchange={zoomCommit}
    onpointerdown={(e) => e.stopPropagation()}
    onpointerup={(e) => e.currentTarget.blur()}
    onclick={(e) => e.stopPropagation()}
  />
{/if}

<style>
  .handle {
    position: absolute;
    left: 50%;
    top: 50%;
    width: 32px;
    height: 32px;
    padding: 0;
    transform: translate(-50%, -50%);
    border-radius: 50%;
    background: rgba(20, 20, 22, 0.75);
    color: #fff;
    font-size: 18px;
    line-height: 1;
    cursor: move;
    z-index: 4;
    pointer-events: auto; /* обёртка CellOverlay мышь не ловит, а свойство наследуется */
  }
  /* Ползунок масштаба внутри у нижнего края: выше полосы переноса рамки (10 px) */
  .zoom {
    position: absolute;
    left: 50%;
    bottom: 16px;
    width: min(60%, 170px);
    margin: 0;
    transform: translateX(-50%);
    z-index: 4;
    pointer-events: auto;
    accent-color: var(--accent);
  }
  /* У круга и скругления нижний угол срезан: ползунок выше и уже */
  .zoom.shaped {
    bottom: calc(16px + 8%);
    width: min(50%, 170px);
  }
  :global(body.dragging) .zoom,
  :global(body.dragging) .handle {
    display: none;
  }
</style>
