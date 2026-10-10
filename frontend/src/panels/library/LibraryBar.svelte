<script lang="ts">
  import { SvelteMap } from 'svelte/reactivity'
  import { store } from '../../base/core/store.svelte'
  import { scanText, scanFraction } from './scan'
  import { fitGroups, type RowGroup } from '../../base/ui/fitrow'
  import MarkBar from '../marks/MarkBar.svelte'
  import MarkFilter from '../marks/MarkFilter.svelte'
  import FolderMenu from './FolderMenu.svelte'
  import MoreMenu from './MoreMenu.svelte'

  // Верхняя полоса ленты фото (спека 024): всегда одна строка. Что не поместилось — уходит в «⋯» по приоритету
  // (prio: меньше — раньше; «Автосборка» не уходит никогда). «Не в альбоме» = не в местах и не на фоне.
  const frac = $derived(scanFraction(store.scan))
  const loading = $derived(store.thumbs.total > 0 && store.thumbs.done < store.thumbs.total)

  // GAP и PAD уходят в CSS полосы через style: (одна правда с расчётом fitGroups)
  const GAP = 8
  const MORE_W = 30
  const PAD = 24 // левый + правый padding полосы
  const SPACER = 2 * GAP // зазоры вокруг растяжки между левой и правой частью

  // Порядок слева направо: до растяжки и после неё.
  const LEFT = ['folders', 'auto', 'per', 'fill']
  const RIGHT = ['status', 'marks', 'filter', 'size']
  const PRIO: Record<string, number> = { folders: 60, auto: Infinity, per: 30, fill: 40, status: 20, marks: 70, filter: 50, size: 10 }

  // Какие группы есть сейчас (пустые не занимают места).
  const present = $derived(
    [...LEFT, ...RIGHT].filter((id) => {
      if (id === 'fill') return store.view.emptyPlaces > 0
      if (id === 'status') return store.scan.active || loading
      if (id === 'marks') return store.selectedIds.length > 0
      return true
    }),
  )

  // Ширины групп, измеренные в строке; ушедшая из строки группа хранит прежнюю ширину.
  const widths = new SvelteMap<string, number>()
  function measure(node: HTMLElement, id: string) {
    const ro = new ResizeObserver(() => {
      const w = Math.ceil(node.getBoundingClientRect().width)
      if (w > 0 && widths.get(id) !== w) widths.set(id, w)
    })
    ro.observe(node)
    return { destroy: () => ro.disconnect() }
  }

  let barW = $state(0)
  const hidden = $derived.by(() => {
    if (barW <= 0) return new Set<string>() // ширина полосы ещё неизвестна — показываем всё
    const groups: RowGroup[] = present.map((id) => ({ id, w: widths.get(id), prio: PRIO[id] }))
    return fitGroups(groups, barW - PAD - SPACER, GAP, MORE_W)
  })
  const inMore = $derived(present.filter((id) => hidden.has(id)))
  const shown = (ids: string[]) => ids.filter((id) => present.includes(id) && !hidden.has(id))

  // Фокус остаётся где был: иначе Enter (хоткей «добавить выделенные») нажмёт кнопку повторно.
  const keep = (e: MouseEvent) => e.preventDefault()
</script>

{#snippet grp(id: string)}
  {#if id === 'folders'}
    <FolderMenu />
  {:else if id === 'auto'}
    <button
      class="primary"
      disabled={store.busy || store.photos.length === 0}
      title="Разложить выделенные фото (или все, что ещё не в альбоме) по новым разворотам. Без выделения отклонённые пропускаются"
      onmousedown={keep}
      onclick={() => store.autoBuild()}
    >
      {store.busy ? 'Собираю…' : 'Автосборка'}
    </button>
  {:else if id === 'per'}
    <select
      bind:value={store.perSpread}
      title="Сколько фото в среднем на разворот при автосборке"
      aria-label="Сколько фото в среднем на разворот при автосборке"
      onchange={(e) => e.currentTarget.blur()}
    >
      {#each [2, 3, 4, 5, 6] as n (n)}<option value={n}>по {n}</option>{/each}
    </select>
  {:else if id === 'fill'}
    <button
      disabled={store.photos.length === 0}
      aria-label="Заполнить пустые места: {store.view.emptyPlaces}"
      title="Заполнить пустые места ({store.view
        .emptyPlaces}): положить выделенные фото (или все, что ещё не в альбоме) в пустые места разворотов-сеток, начиная с этого разворота. Без выделения отклонённые пропускаются"
      onmousedown={keep}
      onclick={() => store.fillPlaces()}
    >
      Заполнить {store.view.emptyPlaces}
    </button>
  {:else if id === 'status'}
    {#if store.scan.active}
      <span class="small scan" role="status">{scanText(store.scan)}</span>
    {:else if loading}
      <span class="small">Готовлю миниатюры {store.thumbs.done} / {store.thumbs.total}</span>
    {/if}
  {:else if id === 'marks'}
    <MarkBar />
  {:else if id === 'filter'}
    <MarkFilter />
  {:else if id === 'size'}
    <input
      class="size"
      type="range"
      min="70"
      max="220"
      bind:value={store.thumbSize}
      title="Размер миниатюр"
      aria-label="Размер миниатюр"
      onchange={(e) => e.currentTarget.blur()}
    />
  {/if}
{/snippet}

<div class="bar" data-bar="library" bind:clientWidth={barW} style:gap="{GAP}px" style:padding-inline="{PAD / 2}px">
  {#each shown(LEFT) as id (id)}
    <span class="g" use:measure={id}>{@render grp(id)}</span>
  {/each}

  <div class="grow"></div>

  {#each shown(RIGHT) as id (id)}
    <span class="g" use:measure={id}>{@render grp(id)}</span>
  {/each}
  {#if inMore.length > 0}
    <MoreMenu ids={inMore} item={grp} />
  {/if}
  {#if store.scan.active}
    <div class="progress" class:indeterminate={frac === null}>
      <div class="fill" style:width="{(frac ?? 0) * 100}%"></div>
    </div>
  {/if}
</div>

<style>
  /* Одна строка при любой ширине: непомещающееся уходит в «⋯», переноса нет. Кадр до измерения обрезается */
  .bar {
    position: relative;
    display: flex;
    align-items: center;
    padding-block: 8px;
    border-bottom: 1px solid var(--line);
    flex-wrap: nowrap;
    overflow: hidden;
    white-space: nowrap;
  }
  /* flex: none — иначе строка сожмёт группу, и измеренная ширина станет неверной */
  .g {
    display: inline-flex;
    align-items: center;
    gap: 8px;
    flex: none;
  }
  .grow {
    flex: 1;
    min-width: 0;
  }
  .size {
    width: 72px;
  }
  /* Тонкая полоса чтения папки по нижнему краю панели; при обходе итог неизвестен */
  .progress {
    position: absolute;
    left: 0;
    right: 0;
    bottom: 0;
    height: 2px;
    overflow: hidden;
    pointer-events: none;
  }
  .fill {
    height: 100%;
    background: var(--accent);
    transition: width 0.2s;
  }
  .progress.indeterminate .fill {
    width: 30%;
    animation: slide 1.2s ease-in-out infinite;
  }
  @keyframes slide {
    from {
      transform: translateX(-100%);
    }
    to {
      transform: translateX(340%);
    }
  }
  .scan {
    color: var(--text);
  }
  .small {
    font-size: 12px;
    color: var(--muted);
    display: inline-flex;
    gap: 4px;
    align-items: center;
  }
</style>
