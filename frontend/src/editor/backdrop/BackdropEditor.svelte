<script lang="ts">
  import { primaryPress } from '../../base/core/platform'
  import type { SpreadView } from '../../base/core/types'
  import { store } from '../../base/core/store.svelte'
  import { coverSides } from '../cover/cover'
  import { bgmode } from './bgmode.svelte'
  import { backdropAt, modeClick, roomForSlider, type BgLive } from './bgmode'
  import { bdAngle, bdLines, moveFrame, panBox, resizeSide, scaleFrame, type Side } from '../../base/geom/bdbox'
  import { center, cursorDeg, toSpread, type Corner, type Pt } from '../../base/geom/textbox'
  import { hasSlack, zoomFrame } from '../../base/geom/frame'
  import { snapThreshold } from '../../base/geom/snap'
  import { canvas } from '../canvas/canvas.svelte'
  import { drag } from '../../base/drag/drag.svelte'
  import { placesel } from '../places/placesel.svelte'
  import { axisLock } from '../../base/geom/gridedit'
  import { turnBackdropBox } from './backdrop'
  import BackdropHandles from './BackdropHandles.svelte'

  // Режим фона (спека 009, design-backdrop §6): над приглушённым разворотом — область пунктиром,
  // рамка фона с ручками, ✥ и ползунок кадра. Предпросмотр жеста — bgmode.live (его рисует и
  // BackdropLayer), в Go — один вызов на отпускании. Сам слой — цель броска: фото заменит фон области.
  let { spread, width }: { spread: SpreadView; width: number } = $props()

  let root = $state<HTMLElement>()
  const sw = $derived(spread.w)
  const b = $derived(bgmode.view)
  const t = $derived(bgmode.live?.box ?? b?.box)
  const fr = $derived(bgmode.live?.frame ?? b?.frame)
  const px = (v: number) => `${(v / sw) * 100}%`
  const py = (v: number) => `${(v / spread.h) * 100}%`
  // ✥ — только когда фото больше рамки: иначе двигать нечего.
  const movable = $derived(!!b && hasSlack({ x: 0, y: 0, w: b.box.w, h: b.box.h }, b.img, b.frame.r ?? 0))
  let panning = $state(false)
  // Ползунок — только в рамке не мельче 48 px (как у фото в ячейке); иначе масштаб в карточке справа.
  const slider = $derived(!!t && width > 0 && roomForSlider(t, width / sw))
  // ⟳ над верхней стороной (30 px); вышел бы за разворот (рамка по области) — прячем внутрь рамки.
  const rotIn = $derived.by(() => {
    if (!t || !(width > 0)) return false
    const p = toSpread(t, t.w / 2, -30 * (sw / width))
    return p.y < 0 || p.y > spread.h || p.x < 0 || p.x > sw
  })

  type Step = (dx: number, dy: number, ev: PointerEvent, p: Pt) => BgLive
  // Общий жест: порог 3 px, Esc отменяет (до Esc режима — фаза захвата), правка — на отпускании.
  function gesture(e: PointerEvent, step: Step, done: (l: BgLive) => Promise<void>) {
    const cur = b
    if (!primaryPress(e) || canvas.space || !cur || !root) return
    e.stopPropagation()
    e.preventDefault()
    const r = root.getBoundingClientRect()
    const k = sw / r.width // мм на пиксель
    const sx = e.clientX
    const sy = e.clientY
    let moved = false
    canvas.busy = true
    const stop = () => {
      window.removeEventListener('pointermove', move)
      window.removeEventListener('pointerup', up)
      window.removeEventListener('pointercancel', cancel)
      window.removeEventListener('keydown', esc, true)
      canvas.busy = false
      panning = false
      bgmode.guides = { x: null, y: null }
    }
    const move = (ev: PointerEvent) => {
      if (!moved && Math.hypot(ev.clientX - sx, ev.clientY - sy) < 3) return
      moved = true
      bgmode.live = step((ev.clientX - sx) * k, (ev.clientY - sy) * k, ev, { x: (ev.clientX - r.left) * k, y: (ev.clientY - r.top) * k })
    }
    const up = async () => {
      stop()
      const l = bgmode.live
      if (moved && l) await done(l)
      bgmode.live = null
    }
    // Система отобрала указатель (pointercancel) — жест отменяется, как по Esc.
    const cancel = () => {
      stop()
      bgmode.live = null
    }
    const esc = (ev: KeyboardEvent) => {
      if (ev.key !== 'Escape') return
      ev.stopPropagation() // Esc жеста режим не закрывает
      ev.preventDefault()
      cancel()
    }
    window.addEventListener('pointermove', move)
    window.addEventListener('pointerup', up)
    window.addEventListener('pointercancel', cancel)
    window.addEventListener('keydown', esc, true)
  }

  const commitBox = (l: BgLive) => bgmode.setBox(l.box)

  function startMove(e: PointerEvent) {
    const base = b!
    const lines = bdLines(base.area, spread.mid, coverSides(spread, store.view.cover))
    const thr = snapThreshold(root?.getBoundingClientRect().width ?? 0, spread.w)
    gesture(
      e,
      (dx0, dy0, ev) => {
        const { dx, dy, axis } = axisLock(dx0, dy0, ev.shiftKey) // Shift во время протяжки держит ось
        const m = moveFrame(base.box, dx, dy, base.area, lines, placesel.snap && !ev.altKey ? thr : 0)
        bgmode.guides = { x: axis === 'y' ? null : m.guideX, y: axis === 'x' ? null : m.guideY }
        return { box: { ...base.box, x: axis === 'y' ? base.box.x : m.x, y: axis === 'x' ? base.box.y : m.y }, frame: base.frame }
      },
      commitBox,
    )
  }

  function startSide(e: PointerEvent, s: Side) {
    const base = b!
    gesture(e, (dx, dy) => ({ box: resizeSide(base.box, s, dx, dy, base.area), frame: base.frame }), commitBox)
  }

  function startCorner(e: PointerEvent, c: Corner) {
    const base = b!
    gesture(e, (dx, dy) => ({ box: scaleFrame(base.box, c, dx, dy, base.area), frame: base.frame }), commitBox)
  }

  function startRotate(e: PointerEvent) {
    const base = b!
    if (!root) return
    const r = root.getBoundingClientRect()
    const k = sw / r.width
    const c = center(base.box)
    const grab = cursorDeg(c, { x: (e.clientX - r.left) * k, y: (e.clientY - r.top) * k })
    gesture(
      e,
      (_dx, _dy, ev, p) => ({
        box: turnBackdropBox(base.box, bdAngle(c, p, grab, base.box.rot, ev.shiftKey, ev.altKey)),
        frame: base.frame,
      }),
      (l) => bgmode.turn(l.box.rot), // коммит считает Go от рамки до жеста
    )
  }

  function startPan(e: PointerEvent) {
    const base = b!
    gesture(
      e,
      (dx, dy) => {
        panning = true
        return { box: base.box, frame: panBox(base.frame, base.box, base.img, dx, dy) }
      },
      (l) => bgmode.setFrame(l.frame),
    )
  }

  // Ползунок: предпросмотр через live, SetBackdropFrame — на отпускании.
  const zoomValue = (e: Event) => +(e.currentTarget as HTMLInputElement).value / 100
  async function zoomCommit(e: Event) {
    if (!b) return
    await bgmode.setFrame(zoomFrame(b.frame, zoomValue(e)))
    bgmode.live = null
  }

  // Нажатие мимо рамки и ручек (поле, приглушённое, серое поле): по фото фона другой страницы —
  // перейти к нему, иначе выход. Ничего не выделяет: событие дальше не идёт. Лента, карточка,
  // полоса и края соседних разворотов работают как обычно.
  function outside(e: PointerEvent) {
    const c = bgmode.cur
    if (!primaryPress(e) || canvas.space || !c || !root) return
    const el = e.target as Element | null
    if (!el?.closest('.stage') || el.closest('[data-chrome], [data-bd-ui], [data-peer], .zoom-badge')) return
    e.stopPropagation()
    const r = root.getBoundingClientRect()
    const k = sw / r.width
    const at = backdropAt(spread.backdrops, { x: (e.clientX - r.left) * k, y: (e.clientY - r.top) * k })
    const act = modeClick(at, c.scope)
    if (act === 'switch' && at) bgmode.enter(spread.id, at)
    else if (act === 'exit') bgmode.exit()
  }
</script>

<svelte:window onpointerdowncapture={outside} />

{#if b && t && fr}
  <div class="bdedit" bind:this={root} data-drop="backdrop" data-spread={spread.id} data-scope={b.scope} role="presentation">
    <div
      class="area"
      class:over={drag.over === `backdrop:${b.scope}`}
      style:left={px(b.area.x)}
      style:top={py(b.area.y)}
      style:width={px(b.area.w)}
      style:height={py(b.area.h)}
    ></div>
    {#if bgmode.guides.x !== null}<div class="guide v" style:left={px(bgmode.guides.x)}></div>{/if}
    {#if bgmode.guides.y !== null}<div class="guide h" style:top={py(bgmode.guides.y)}></div>{/if}
    <div
      class="box"
      data-bd-ui
      role="presentation"
      style:left={px(t.x)}
      style:top={py(t.y)}
      style:width={px(t.w)}
      style:height={py(t.h)}
      style:transform={t.rot ? `rotate(${t.rot}deg)` : undefined}
      onpointerdown={startMove}
    >
      <BackdropHandles {rotIn} onside={startSide} oncorner={startCorner} onrotate={startRotate} onzero={() => b && bgmode.turn(0)} />
      {#if movable && !panning}
        <button class="pan" title="Сдвинуть фото в рамке" aria-label="Сдвинуть фото в рамке" onpointerdown={startPan}>✥</button>
      {/if}
      {#if !panning && slider}
        <input
          class="zoom"
          type="range"
          min="100"
          max="400"
          step="1"
          value={Math.round(fr.s * 100)}
          aria-label="Масштаб фото в рамке"
          title="Масштаб фото в рамке"
          oninput={(e) => b && (bgmode.live = { box: b.box, frame: zoomFrame(b.frame, zoomValue(e)) })}
          onchange={zoomCommit}
          onpointerdown={(e) => e.stopPropagation()}
          onpointerup={(e) => e.currentTarget.blur()}
        />
      {/if}
    </div>
  </div>
{/if}

<style>
  /* Над приглушёнными местами и текстами; ловит мышь по всему развороту (бросок — замена фото фона). */
  .bdedit {
    position: absolute;
    inset: 0;
    z-index: 40;
  }
  .area {
    position: absolute;
    box-sizing: border-box;
    border: 1px dashed rgba(245, 217, 10, 0.7);
    pointer-events: none;
    transition: background 0.15s;
  }
  .area.over {
    background: rgba(245, 217, 10, 0.12);
  }
  .guide {
    position: absolute;
    pointer-events: none;
    background: #3fa9f5;
  }
  .guide.v {
    top: 0;
    bottom: 0;
    width: 1px;
  }
  .guide.h {
    left: 0;
    right: 0;
    height: 1px;
  }
  .box {
    position: absolute;
    box-sizing: border-box;
    border: 1.5px solid var(--accent);
    transform-origin: 0 0;
    cursor: move;
    touch-action: none;
  }
  .pan {
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
    z-index: 1; /* ручки (z-index 2) всегда выше */
  }
  .zoom {
    position: absolute;
    left: 50%;
    bottom: 16px;
    width: min(60%, 170px);
    margin: 0;
    transform: translateX(-50%);
    accent-color: var(--accent);
    z-index: 1;
  }
  :global(body.dragging) .box > * {
    display: none;
  }
</style>
