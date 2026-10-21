<script lang="ts">
  import type { CoverView } from '../../base/core/types'
  import { drag } from '../../base/drag/drag.svelte'
  import { backdropAccepts } from '../../base/drag/drop'
  import { COVER_Z, SPINE_HIT_PX, boxCss, dropOrder, scopeOfDrop } from './cover'

  // Пустые зоны броска (спека 032, п. 12): раскладка обложки решает, какие области принимают фото фона.
  // Слой лежит под местами и текстами, поэтому бросок в место выигрывает у зоны; ловит мышь только
  // пока тащат фото. Цель — тот же `backdrop`, что у карточки: фото встаёт на фон области и остаётся в своём развороте.
  let { cover }: { cover: CoverView } = $props()

  const sheet = $derived({ w: cover.sheet.w, h: cover.sheet.h })
  const live = $derived(drag.active && drag.payload?.kind === 'photos')
</script>

{#each dropOrder(cover.drop) as z (z.kind)}
  {@const scope = scopeOfDrop(z.kind)}
  {#if scope}
    <div
      class="drop"
      class:live
      class:over={drag.over === `backdrop:${scope}` && backdropAccepts(drag.payload)}
      class:narrow={z.kind === 'spine'}
      style="{boxCss(z.rect, sheet)};z-index:{COVER_Z.drop};--spine-hit:{SPINE_HIT_PX}px"
      data-drop="backdrop"
      data-spread={cover.sheet.id}
      data-scope={scope}
      role="presentation"
    >
      {#if z.kind === 'spine'}<i class="grab" aria-hidden="true"></i>{:else}<span class="hint">Перетащите фото</span>{/if}
      {#if z.label}<span class="cap">{z.label}</span>{/if}
    </div>
  {/if}
{/each}

<style>
  .drop {
    position: absolute;
    box-sizing: border-box;
    display: grid;
    align-content: center;
    justify-items: center;
    gap: 2px;
    border: 2px dashed rgba(0, 0, 0, 0.35);
    background: rgba(0, 0, 0, 0.04);
    color: #6a6a70;
    text-align: center;
    font-size: 13px;
    pointer-events: none;
    transition:
      border-color 0.15s,
      background 0.15s;
  }
  .drop.live {
    pointer-events: auto;
  }
  .drop.over {
    border-color: var(--accent);
    background: rgba(245, 217, 10, 0.18);
    color: #1e1e1e;
  }
  .cap {
    font-size: 11px;
    opacity: 0.8;
  }
  /* Корешок 6–30 мм = 5–50 px: подсказки нет, обводка пока тащат, табличка при наведении (087-1) */
  .narrow {
    padding: 0;
    border-width: 1px;
  }
  .narrow.live {
    outline: 2px solid var(--accent);
    outline-offset: 0;
  }
  .narrow.over {
    background: rgba(245, 217, 10, 0.45);
  }
  /* Полоса попадания: не уже 24 px по центру зоны, шире корешок — равна зоне; ловит мышь как зона */
  .grab {
    position: absolute;
    inset-block: 0;
    left: 50%;
    width: max(100%, var(--spine-hit));
    transform: translateX(-50%);
  }
  .narrow .cap {
    display: none;
  }
  .narrow.over .cap {
    display: block;
    position: absolute;
    bottom: 100%;
    left: 50%;
    transform: translateX(-50%);
    margin-bottom: 4px;
    padding: 2px 6px;
    border-radius: 3px;
    background: #1e1e1e;
    color: #f2f2f2;
    opacity: 1;
    white-space: nowrap;
    font-size: 11px;
    pointer-events: none;
  }
</style>
