<script lang="ts">
  import type { SpreadView } from '../../base/core/types'
  import RotateHandle from '../../base/ui/RotateHandle.svelte'
  import { resizeCursor, rotHandleIn } from '../../base/geom/rotbox'
  import { placesel } from '../places/placesel.svelte'
  import { photoui } from '../photo/photoui.svelte'
  import { sticker } from './sticker.svelte'
  import type { StCorner } from './sticker'

  // Рамка одного выделенного стикера (048 Ф2, образец — SelFrame): 4 угловые ручки (пропорция держится всегда, у
  // плашки и волны-ленты — свободно), ⟳ над серединой верхней стороны, полоса по краю — перенос (стикер может лежать
  // под фото, а ручки и края обязаны быть доступны, поэтому рамка выше стека `.content`). Группа — у GroupFrame.
  // Жесты — sticker.svelte.ts; рамка едет за курсором (placesel.liveStickers), картинка встаёт после ответа Go.
  let { spread, width }: { spread: SpreadView; width: number } = $props()

  const sw = $derived(spread.w)
  const ph = $derived(spread.h)
  const ppm = $derived(width / sw) // px на мм
  const corners: StCorner[] = ['nw', 'ne', 'se', 'sw']
  const hx = (h: StCorner) => (h.includes('w') ? 0 : 100)
  const hy = (h: StCorner) => (h.includes('n') ? 0 : 100)
  const ROT_REACH = 30 // px от верхней стороны до ⟳

  const st = $derived(placesel.soloSticker)
  const box = $derived(st ? sticker.boxOf(st) : null)
  const live = $derived(!!placesel.liveStickers)
  // Члены группы: только обводки, рамку и ручки группы рисует GroupFrame.
  const members = $derived(st ? [] : placesel.selectedStickers.map((m) => ({ id: m.id, box: sticker.boxOf(m) })))

  let root: HTMLDivElement
  const pc = (v: number, of: number) => `${(v / of) * 100}%`

  const move = (e: PointerEvent) => {
    if (e.button !== 0 || !st) return
    e.stopPropagation()
    sticker.move(e, root, st)
  }
  const resize = (e: PointerEvent, h: StCorner) => {
    if (e.button !== 0 || !st) return
    e.stopPropagation()
    sticker.resize(e, root, st, h)
  }
  const rotate = (e: PointerEvent) => st && sticker.rotate(e, root, st)
  // Щелчок по краю — не щелчок по фону: выделение остаётся.
  const keep = (e: MouseEvent) => e.stopPropagation()
</script>

<div bind:this={root} class="stkframe" role="presentation">
  {#if st && box}
    <div
      class="frame"
      class:live
      style:left={pc(box.x, sw)}
      style:top={pc(box.y, ph)}
      style:width={pc(box.w, sw)}
      style:height={pc(box.h, ph)}
      style:transform={box.rot ? `rotate(${box.rot}deg)` : undefined}
    >
      {#each ['t', 'b', 'l', 'r'] as side (side)}
        <span
          class="edge {side}"
          role="presentation"
          onpointerdown={move}
          onclick={keep}
          oncontextmenu={(e) => photoui.openSticker(e, st.id)}
        ></span>
      {/each}
      {#each corners as h (h)}
        <span
          class="h {h}"
          style:left="{hx(h)}%"
          style:top="{hy(h)}%"
          style:cursor={resizeCursor(h, box.rot)}
          role="presentation"
          onpointerdown={(e) => resize(e, h)}
          onclick={keep}
        ></span>
      {/each}
      <RotateHandle rotIn={rotHandleIn(box, ROT_REACH / ppm, sw, ph)} onrotate={rotate} onzero={() => sticker.zero(st)} />
    </div>
  {/if}

  {#each members as m (m.id)}
    <div
      class="frame member"
      class:live
      style:left={pc(m.box.x, sw)}
      style:top={pc(m.box.y, ph)}
      style:width={pc(m.box.w, sw)}
      style:height={pc(m.box.h, ph)}
      style:transform={m.box.rot ? `rotate(${m.box.rot}deg)` : undefined}
    ></div>
  {/each}

  {#if placesel.guides.x !== null}
    <div class="guide gv" style:left={pc(placesel.guides.x, sw)}></div>
  {/if}
  {#if placesel.guides.y !== null}
    <div class="guide gh" style:top={pc(placesel.guides.y, ph)}></div>
  {/if}
</div>

<style>
  .stkframe {
    position: absolute;
    inset: 0;
    z-index: 7; /* над всем контентом (.content, 3), притемнением (5) и безопасной зоной (6) */
    pointer-events: none;
  }
  .frame {
    position: absolute;
    box-sizing: border-box;
    outline: 3px solid var(--accent);
    outline-offset: -1px;
  }
  .frame.member {
    outline-width: 2px;
  }
  .frame.live {
    background: rgba(245, 217, 10, 0.12);
  }
  /* Полоса по краю внутри рамки: перенос. У совсем мелкого стикера не больше трети стороны. */
  .edge {
    position: absolute;
    pointer-events: auto;
    cursor: move;
    touch-action: none;
  }
  .edge.t,
  .edge.b {
    left: 0;
    right: 0;
    height: min(10px, 33%);
  }
  .edge.t {
    top: 0;
  }
  .edge.b {
    bottom: 0;
  }
  .edge.l,
  .edge.r {
    top: 0;
    bottom: 0;
    width: min(10px, 33%);
  }
  .edge.l {
    left: 0;
  }
  .edge.r {
    right: 0;
  }
  /* Ручка: видимая 8 px, зона попадания 14 px (тачпад). */
  .h {
    position: absolute;
    width: 14px;
    height: 14px;
    transform: translate(-50%, -50%);
    pointer-events: auto;
    touch-action: none;
  }
  .h::before {
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
  .guide {
    position: absolute;
    pointer-events: none;
    background: #4aa3ff;
  }
  .guide.gv {
    top: 0;
    bottom: 0;
    width: 1px;
  }
  .guide.gh {
    left: 0;
    right: 0;
    height: 1px;
  }
</style>
