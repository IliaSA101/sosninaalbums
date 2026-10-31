<script lang="ts">
  import { store } from '../../base/core/store.svelte'
  import { drag } from '../../base/drag/drag.svelte'
  import { canvas } from '../canvas/canvas.svelte'
  import { row } from '../spread/spreadrow.svelte'
  import { MAX_ZOOM, percent, posZoom, ZOOM_STEP, zoomPos } from './zoom'
  import { zoomui } from './zoomui.svelte'

  const SCALE = 1000 // шагов на всю шкалу
  const pos = $derived(Math.round(zoomPos(canvas.cur.z) * SCALE))
  const p = $derived(percent(canvas.cur.z))

  let input: HTMLInputElement | undefined = $state()
  export const focus = () => input?.focus()

  // Те же стражи, что у колеса и Ctrl+=: посреди жеста, переноса и на слоте «новый» масштаб не меняется.
  const blocked = () => !store.spread || canvas.busy || canvas.pressed || drag.active || row.atNew
  function apply(z: number) {
    if (blocked()) return false
    row.finish() // зум не начинается посреди движения ряда
    canvas.setZoom(z)
    return true
  }

  function oninput(e: Event & { currentTarget: HTMLInputElement }) {
    const el = e.currentTarget
    // Отброшено стражем — ручка возвращается на текущий масштаб.
    if (!apply(posZoom(Number(el.value) / SCALE))) el.value = String(pos)
  }

  // Мышь ручку берёт: подсказка закрепляется. Фокус после щелчка снимаем, чтобы стрелки дальше двигали
  // выделенное; если ручка уже была в фокусе с клавиатуры, оставляем.
  function pointerdown() {
    zoomui.grab()
    if (document.activeElement === input) return
    window.addEventListener('pointerup', () => input?.blur(), { once: true, capture: true })
  }

  // Клавиши ручки не уходят в общие хоткеи (стрелки не двигают места). Esc — в ZoomBadge.
  function keydown(e: KeyboardEvent) {
    if (e.ctrlKey || e.metaKey || e.altKey) return
    const z = canvas.cur.z
    const to: Record<string, number> = {
      ArrowLeft: z / ZOOM_STEP,
      ArrowDown: z / ZOOM_STEP,
      ArrowRight: z * ZOOM_STEP,
      ArrowUp: z * ZOOM_STEP,
      PageDown: z / 2,
      PageUp: z * 2,
      Home: 1,
      End: MAX_ZOOM,
    }
    if (!(e.key in to)) return
    e.preventDefault()
    e.stopPropagation()
    apply(to[e.key])
  }
</script>

<!-- Бегунок масштаба (спека 016): только полоса и ручка, число — в бейдже рядом -->
<div
  class="slider"
  class:shown={zoomui.state !== 'closed'}
  inert={zoomui.state === 'closed'}
  role="presentation"
  onpointerenter={() => zoomui.hover(true)}
  onpointerleave={() => zoomui.hover(false)}
>
  <input
    bind:this={input}
    type="range"
    min="0"
    max={SCALE}
    step="1"
    value={pos}
    aria-label="Масштаб"
    aria-valuetext="{p} %"
    tabindex={zoomui.state === 'open' ? 0 : -1}
    {oninput}
    onpointerdown={pointerdown}
    onfocus={() => zoomui.grab()}
    onkeydown={keydown}
  />
</div>

<style>
  .slider {
    display: flex;
    align-items: center;
    height: 24px;
    padding: 0 8px;
    background: rgba(32, 32, 34, 0.85);
    border: 1px solid var(--line);
    pointer-events: auto;
    opacity: 0;
    visibility: hidden;
    transition:
      opacity 0.3s,
      visibility 0s 0.3s; /* гаснет плавно, затем уходит из доступности */
  }
  .slider.shown {
    opacity: 1;
    visibility: visible;
    transition: opacity 0.15s;
  }
  /* Блеклый серый, как полоса прокрутки ленты; глобальный accent-color здесь перекрыт */
  input {
    appearance: none;
    width: 160px;
    height: 16px;
    margin: 0;
    background: transparent;
    accent-color: initial;
    cursor: pointer;
  }
  input:focus {
    outline: none;
  }
  input::-webkit-slider-runnable-track {
    height: 4px;
    border-radius: 2px;
    background: #4a4a50;
  }
  input::-webkit-slider-thumb {
    appearance: none;
    width: 12px;
    height: 12px;
    margin-top: -4px; /* (4 − 12) / 2: ручка по центру дорожки */
    border: 0;
    border-radius: 50%;
    background: #8a8a90;
  }
  input:hover::-webkit-slider-thumb {
    background: #a4a4aa;
  }
  input:focus-visible::-webkit-slider-thumb {
    box-shadow: 0 0 0 2px var(--muted);
  }
</style>
