<script lang="ts">
  import type { Settings, SpreadView } from '../../base/core/types'
  import { placesel } from './placesel.svelte'
  import { prefs } from '../../base/prefs/prefs.svelte'
  import { startGroupMove, startGroupRotate, startGroupScale, startGroupSide, straightenGroup } from './groupgesture'
  import type { GroupCorner } from '../../base/groupgeom/groupxf'
  import type { GroupSide } from '../../base/groupgeom/groupframe'
  import { aabbOf, resizeCursor, rotHandleAt, rotHandleIn } from '../../base/geom/rotbox'
  import MiniPanel from './MiniPanel.svelte'
  import RotateHandle from '../../base/ui/RotateHandle.svelte'

  // Рамка группы (спека 009, 018): описанный прямоугольник выделенного в осях угла рамки, полоса 10 px СНАРУЖИ
  // по краю — перенос группы, 4 угловые ручки снаружи углов — масштаб от противоположного угла, 4 боковые по
  // серединам сторон — растяжка по одной оси, ⟳ над верхней стороной — поворот. Рамка повёрнута целиком
  // (transform), поэтому ручки и полосы идут вместе с ней. Внутрь полоса не заходит: она легла бы на крайних
  // членов, а в «Авто» от этого зависит обмен или закрепление. Обводки членов рисуют SelFrame и TextLayer.
  // Предпросмотр — placesel.live / liveTexts / liveTurn, итог — один вызов Go на отпускании.
  let { spread, settings, width }: { spread: SpreadView; settings: Settings; width: number } = $props()

  const sw = $derived(settings.pageW * 2)
  const ph = $derived(settings.pageH)
  const ppm = $derived(width / sw) // px на мм
  const corners: GroupCorner[] = ['nw', 'ne', 'se', 'sw']
  const sides: GroupSide[] = ['n', 'e', 's', 'w']
  const EDGES = ['t', 'b', 'l', 'r'] as const
  const OUT = 10 // полоса снаружи, px
  const ROT_REACH = 30 // px от верхней стороны до ⟳
  const ROT_MID = 24 // px от стороны до центра кружка ⟳

  const live = $derived(placesel.live?.spread === spread.id ? placesel.live.boxes : null)
  const frame = $derived(placesel.frame)
  // Уровень (спека 018): боковые и угловые ручки — «Масштаб и поворот группы»; ⟳ ещё требует поворот мест
  // и (или) текстов — тех, что есть в группе.
  const scalable = $derived(prefs.can('groupXf'))
  const rotatable = $derived(
    scalable && (placesel.selected.length === 0 || prefs.can('rotate')) && (placesel.texts.length === 0 || prefs.can('textRotate')),
  )
  const rotIn = $derived(frame ? rotHandleIn(frame, ROT_REACH / ppm, sw, ph) : false)
  // Мини-панель стоит за полосой и ручками: рамка для неё шире на их размер; ⟳ она обходит.
  const panelBox = $derived.by(() => {
    if (!frame || live) return null
    const b = aabbOf(frame)
    return { x: b.x - 12 / ppm, y: b.y - 12 / ppm, w: b.w + 24 / ppm, h: b.h + 24 / ppm }
  })
  const rotAt = $derived.by(() => {
    if (!frame || !rotatable) return undefined
    const p = rotHandleAt(frame, (rotIn ? -ROT_MID : ROT_MID) / ppm)
    return { x: p.x * ppm, y: p.y * ppm }
  })

  let root: HTMLDivElement

  const pc = (v: number, of: number) => `${(v / of) * 100}%`
  const move = (e: PointerEvent) => {
    if (e.button !== 0) return
    e.stopPropagation()
    startGroupMove(e, root)
  }
  const scale = (e: PointerEvent, c: GroupCorner) => {
    if (e.button !== 0) return
    e.stopPropagation()
    startGroupScale(e, root, c)
  }
  const stretch = (e: PointerEvent, s: GroupSide) => {
    if (e.button !== 0) return
    e.stopPropagation()
    startGroupSide(e, root, s)
  }
  // Клик по рамке — не клик по фону: выделение остаётся.
  const keep = (e: MouseEvent) => e.stopPropagation()
</script>

<div bind:this={root} class="groupframe" role="presentation">
  {#if frame}
    <div
      class="frame"
      style:left={pc(frame.x, sw)}
      style:top={pc(frame.y, ph)}
      style:width={pc(frame.w, sw)}
      style:height={pc(frame.h, ph)}
      style:transform={frame.rot ? `rotate(${frame.rot}deg)` : undefined}
    >
      {#each prefs.can('manual') ? EDGES : [] as side (side)}
        <span class="edge {side}" style:--out="{OUT}px" role="presentation" onpointerdown={move} onclick={keep}></span>
      {/each}
      {#each scalable ? corners : [] as c (c)}
        <span class="h {c}" style:cursor={resizeCursor(c, frame.rot)} role="presentation" onpointerdown={(e) => scale(e, c)} onclick={keep}
        ></span>
      {/each}
      {#each scalable ? sides : [] as s (s)}
        <span
          class="side {s}"
          style:cursor={resizeCursor(s, frame.rot)}
          role="presentation"
          onpointerdown={(e) => stretch(e, s)}
          onclick={keep}
        ></span>
      {/each}
      {#if rotatable}
        <RotateHandle {rotIn} onrotate={(e) => startGroupRotate(e, root)} onzero={straightenGroup} />
      {/if}
    </div>
  {/if}

  {#if panelBox}
    <MiniPanel {spread} box={panelBox} {width} height={(width * ph) / sw} {ppm} avoid={rotAt} />
  {/if}
</div>

<style>
  .groupframe {
    position: absolute;
    inset: 0;
    z-index: 8; /* над рамкой выделения (7) */
    pointer-events: none;
  }
  .frame {
    position: absolute;
    box-sizing: border-box;
    outline: 1px dashed var(--accent);
    outline-offset: 0;
    transform-origin: 50% 50%;
  }
  /* Полоса снаружи рамки: перенос группы. */
  .edge {
    position: absolute;
    pointer-events: auto;
    cursor: move;
    touch-action: none;
  }
  .edge.t,
  .edge.b {
    left: calc(var(--out) * -1);
    right: calc(var(--out) * -1);
    height: var(--out);
  }
  .edge.t {
    bottom: 100%;
  }
  .edge.b {
    top: 100%;
  }
  .edge.l,
  .edge.r {
    top: 0;
    bottom: 0;
    width: var(--out);
  }
  .edge.l {
    right: 100%;
  }
  .edge.r {
    left: 100%;
  }
  /* Ручка снаружи угла и посередине стороны: зона 14 px, видимый квадрат 8 px (тачпад). */
  .h,
  .side {
    position: absolute;
    width: 14px;
    height: 14px;
    pointer-events: auto;
    touch-action: none;
  }
  .h::before,
  .side::before {
    content: '';
    position: absolute;
    left: 3px;
    top: 3px;
    width: 8px;
    height: 8px;
    box-sizing: border-box;
    background: #fff;
    border: 1.5px solid #1e1e1e;
    border-radius: 2px;
  }
  .h.nw {
    right: 100%;
    bottom: 100%;
  }
  .h.ne {
    left: 100%;
    bottom: 100%;
  }
  .h.se {
    left: 100%;
    top: 100%;
  }
  .h.sw {
    right: 100%;
    top: 100%;
  }
  /* Боковые — центром на стороне: снаружи поместился бы ⟳ рядом с «усиком». */
  .side.n,
  .side.s {
    left: calc(50% - 7px);
  }
  .side.n {
    top: -7px;
  }
  .side.s {
    bottom: -7px;
  }
  .side.e,
  .side.w {
    top: calc(50% - 7px);
  }
  .side.e {
    right: -7px;
  }
  .side.w {
    left: -7px;
  }
</style>
