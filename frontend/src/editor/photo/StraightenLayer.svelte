<script lang="ts">
  import type { SpreadView } from '../../base/core/types'
  import { canvas } from '../canvas/canvas.svelte'
  import { straighten } from './straighten.svelte'
  import { hitTarget, targetFrame } from './straighten'
  import type { Pt } from '../../base/geom/textbox'

  // Слой режима «Выпрямить» (спека 045) над разворотом цели: ловит мышь целиком, ведёт линию и показывает
  // сетку третей на цели. Нажатие мимо цели — выход; короткая линия — ничего. Углов не считает: в Go уйдёт вектор.
  let { spread }: { spread: SpreadView } = $props()

  let root = $state<HTMLElement>()
  // Линия в px слоя, пока кнопка зажата.
  let line = $state<{ a: Pt; b: Pt } | null>(null)
  let off: (() => void) | null = null

  const t = $derived(straighten.cur?.spread === spread.id ? straighten.cur : null)
  const frame = $derived(t ? targetFrame(spread, t) : null)
  const px = (v: number, of: number) => `${(v / of) * 100}%`

  function stop() {
    if (!off) return
    off()
    off = null
    line = null
    canvas.busy = false
  }
  // Режим закрыли (Esc, смена выделения) — слой размонтируется, протяжка отменяется.
  $effect(() => stop)

  function down(e: PointerEvent) {
    if (off || e.button !== 0 || canvas.space || !root || !t) return
    e.stopPropagation()
    e.preventDefault()
    const r = root.getBoundingClientRect()
    const k = spread.w / r.width // мм на пиксель
    const at = (ev: PointerEvent): Pt => ({ x: ev.clientX - r.left, y: ev.clientY - r.top })
    const a = at(e)
    // Мимо цели — выход, как «щелчок по бледному месту» в режиме фона; по другому фото — ещё и выделение.
    const pt = { x: a.x * k, y: a.y * k }
    if (!hitTarget(spread, t, pt)) {
      straighten.miss(spread, pt)
      return
    }
    line = { a, b: a }
    canvas.busy = true
    const move = (ev: PointerEvent) => (line = { a, b: at(ev) })
    const up = (ev: PointerEvent) => {
      stop()
      void straighten.finish(a, at(ev), k)
    }
    window.addEventListener('pointermove', move)
    window.addEventListener('pointerup', up)
    window.addEventListener('pointercancel', stop)
    off = () => {
      window.removeEventListener('pointermove', move)
      window.removeEventListener('pointerup', up)
      window.removeEventListener('pointercancel', stop)
    }
  }

  // Esc — до остальных обработчиков (фаза захвата): режим закрывается, выделение и режим фона не трогаем.
  function key(e: KeyboardEvent) {
    if (!straighten.key(e)) return
    e.stopPropagation()
    e.preventDefault()
  }
</script>

<svelte:window onkeydowncapture={key} />

{#if t && frame}
  <!-- click и dblclick глушим: иначе щелчок после линии дошёл бы до поля (снял бы выделение) и до «вписать» -->
  <div
    class="layer"
    bind:this={root}
    data-bd-ui
    role="presentation"
    onpointerdown={down}
    onclick={(e) => e.stopPropagation()}
    ondblclick={(e) => e.stopPropagation()}
  >
    {#if frame.area}
      <!-- Область фона обрезает рамку: сетка рисуется только внутри неё -->
      <div
        class="clip"
        style:left={px(frame.area.x, spread.w)}
        style:top={px(frame.area.y, spread.h)}
        style:width={px(frame.area.w, spread.w)}
        style:height={px(frame.area.h, spread.h)}
      >
        <div
          class="box"
          style:left={px(frame.box.x - frame.area.x, frame.area.w)}
          style:top={px(frame.box.y - frame.area.y, frame.area.h)}
          style:width={px(frame.box.w, frame.area.w)}
          style:height={px(frame.box.h, frame.area.h)}
          style:transform={frame.box.rot ? `rotate(${frame.box.rot}deg)` : undefined}
        >
          {@render thirds()}
        </div>
      </div>
    {:else}
      <div
        class="box"
        style:left={px(frame.box.x, spread.w)}
        style:top={px(frame.box.y, spread.h)}
        style:width={px(frame.box.w, spread.w)}
        style:height={px(frame.box.h, spread.h)}
        style:transform={frame.box.rot ? `rotate(${frame.box.rot}deg)` : undefined}
      >
        {@render thirds()}
      </div>
    {/if}
    {#if line}
      <svg class="line" aria-hidden="true">
        <line x1={line.a.x} y1={line.a.y} x2={line.b.x} y2={line.b.y} class="under" />
        <line x1={line.a.x} y1={line.a.y} x2={line.b.x} y2={line.b.y} class="over" />
      </svg>
    {/if}
  </div>
{/if}

{#snippet thirds()}
  <!-- Сетка третей видна, только пока тянут линию: по ней глаз сверяет горизонт -->
  {#if line}
    <span class="third v" style:left="33.333%"></span>
    <span class="third v" style:left="66.666%"></span>
    <span class="third h" style:top="33.333%"></span>
    <span class="third h" style:top="66.666%"></span>
  {/if}
{/snippet}

<style>
  /* Выше рамки фона (BackdropEditor, 40) и всех рамок выделения */
  .layer {
    position: absolute;
    inset: 0;
    z-index: 45;
    cursor: crosshair;
    touch-action: none;
  }
  .clip {
    position: absolute;
    overflow: hidden;
    pointer-events: none;
  }
  .box {
    position: absolute;
    box-sizing: border-box;
    border: 1px dashed var(--accent);
    transform-origin: 0 0;
    pointer-events: none;
  }
  .third {
    position: absolute;
    background: rgba(255, 255, 255, 0.55);
  }
  .third.v {
    top: 0;
    bottom: 0;
    width: 1px;
  }
  .third.h {
    left: 0;
    right: 0;
    height: 1px;
  }
  .line {
    position: absolute;
    inset: 0;
    width: 100%;
    height: 100%;
    overflow: visible;
    pointer-events: none;
  }
  .under {
    stroke: rgba(0, 0, 0, 0.6);
    stroke-width: 4;
    stroke-linecap: round;
  }
  .over {
    stroke: var(--accent);
    stroke-width: 2;
    stroke-linecap: round;
  }
</style>
