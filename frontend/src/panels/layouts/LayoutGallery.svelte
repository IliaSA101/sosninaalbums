<script lang="ts">
  import { slide } from 'svelte/transition'
  import { store } from '../../base/core/store.svelte'
  import { gallery } from './gallery.svelte'
  import { plural } from '../../base/core/plural'
  import { prefs } from '../../base/prefs/prefs.svelte'
  import Icon from '../../base/ui/Icon.svelte'
  import GridGallery from './GridGallery.svelte'
  import type { Choice, ChoiceRows } from '../../base/core/types'

  const H = 64 // высота карточки
  const s = $derived(store.view.settings)
  const sw = $derived(s.pageW * 2)
  const cardW = $derived((H * sw) / s.pageH)

  // Пока галерея уезжает после закрытия, gallery.cur уже null — держим последнюю.
  let last = $state(gallery.cur!)
  $effect.pre(() => {
    if (gallery.cur) last = gallery.cur
  })
  const g = $derived(gallery.cur ?? last)
  const sp = $derived(store.view.spreads.find((x) => x.id === g.spread))
  const num = $derived(store.view.spreads.findIndex((x) => x.id === g.spread) + 1)
  // Варианты показываем, только если они посчитаны для этого разворота.
  const c = $derived(gallery.choices?.spread === g.spread ? gallery.choices : null)
  const rows = $derived<ChoiceRows | null>(c ? (g.scope === 'spread' ? c.whole : c[g.scope]) : null)
  const inOrder = $derived(rows?.inOrder ?? [])
  const rearranged = $derived(rows?.rearranged ?? [])
  const k = $derived(inOrder.length + rearranged.length)
  const page = $derived(g.scope === 'spread' ? null : g.scope)

  const title = $derived(
    page
      ? `Разворот ${num}, ${page === 'left' ? 'левая' : 'правая'} страница: ${k} ${plural(k, ['раскладка', 'раскладки', 'раскладок'])}`
      : `Разворот ${num}: ${k} ${plural(k, ['подходящая раскладка', 'подходящие раскладки', 'подходящих раскладок'])}`,
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

  const lines = $derived([
    { label: 'С перестановкой порядка', list: rearranged, empty: why },
    { label: 'В порядке ленты', list: inOrder, empty: 'Нет раскладок' },
  ])

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

  function pick(ch: Choice) {
    if (ch.key === sp?.layout) gallery.close()
    else gallery.setLayout(ch.key)
  }
</script>

<section class="gallery" bind:this={box} transition:slide={{ duration: 180 }}>
  <header>
    <!-- Вкладки — только если есть что выбирать: без «Вариантов» остаются шаблоны, без «Шаблонов» — варианты -->
    {#if prefs.can('templates') && prefs.can('variants')}
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
    {#if g.tab === 'layouts'}
      <span class="title">{c ? title : `Разворот ${num}: подбираем раскладки…`}</span>
    {:else}
      <span class="title">Разворот {num}</span>
    {/if}
    <button class="close" onclick={() => gallery.close()} title="Закрыть (Esc)" aria-label="Закрыть галерею раскладок"
      ><Icon name="x" /></button
    >
  </header>

  {#if g.tab === 'grids'}
    <GridGallery {g} />
  {/if}
  {#each g.tab === 'grids' ? [] : lines as line (line.label)}
    <div class="line">
      <div class="label">{line.label}</div>
      <div class="track">
        {#if c && line.list.length === 0}
          <div class="empty">{line.empty}</div>
        {:else}
          <button class="nav" onclick={(e) => scrollBy(e, -1)} title="Прокрутить влево" aria-label="Прокрутить влево"
            ><Icon name="chevron-left" size={18} /></button
          >
          <div class="scroll" onwheel={wheel}>
            {#each line.list as ch (ch.key)}
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
                    style:left="{(r.x / sw) * 100}%"
                    style:top="{(r.y / s.pageH) * 100}%"
                    style:width="{(r.w / sw) * 100}%"
                    style:height="{(r.h / s.pageH) * 100}%"
                  ></span>
                {/each}
                <span class="spine"></span>
                {#if page}
                  <span class="other" style:left={page === 'left' ? '50%' : '0'}></span>
                {/if}
              </button>
            {/each}
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
