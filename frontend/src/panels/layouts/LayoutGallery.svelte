<script lang="ts">
  import { tick } from 'svelte'
  import { slide } from 'svelte/transition'
  import { store } from '../../base/core/store.svelte'
  import { gallery } from './gallery.svelte'
  import { plural } from '../../base/core/plural'
  import { prefs } from '../../base/prefs/prefs.svelte'
  import Icon from '../../base/ui/Icon.svelte'
  import { newSheetSize, sheetSize, unitOf } from '../../base/geom/sheet'
  import GridGallery from './GridGallery.svelte'
  import VigGallery from '../vignette/VigGallery.svelte'
  import { isVignette } from '../vignette/vignette'
  import type { Choice, ChoiceRows } from '../../base/core/types'
  import { activeFamily, familiesIn, galleryCount, galleryLines, shownRow, twinsLabel } from './variants'

  const H = 64 // высота карточки
  const s = $derived(store.view.settings)

  // Пока галерея уезжает после закрытия, gallery.cur уже null — держим последнюю.
  let last = $state(gallery.cur!)
  $effect.pre(() => {
    if (gallery.cur) last = gallery.cur
  })
  const g = $derived(gallery.cur ?? last)
  const sp = $derived(store.view.spreads.find((x) => x.id === g.spread))
  // Карточка — пропорции листа (Go: sv.w × sv.h); нет листа — по настройкам.
  const sheet = $derived(sp ? sheetSize(sp) : newSheetSize(s))
  const cardW = $derived((H * sheet.w) / sheet.h)
  const unit = $derived(unitOf(s))
  // Виньетка (решение 80): вместо раскладок «Авто» и шаблонов сетки — шаблоны виньетки.
  const vig = $derived(isVignette(sp) ? sp : null)
  const num = $derived(store.view.spreads.findIndex((x) => x.id === g.spread) + 1)
  // Варианты показываем, только если они посчитаны для этого разворота.
  const c = $derived(gallery.choices?.spread === g.spread ? gallery.choices : null)
  const rows = $derived<ChoiceRows | null>(c ? (g.scope === 'spread' ? c.whole : c[g.scope]) : null)
  const inOrder = $derived(rows?.inOrder ?? [])
  const rearranged = $derived(rows?.rearranged ?? [])
  // 041: «Другие варианты» — третий ряд той же разметки, только у разворота целиком (у страницы Go его не считает).
  const extra = $derived(g.scope === 'spread' ? (rows?.extra ?? []) : [])
  // У разворота — столько же, сколько листает счётчик «Раскладка N / M» (без «С перестановкой»).
  const k = $derived(galleryCount(g.scope === 'spread', { inOrder, rearranged, extra }))
  const page = $derived(g.scope === 'spread' ? null : g.scope)

  const title = $derived(
    page
      ? `${unit.cap} ${num}, ${page === 'left' ? 'левая' : 'правая'} страница: ${k} ${plural(k, ['раскладка', 'раскладки', 'раскладок'])}`
      : `${unit.cap} ${num}: ${k} ${plural(k, ['подходящая раскладка', 'подходящие раскладки', 'подходящих раскладок'])}`,
  )

  // Почему ряд с перестановкой пуст — причину сообщает Go.
  const why = $derived(
    rows?.block === 'same'
      ? 'Перестановка возможна, только если есть и вертикальные, и горизонтальные фото'
      : rows?.block === 'many'
        ? page
          ? 'Перестановка доступна для страниц до 5 фото'
          : 'Перестановка доступна для разворотов до 7 фото'
        : 'Перестановка не даёт лучших раскладок',
  )

  // «Ещё» (027): Go может досчитать ряд только у разворота целиком (17+ фото).
  const canMore = $derived(g.scope === 'spread' && !!rows?.more)
  const base = $derived(galleryLines(g.scope, { inOrder, rearranged, extra }, { why, canMore }))
  // Фильтр семейств — только у разворота и когда есть из чего выбирать; похожие свёрнуты в плитку «Похожие: N».
  const families = $derived(g.scope === 'spread' ? familiesIn([inOrder, rearranged, extra]) : [])
  const fam = $derived(activeFamily(gallery.family, families))
  const lines = $derived(
    base.map((l) => ({ ...l, ...shownRow(l.list, { family: fam, current: sp?.layout, open: gallery.openTwins.includes(l.label) }) })),
  )

  // При открытии (и смене области) ряд прокручен к текущей карточке.
  let box: HTMLElement
  $effect(() => {
    void [g.scope, c?.rev, sp?.layout]
    queueMicrotask(() => {
      for (const el of box?.querySelectorAll<HTMLElement>('.card.current') ?? []) {
        const sc = el.closest<HTMLElement>('.scroll')!
        sc.scrollLeft = el.offsetLeft - (sc.clientWidth - el.offsetWidth) / 2
      }
    })
  })

  function scrollBy(e: MouseEvent, dir: number) {
    const sc = (e.currentTarget as HTMLElement).parentElement!.querySelector<HTMLElement>('.scroll')!
    sc.scrollBy({ left: dir * sc.clientWidth * 0.8, behavior: 'smooth' })
  }

  function wheel(e: WheelEvent) {
    const sc = e.currentTarget as HTMLElement
    if (Math.abs(e.deltaY) > Math.abs(e.deltaX)) sc.scrollLeft += e.deltaY
  }

  // Плитка «Ещё»: ряд дописывается, первая новая карточка встаёт в начало видимой части.
  async function more(e: MouseEvent) {
    // Карточек в DOM столько, сколько показано (похожие свёрнуты), а не весь ряд.
    const have = lines.find((l) => l.list === inOrder)?.shown.length ?? inOrder.length
    const sc = (e.currentTarget as HTMLElement).closest<HTMLElement>('.scroll')!
    await gallery.loadMore()
    await tick()
    const first = sc.querySelectorAll<HTMLElement>('.card:not(.more)')[have]
    if (first) sc.scrollTo({ left: first.offsetLeft - 8, behavior: 'smooth' })
  }

  function pick(ch: Choice) {
    if (ch.key === sp?.layout) gallery.close()
    else gallery.setLayout(ch.key)
  }
</script>

<section class="gallery" bind:this={box} transition:slide={{ duration: 180 }}>
  <header>
    <!-- Вкладки — только если есть что выбирать: без «Вариантов» остаются шаблоны, без «Шаблонов» — варианты -->
    {#if !vig && prefs.can('templates') && prefs.can('variants')}
      <div class="tabs" role="tablist" aria-label="Что показать">
        <!-- На сетке выбор раскладки спросит «Вернуть авто?» -->
        <button
          role="tab"
          aria-selected={g.tab === 'layouts'}
          class:on={g.tab === 'layouts'}
          onclick={() => g.tab !== 'layouts' && gallery.open('spread', 'layouts')}>Варианты</button
        >
        <button
          role="tab"
          aria-selected={g.tab === 'grids'}
          class:on={g.tab === 'grids'}
          onclick={() => g.tab !== 'grids' && gallery.open('spread', 'grids')}>Шаблоны</button
        >
      </div>
    {/if}
    {#if g.tab === 'layouts' && !vig}
      <span class="title">{c ? title : `${unit.cap} ${num}: подбираем раскладки…`}</span>
    {:else}
      <span class="title">{unit.cap} {num}</span>
    {/if}
    <button class="close" onclick={() => gallery.close()} title="Закрыть (Esc)" aria-label="Закрыть галерею раскладок"
      ><Icon name="x" /></button
    >
  </header>

  {#if vig}
    <VigGallery sp={vig} />
  {:else if g.tab === 'grids'}
    <GridGallery {g} />
  {/if}
  <!-- Фильтр семейств: сегменты, как «Область» в галерее страницы -->
  {#if !vig && g.tab === 'layouts' && families.length > 1}
    <div class="bar">
      <div class="seg" role="group" aria-label="Семейство раскладок">
        <button class:on={fam === 'all'} aria-pressed={fam === 'all'} onclick={() => gallery.setFamily('all')}>Все</button>
        {#each families as f (f.id)}
          <button class:on={fam === f.id} aria-pressed={fam === f.id} onclick={() => gallery.setFamily(f.id)}>{f.label}</button>
        {/each}
      </div>
    </div>
  {/if}
  {#each vig || g.tab === 'grids' ? [] : lines as line (line.label)}
    <div class="line">
      <div class="label">{line.label}</div>
      <div class="track">
        {#if c && line.shown.length === 0}
          <div class="empty">{line.list.length > 0 ? 'Нет раскладок этого семейства' : line.empty}</div>
        {:else}
          <button class="nav" onclick={(e) => scrollBy(e, -1)} title="Прокрутить влево" aria-label="Прокрутить влево"
            ><Icon name="chevron-left" size={18} /></button
          >
          <div class="scroll" onwheel={wheel}>
            {#each line.shown as ch (ch.key)}
              <button
                class="card"
                class:current={ch.key === sp?.layout}
                style:width="{cardW}px"
                style:height="{H}px"
                onmouseenter={() => gallery.previewChoice(ch)}
                onmouseleave={() => gallery.previewChoice(null)}
                onclick={() => pick(ch)}
              >
                {#each ch.rects as r, i (i)}
                  <span
                    class="cell"
                    style:left="{(r.x / sheet.w) * 100}%"
                    style:top="{(r.y / sheet.h) * 100}%"
                    style:width="{(r.w / sheet.w) * 100}%"
                    style:height="{(r.h / sheet.h) * 100}%"
                  ></span>
                {/each}
                <span class="spine"></span>
                {#if page}
                  <span class="other" style:left={page === 'left' ? '50%' : '0'}></span>
                {/if}
              </button>
            {/each}
            {#if line.hidden > 0}
              {@const open = gallery.openTwins.includes(line.label)}
              <button
                class="card more"
                style:width="{cardW}px"
                style:height="{H}px"
                aria-expanded={open}
                onclick={() => gallery.toggleTwins(line.label)}
                title="Раскладки, почти не отличающиеся от соседних">{twinsLabel(line.hidden, open)}</button
              >
            {/if}
            {#if line.more}
              <button
                class="card more"
                style:width="{cardW}px"
                style:height="{H}px"
                disabled={gallery.moreBusy}
                onclick={more}
                title="Досчитать ещё 12 раскладок">{gallery.moreBusy ? 'Считаю…' : 'Ещё'}</button
              >
            {/if}
          </div>
          <button class="nav" onclick={(e) => scrollBy(e, 1)} title="Прокрутить вправо" aria-label="Прокрутить вправо"
            ><Icon name="chevron-right" size={18} /></button
          >
        {/if}
      </div>
    </div>
  {/each}
</section>

<style>
  .gallery {
    flex: none;
    background: var(--panel2);
    border-top: 1px solid var(--line);
    border-bottom: 1px solid var(--line);
    padding: 6px 12px 10px;
    overflow: hidden;
  }
  header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    height: 28px;
    font-size: 13px;
  }
  .title {
    flex: 1;
    margin-left: 12px;
    color: var(--text);
  }
  .tabs {
    display: flex;
    gap: 1px;
    padding: 2px;
    background: #1b1b1d;
    border-radius: 7px;
  }
  .tabs button {
    height: 24px;
    padding: 0 12px;
    font-size: 12px;
    background: transparent;
    border-radius: 5px;
  }
  .tabs button.on {
    background: var(--accent);
    color: #1e1e1e;
    font-weight: 600;
  }
  .close {
    width: 28px;
    height: 28px;
    padding: 0;
    display: grid;
    place-items: center;
    background: transparent;
  }
  /* Фильтр семейств — как строка выбора области в GridGallery */
  .bar {
    display: flex;
    align-items: center;
    gap: 14px;
    height: 32px;
    margin-top: 4px;
  }
  .seg {
    display: flex;
    gap: 1px;
    padding: 2px;
    background: #1b1b1d;
    border-radius: 7px;
  }
  .seg button {
    height: 24px;
    min-width: 28px;
    padding: 0 8px;
    font-size: 12px;
    background: transparent;
    border-radius: 5px;
  }
  .seg button.on {
    background: var(--accent);
    color: #1e1e1e;
    font-weight: 600;
  }
  .line {
    display: flex;
    align-items: center;
    gap: 8px;
    margin-top: 6px;
  }
  .label {
    flex: none;
    width: 170px;
    font-size: 12px;
    color: var(--muted);
  }
  .track {
    flex: 1;
    min-width: 0;
    display: flex;
    align-items: center;
    gap: 4px;
    height: 72px; /* карточка + рамка текущей */
  }
  .scroll {
    flex: 1;
    min-width: 0;
    display: flex;
    gap: 10px;
    padding: 4px;
    overflow-x: auto;
    overflow-y: hidden;
    scrollbar-width: none;
  }
  .nav {
    flex: none;
    width: 24px;
    height: 64px;
    padding: 0;
    display: grid;
    place-items: center;
    background: transparent;
  }
  .card {
    all: unset;
    flex: none;
    position: relative;
    background: #fff;
    cursor: pointer;
    outline: 2px solid transparent;
    outline-offset: 2px;
    transition: outline-color 0.15s;
  }
  .card:hover {
    outline-color: #8a8a90;
  }
  .card.current {
    outline-color: var(--accent);
  }
  /* Плитка «Ещё» — не раскладка: пунктир на фоне панели */
  .card.more {
    display: grid;
    place-items: center;
    background: transparent;
    outline: 1px dashed var(--line);
    outline-offset: 0;
    color: var(--muted);
    font-size: 13px;
  }
  .card.more:hover:not(:disabled) {
    outline-color: #8a8a90;
    color: var(--text);
  }
  .card.more:disabled {
    cursor: progress;
  }
  .cell {
    position: absolute;
    background: #d8d8d8;
    box-shadow: inset 0 0 0 1px #a8a8a8;
  }
  .spine {
    position: absolute;
    left: 50%;
    top: 0;
    bottom: 0;
    width: 1px;
    background: rgba(0, 0, 0, 0.25);
  }
  /* Галерея страницы: чужая половина не участвует */
  .other {
    position: absolute;
    top: 0;
    bottom: 0;
    width: 50%;
    background: #555;
  }
  .empty {
    color: var(--muted);
    font-size: 13px;
    padding-left: 28px;
  }
</style>
