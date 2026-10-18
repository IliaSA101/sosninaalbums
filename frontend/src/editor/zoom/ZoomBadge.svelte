<script lang="ts">
  import { tick, untrack } from 'svelte'
  import { prefs } from '../../base/prefs/prefs.svelte'
  import { store } from '../../base/core/store.svelte'
  import { drag } from '../../base/drag/drag.svelte'
  import { canvas } from '../canvas/canvas.svelte'
  import { row } from '../spread/spreadrow.svelte'
  import { percent } from './zoom'
  import { keymap } from '../../base/keys/keymap.svelte'
  import { zoomui } from './zoomui.svelte'
  import ZoomSlider from './ZoomSlider.svelte'

  const p = $derived(percent(canvas.cur.z))
  const can = $derived(prefs.can('zoom'))
  // Клавиша «вписать» — из реестра: после переназначения подсказка не врёт
  const fitTip = $derived('Ctrl+колесо — масштаб' + (keymap.hint('fitZoom') ? `, ${keymap.hint('fitZoom')} — вписать` : ''))

  let badge: HTMLButtonElement | undefined = $state()
  let slider: ZoomSlider | undefined = $state()

  // Любая смена масштаба — колесо, щипок, клавиши, меню, двойной щелчок по фону — показывает бегунок (подсказка).
  // Первый запуск эффекта — не смена.
  let seen = canvas.zoomSeq
  $effect(() => {
    const n = canvas.zoomSeq
    if (n === seen) return
    seen = n
    if (can) untrack(() => zoomui.pulse())
  })
  // Уровень без зума или уход бейджа (нет разворота): бегунок закрывается.
  $effect(() => {
    if (!can) zoomui.close()
  })
  $effect(() => () => zoomui.close())

  // Открыть и перевести фокус на ручку (клавиатура).
  async function openFocus() {
    zoomui.open()
    await tick()
    slider?.focus()
  }

  // Мышь: одиночный щелчок открывает или закрывает бегунок; в паре щелчков первый работает как одиночный,
  // второй его обратно не переключает, поэтому бегунок не мигает. Без зума щелчок вписывает.
  function click(e: MouseEvent) {
    if (!can) return canvas.fit()
    if (e.detail >= 2) return
    if (e.detail === 0) void openFocus()
    else zoomui.toggle()
  }
  function dblclick() {
    if (!can || !store.spread || canvas.busy || canvas.pressed || drag.active || row.atNew) return
    row.finish()
    canvas.fit()
  }
  // Enter и Пробел на бейдже открывают бегунок; Пробел иначе ушёл бы в хоткей раскладок.
  function keydown(e: KeyboardEvent) {
    if (!can || (e.key !== 'Enter' && e.key !== ' ') || e.ctrlKey || e.metaKey || e.altKey) return
    e.preventDefault()
    e.stopPropagation()
    if (!e.repeat) void openFocus()
  }

  // Щелчок мимо закрывает бегунок и не проглатывается. Esc: только бегунок, остальное не трогаем.
  function outside(e: PointerEvent) {
    if (zoomui.state !== 'closed' && !(e.target as Element | null)?.closest?.('.zoomctl')) zoomui.close()
  }
  function esc(e: KeyboardEvent) {
    if (e.key !== 'Escape' || zoomui.state !== 'open') return
    e.stopPropagation()
    e.preventDefault()
    if (document.activeElement?.closest('.zoomctl')) badge?.focus()
    zoomui.close()
  }
</script>

<svelte:window onpointerdowncapture={outside} onkeydowncapture={esc} />

<!-- Масштаб холста виден всегда; щелчок — бегунок, двойной щелчок — вписать (без зума щелчок вписывает) -->
<div class="zoomctl" data-chrome data-guide="zoom-badge">
  {#if can}<ZoomSlider bind:this={slider} />{/if}
  <button
    bind:this={badge}
    class="zoom-badge"
    class:fit={p === 100}
    onclick={click}
    ondblclick={dblclick}
    onkeydown={keydown}
    onpointerenter={() => zoomui.hover(true)}
    onpointerleave={() => zoomui.hover(false)}
    title={fitTip}
    aria-label={can ? `Масштаб ${p} %. Щелчок — бегунок, двойной щелчок — вписать` : `Масштаб ${p} %, вписать`}>{p} %</button
  >
</div>

<style>
  /* Бегунок слева от бейджа на одной высоте, зазор 6 px; обёртка сама мыши не ловит */
  .zoomctl {
    position: absolute;
    right: 10px;
    bottom: 10px;
    z-index: 20;
    display: flex;
    align-items: center;
    gap: 6px;
    pointer-events: none;
  }
  .zoom-badge {
    pointer-events: auto;
    height: 24px;
    padding: 0 8px;
    font-size: 12px;
    font-variant-numeric: tabular-nums;
    background: rgba(32, 32, 34, 0.85);
    border: 1px solid var(--line);
    color: var(--text);
  }
  .zoom-badge.fit {
    color: var(--muted);
    opacity: 0.6;
  }
  .zoom-badge:hover:not(:disabled) {
    background: #48484d;
    opacity: 1;
  }
</style>
