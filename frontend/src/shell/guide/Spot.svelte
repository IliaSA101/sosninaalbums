<script lang="ts">
  import { spot } from './spotstate.svelte'
  import { placeLabel } from './spot'

  // Кольцо и подпись «показать» (спека 033). position: fixed поверх всего, мышь не перехватывает.
  // Подпись читает скринридер: область status живёт всегда, иначе новое содержимое не озвучится.
  let winW = $state(0)
  let winH = $state(0)
  let lw = $state(0)
  let lh = $state(0)
  const at = $derived(spot.box ? placeLabel(spot.box, { w: lw, h: lh }, { w: winW, h: winH }) : null)
</script>

<svelte:window bind:innerWidth={winW} bind:innerHeight={winH} />

<div class="sr" role="status" aria-live="polite">
  {#if spot.cur && (spot.cur.label || spot.cur.hint)}{spot.cur.label}. {spot.cur.hint}{/if}
</div>

{#if spot.cur && spot.box}
  <div
    class="ring"
    class:still={spot.cur.reduced}
    style:left="{spot.box.left}px"
    style:top="{spot.box.top}px"
    style:width="{spot.box.width}px"
    style:height="{spot.box.height}px"
    aria-hidden="true"
  ></div>
  {#if spot.cur.label || spot.cur.hint}
    <div
      class="label"
      bind:clientWidth={lw}
      bind:clientHeight={lh}
      style:left="{at?.left ?? 0}px"
      style:top="{at?.top ?? 0}px"
      style:visibility={lh ? undefined : 'hidden'}
      aria-hidden="true"
    >
      <b>{spot.cur.label}</b>
      {#if spot.cur.hint}<span>{spot.cur.hint}</span>{/if}
    </div>
  {/if}
{/if}

<style>
  .sr {
    position: fixed;
    width: 1px;
    height: 1px;
    overflow: hidden;
    clip-path: inset(50%);
    white-space: nowrap;
  }
  .ring,
  .label {
    position: fixed;
    z-index: 90;
    pointer-events: none;
  }
  .ring {
    border: 2px solid var(--accent);
    border-radius: 8px;
    /* пульсы по 0,8 с до конца показа: его длину (SPOT_MS / SPOT_HINT_MS в spot.ts) держит таймер */
    animation: pulse 0.8s ease-out infinite;
  }
  .ring.still {
    animation: none;
    box-shadow: 0 0 0 4px rgba(245, 217, 10, 0.35);
  }
  @keyframes pulse {
    from {
      box-shadow: 0 0 0 0 rgba(245, 217, 10, 0.7);
    }
    to {
      box-shadow: 0 0 0 10px rgba(245, 217, 10, 0);
    }
  }
  @media (prefers-reduced-motion: reduce) {
    .ring {
      animation: none;
      box-shadow: 0 0 0 4px rgba(245, 217, 10, 0.35);
    }
  }
  .label {
    max-width: 280px;
    display: flex;
    flex-direction: column;
    gap: 2px;
    padding: 5px 9px;
    background: var(--accent);
    color: #1e1e1e;
    border-radius: 6px;
    font-size: 12px;
    line-height: 1.3;
    box-shadow: 0 4px 14px rgba(0, 0, 0, 0.45);
  }
  .label span {
    font-weight: 500;
  }
</style>
