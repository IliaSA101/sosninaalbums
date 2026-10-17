<script lang="ts">
  import type { CoverView } from '../../base/core/types'
  import { drag } from '../../base/drag/drag.svelte'
  import { backdropAccepts } from '../../base/drag/drop'
  import { COVER_Z, boxCss, scopeOfDrop } from './cover'

  // Пустые зоны броска (спека 032, п. 12): раскладка обложки решает, какие области принимают фото фона.
  // Слой лежит под местами и текстами, поэтому бросок в место выигрывает у зоны; ловит мышь только
  // пока тащат фото. Цель — тот же `backdrop`, что у карточки: фото встаёт на фон области и остаётся в своём развороте.
  let { cover }: { cover: CoverView } = $props()

  const sheet = $derived({ w: cover.sheet.w, h: cover.sheet.h })
  const live = $derived(drag.active && drag.payload?.kind === 'photos')
</script>

{#each cover.drop as z (z.kind)}
  {@const scope = scopeOfDrop(z.kind)}
  {#if scope}
    <div
      class="drop"
      class:live
      class:over={drag.over === `backdrop:${scope}` && backdropAccepts(drag.payload)}
      class:narrow={z.kind === 'spine'}
      style="{boxCss(z.rect, sheet)};z-index:{COVER_Z.drop}"
      data-drop="backdrop"
      data-spread={cover.sheet.id}
      data-scope={scope}
      role="presentation"
    >
      {#if z.kind !== 'spine'}<span class="hint">Перетащите фото</span>{/if}
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
  /* Корешок 6–30 мм = 10–50 px: подсказки нет, подпись вертикально */
  .narrow {
    padding: 0;
    border-width: 1px;
  }
  .narrow .cap {
    writing-mode: vertical-rl;
    white-space: nowrap;
    font-size: 10px;
  }
</style>
