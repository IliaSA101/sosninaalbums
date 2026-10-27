<script lang="ts">
  import { SvelteMap } from 'svelte/reactivity'
  import { store } from '../../base/core/store.svelte'
  import { modes, THUMB_MIN, THUMB_MAX } from '../../base/prefs/view.svelte'
  import { unitOf } from '../../base/geom/sheet'
  import { scanText, scanFraction } from '../folders/scan'
  import { fitGroups, type RowGroup } from '../../base/ui/fitrow'
  import MarkBar from '../marks/MarkBar.svelte'
  import MarkFilter from '../marks/MarkFilter.svelte'
  import NameSearch from '../marks/NameSearch.svelte'
  import { nameSearch } from '../marks/namefind.svelte'
  import FolderMenu from '../folders/FolderMenu.svelte'
  import { hasSources } from '../folders/folders'
  import BinMenu from '../bins/BinMenu.svelte'
  import { binHost, moreBinDrop } from '../bins/bins'
  import { binsel } from '../bins/binsel.svelte'
  import { gallery } from '../gallery/gallery.svelte'
  import { drag } from '../../base/drag/drag.svelte'
  import { prefs } from '../../base/prefs/prefs.svelte'
  import MoreMenu from './MoreMenu.svelte'
  import { fillCount, fillDigits, fillTitle } from './freephotos'

  // Верхняя полоса ленты фото (спека 024): всегда одна строка. Что не поместилось — уходит в «⋯» по приоритету
  // (prio: меньше — раньше; «Автосборка» не уходит никогда). «Не в альбоме» = не в местах и не на фоне.
  const frac = $derived(scanFraction(store.scan))
  const unit = $derived(unitOf(store.view.settings))
  // 132: число на кнопке — места «отсюда» (с обложки — обложка и развороты); видна она по пустым местам всего альбома,
  // иначе мигала бы при листании, а узкая полоса перекидывала бы группы в «⋯» (0.27-132-16).
  const fillN = $derived(fillCount(store.view, store.onCover, store.current))
  // Ширина числа резервируется по разрядам пустых мест альбома (0.27-132-22): при листании «12» → «0» кнопка не
  // меняет ширину, и у границы полосы группы не прыгают в «⋯» и обратно.
  const fillW = $derived(fillDigits(store.view.emptyPlaces, fillN))
  const loading = $derived(store.thumbs.total > 0 && store.thumbs.done < store.thumbs.total)

  // GAP и PAD уходят в CSS полосы через style: (одна правда с расчётом fitGroups)
  const GAP = 8
  const MORE_W = 30
  const PAD = 24 // левый + правый padding полосы
  const SPACER = 2 * GAP // зазоры вокруг растяжки между левой и правой частью

  // Порядок слева направо: до растяжки и после неё.
  const LEFT = ['folders', 'bins', 'auto', 'fill']
  const RIGHT = ['status', 'marks', 'search', 'filter', 'size']
  const PRIO: Record<string, number> = {
    folders: 60,
    bins: 55,
    auto: Infinity,
    fill: 40,
    status: 20,
    marks: 70,
    search: 45,
    filter: 50,
    size: 10,
  }
  // Открытое или непустое поле поиска не уходит в «⋯» (иначе оно исчезло бы под курсором, а запрос продолжал бы сужать ленту); пустое уходит раньше «Все N ▾»
  const prio = (id: string) => (id === 'search' && nameSearch.held ? Infinity : PRIO[id])

  // Какие группы есть сейчас (пустые не занимают места).
  const present = $derived(
    [...LEFT, ...RIGHT].filter((id) => {
      if (id === 'fill') return store.view.emptyPlaces > 0
      if (id === 'bins') return prefs.can('bins') && (hasSources(store.view) || (store.view.bins ?? []).length > 0)
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
    const groups: RowGroup[] = present.map((id) => ({ id, w: widths.get(id), prio: prio(id) }))
    return fitGroups(groups, barW - PAD - SPACER, GAP, MORE_W)
  })
  const inMore = $derived(present.filter((id) => hidden.has(id)))

  // Окно «Подборки» просят открыть клавишей (B, Shift+B), а кнопка ушла в «⋯»: открываем «⋯» — окно встанет там.
  // Окну негде быть (нет группы, лента свёрнута) — флаг сбрасываем, иначе окно всплыло бы при следующем открытии.
  let moreOpen = $state(false)
  // Хост кнопки нужен и закрытому окну: тост B без подборок называет путь через «⋯».
  $effect(() => {
    binsel.host = binHost(present.includes('bins'), hidden.has('bins'), modes.big || !!gallery.cur)
  })
  $effect(() => {
    if (!binsel.open) return
    const host = binsel.host
    if (host === 'more') moreOpen = true
    else if (host === 'none') binsel.close()
  })
  // Бросок фото на подборку, пока её кнопка в «⋯»: «⋯» — цель (083-29).
  const moreDrop = $derived(
    moreBinDrop(
      binHost(present.includes('bins'), hidden.has('bins'), modes.big || !!gallery.cur),
      drag.active && drag.payload?.kind === 'photos' && !drag.payload.fromSpread,
      drag.over === 'bin',
      binsel.targetName,
    ),
  )
  // Якоря справочника (033) групп, что ушли в «⋯»: кнопка «⋯» мигает вместо них.
  const GUIDE_OF: Record<string, string> = {
    folders: 'folders',
    bins: 'bins',
    fill: 'fill-empty',
    marks: 'mark-bar',
    search: 'name-search',
    filter: 'mark-filter',
    size: 'thumb-size',
  }
  const hold = $derived(
    inMore
      .map((id) => GUIDE_OF[id])
      .filter(Boolean)
      .join(' '),
  )
  const shown = (ids: string[]) => ids.filter((id) => present.includes(id) && !hidden.has(id))

  // Фокус остаётся где был: иначе Enter (хоткей «добавить выделенные») нажмёт кнопку повторно.
  const keep = (e: MouseEvent) => e.preventDefault()
</script>

{#snippet grp(id: string)}
  {#if id === 'folders'}
    <FolderMenu />
  {:else if id === 'bins'}
    <BinMenu />
  {:else if id === 'auto'}
    <button
      class="primary"
      data-guide="autobuild"
      disabled={store.busy || store.photos.length === 0}
      title="Собрать из выделенных фото (или из всех, что ещё не в альбоме) заданное число {unit
        .plural[2]}. Без выделения отклонённые пропускаются"
      onmousedown={keep}
      onclick={() => store.openAuto()}
    >
      {store.busy ? 'Собираю…' : 'Автосборка'}
    </button>
  {:else if id === 'fill'}
    <button
      data-guide="fill-empty"
      disabled={store.photos.length === 0}
      aria-label="Заполнить пустые места: {fillN}"
      title={fillTitle(store.onCover, fillN, unit)}
      onmousedown={keep}
      onclick={() => store.fillPlaces()}
    >
      Заполнить места: <span class="num" style:min-width="{fillW}ch">{fillN}</span>
    </button>
  {:else if id === 'status'}
    {#if store.scan.active}
      <span class="small scan" role="status">{scanText(store.scan)}</span>
    {:else if loading}
      <span class="small">Готовлю миниатюры {store.thumbs.done} / {store.thumbs.total}</span>
    {/if}
  {:else if id === 'marks'}
    <MarkBar />
  {:else if id === 'search'}
    <NameSearch />
  {:else if id === 'filter'}
    <MarkFilter />
  {:else if id === 'size'}
    <input
      class="size"
      data-guide="thumb-size"
      type="range"
      min={THUMB_MIN}
      max={THUMB_MAX}
      value={modes.thumbSize}
      oninput={(e) => modes.setThumb(Number(e.currentTarget.value))}
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
    <MoreMenu ids={inMore} item={grp} {hold} bind:open={moreOpen} dropBin={moreDrop.drop} dropText={moreDrop.text} />
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
  /* Число кнопки «Заполнить места»: моноширинные цифры и запас по разрядам — ширина не зависит от значения */
  .num {
    display: inline-block;
    text-align: left;
    font-variant-numeric: tabular-nums;
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
