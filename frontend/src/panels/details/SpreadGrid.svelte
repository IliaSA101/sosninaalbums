<script lang="ts">
  import Spread from '../../editor/spread/Spread.svelte'
  import LowDpiBadge from '../../base/ui/LowDpiBadge.svelte'
  import VigMark from '../vignette/VigMark.svelte'
  import { store } from '../../base/core/store.svelte'
  import { newSheetSize, sheetSize, unitOf } from '../../base/geom/sheet'
  import { modes } from '../../base/prefs/view.svelte'
  import { labelTitle, spreadLabel } from '../timeline/spreadname'

  // Ширина панели 400, поля раздела 16, между плитками 8: три колонки.
  const COLS = 3
  const GAP = 8
  const TILE = Math.floor((400 - 2 * 16 - (COLS - 1) * GAP) / COLS)
  // Внутренний отступ плитки: content-visibility обрезает всё за её краем, рамка должна влезть.
  const PAD = 4
  const MINI = TILE - 2 * PAD
  const s = $derived(store.view.settings)
  // Все плитки одного размера: берём размер листа у первого (Go: sv.w × sv.h); альбом пуст — по настройкам.
  const sheet = $derived(store.view.spreads[0] ? sheetSize(store.view.spreads[0]) : newSheetSize(s))
  const unit = $derived(unitOf(s))
  const tileH = $derived((MINI * sheet.h) / sheet.w)

  let box = $state<HTMLDivElement>()

  // Текущая плитка остаётся видимой в панели. Прокручиваем только вертикально
  // и только свой контейнер: scrollIntoView сдвинул бы и панель с overflow: hidden.
  $effect(() => {
    const i = store.current
    if (!modes.details || !modes.sections.grid) return
    const el = box?.querySelector<HTMLElement>(`[data-tile="${i}"]`)
    const sc = box?.closest<HTMLElement>('.scroll')
    if (!el || !sc) return
    const a = sc.getBoundingClientRect()
    const b = el.getBoundingClientRect()
    if (b.top < a.top) sc.scrollBy({ top: b.top - a.top - 8, behavior: 'smooth' })
    else if (b.bottom > a.bottom) sc.scrollBy({ top: b.bottom - a.bottom + 8, behavior: 'smooth' })
  })
</script>

{#if store.view.spreads.length}
  <div
    class="grid"
    data-guide="spread-grid"
    bind:this={box}
    style:--tile="{TILE}px"
    style:--gap="{GAP}px"
    style:--pad="{PAD}px"
    style:--tile-h="{tileH}px"
  >
    {#each store.view.spreads as sp, i (sp.id)}
      <div
        class="tile"
        class:current={i === store.current}
        data-tile={i}
        role="button"
        tabindex="-1"
        title={labelTitle(i, sp.name) ?? `${unit.cap} ${i + 1}`}
        onclick={() => store.go(i)}
        onkeydown={() => {}}
      >
        <div class="frame">
          <Spread spread={sp} settings={s} width={MINI} mini />
        </div>
        <LowDpiBadge count={sp.lowDPI} min={store.view.minDPI} />
        <VigMark kind={sp.kind} />
        <div class="num">{spreadLabel(i, sp.name)}</div>
      </div>
    {/each}
  </div>
{:else}
  <div class="empty">{unit.none}</div>
{/if}

<style>
  .grid {
    display: grid;
    grid-template-columns: repeat(3, var(--tile));
    gap: 4px var(--gap);
  }
  .tile {
    position: relative;
    padding: var(--pad);
    cursor: pointer;
    /* Большие альбомы: плитки вне вида не рисуются */
    content-visibility: auto;
    contain-intrinsic-size: auto var(--tile) auto calc(var(--tile-h) + 2 * var(--pad) + 18px);
  }
  .frame {
    pointer-events: none;
    outline: 2px solid transparent;
    outline-offset: 1px;
    transition: outline-color 0.15s;
  }
  .tile:hover .frame {
    outline-color: #5a5a60;
  }
  .tile.current .frame {
    outline-color: var(--accent);
  }
  .num {
    text-align: center;
    font-size: 11px;
    color: var(--muted);
    margin-top: 3px;
    /* Длинное название — многоточие, плитку не растягивает */
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .tile.current .num {
    color: var(--accent);
    font-weight: 600;
  }
  .empty {
    color: var(--muted);
    font-size: 13px;
  }
</style>
