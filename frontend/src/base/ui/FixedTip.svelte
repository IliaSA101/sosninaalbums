<script lang="ts">
  import { drag } from '../drag/drag.svelte'
  import { tip } from './tip.svelte'
  import { tipLeft } from '../../panels/timeline/timeline'

  // Рисует общую подсказку (ui/tip.svelte.ts) поверх всего интерфейса.
  let tipW = $state(0)
  let ww = $state(window.innerWidth)
</script>

<svelte:window bind:innerWidth={ww} />

{#if tip.state && !drag.active}
  {#if tip.state.side === 'right'}
    <div class="tip right" style:left="{tip.state.x + 8}px" style:top="{tip.state.y}px">{tip.state.text}</div>
  {:else}
    <div class="tip" bind:clientWidth={tipW} style:left="{tipLeft(tip.state.x, tipW, ww)}px" style:top="{tip.state.y - 8}px">
      {tip.state.text}
    </div>
  {/if}
{/if}

<style>
  .tip {
    position: fixed;
    z-index: 95;
    transform: translateY(-100%);
    padding: 4px 8px;
    border-radius: 4px;
    background: #111;
    border: 1px solid #444;
    color: var(--text);
    font-size: 12px;
    white-space: nowrap;
    pointer-events: none;
  }
  .tip.right {
    transform: translateY(-50%);
  }
</style>
