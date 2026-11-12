<script lang="ts">
  import { store } from '../../base/core/store.svelte'
  import type { CoverLayout, SpreadView } from '../../base/core/types'
  import { prefs } from '../../base/prefs/prefs.svelte'
  import Icon from '../../base/ui/Icon.svelte'
  import { COVER_LAYOUTS } from '../../editor/cover/cover'
  import InspectorBackdrop from '../../editor/backdrop/InspectorBackdrop.svelte'
  import CardGroup from '../../editor/inspector/CardGroup.svelte'
  import { useCard } from '../../base/prefs/cardgroups.svelte'
  import { coverAct } from './coveract.svelte'
  import { layoutLabel } from './coverpanel'

  // Группа «Фото фона» (спека 124 U6, 0.26-124-7, -30; имя — 0.26-127-6): три плитки со схемой в ряд и зоны фото под ними.
  // «Раскладка» в программе — места (127), поэтому слово здесь не используем; ключ памяти `cover.layout` прежний.
  // Зоны задаёт раскладка, поэтому они здесь, а не в «Фоне». dropOpen: пока тянут фото, свёрнутая группа раскрыта.
  // В узкой карточке (плитка ≈ 55 px) под схемой короткое слово, полная подпись — в title и aria-label.
  let { sp }: { sp: SpreadView } = $props()
  const narrow = useCard().narrow
  const cv = $derived(store.cover)

  const ICON = { front: 'cover-front', wrap: 'cover-wrap', both: 'cover-three' } as const satisfies Record<CoverLayout, string>
</script>

{#if cv}
  <CardGroup id="cover.layout" title="Фото фона" summary={layoutLabel(cv.layout, narrow)} dropOpen>
    <div class="layouts" role="group" aria-label="Схема фото фона">
      {#each COVER_LAYOUTS as l (l.kind)}
        <button
          class="tile"
          class:on={cv.layout === l.kind}
          aria-pressed={cv.layout === l.kind}
          aria-label={l.label}
          title={narrow ? l.label : undefined}
          onclick={() => cv.layout !== l.kind && coverAct.setLayout(l.kind)}
        >
          <Icon name={ICON[l.kind]} size={30} stroke={1.5} />
          <span>{narrow ? l.short : l.label}</span>
        </button>
      {/each}
    </div>
    <!-- Зоны фото фона: цель переноса фото (Inspector подводит эту зону в поле зрения) -->
    {#if prefs.can('bgPhoto')}<div data-slot="backdrop"><InspectorBackdrop {sp} split={false} /></div>{/if}
  </CardGroup>
{/if}

<style>
  .layouts {
    display: grid;
    grid-template-columns: repeat(3, 1fr);
    gap: 4px;
  }
  .tile {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 2px;
    min-width: 0;
    height: auto;
    padding: 4px 2px;
    font-size: 11px;
    line-height: 1.2;
    text-align: center;
    white-space: normal;
    background: #38383c;
    color: #cfcfd4;
    border-radius: 5px;
  }
  .tile.on {
    background: var(--accent);
    color: #1e1e1e;
  }
</style>
