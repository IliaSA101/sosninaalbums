<script lang="ts">
  import { tick, type Snippet } from 'svelte'
  import { prefs } from '../../base/prefs/prefs.svelte'
  import { LEVEL_TITLE, minLevel } from '../../base/prefs/levels'
  import { articleById, GUIDE_GROUPS, placesOf, type Article, type GroupId } from '../guidetext/articles'
  import { navKey } from '../hotkeys/keylist'
  import { guide } from './guidestate.svelte'
  import { openGroups, orderOf, sectionMark, stepSel } from './guidelist'
  import { learned } from './learned.svelte'

  // Список статей окна справочника (спека 136, design §1). Образец — список «Горячих клавиш» (KeyList): строки
  // `button.item` с roving tabindex (в Tab входит только выбранная), ↑/↓/Home/End меняют выбор, Enter — «Показать».
  // Разделы сворачиваются щелчком по липкому заголовку (как группы карточки); раздел выбранной статьи открыт всегда.
  // fold = false (поиск, фильтр строки уровней): список плоский или все разделы открыты, заголовки не нажимаются.
  let {
    sections,
    curId,
    fold,
    onsearch,
    oncard,
    empty,
  }: {
    sections: { id: string; title: string; items: Article[] }[]
    curId: string | undefined
    fold: boolean
    onsearch: () => void // ↑ с первой статьи — назад в поиск
    oncard: () => void // Enter у статьи без места — фокус в карточку
    empty?: Snippet
  } = $props()

  const order = $derived(orderOf(sections))
  const open = $derived(openGroups(guide.opened, curId ? articleById(curId)?.group : undefined))
  const isOpen = (id: string) => !fold || open.has(id as GroupId)
  const groupTitle = (a: Article) => GUIDE_GROUPS.find((g) => g.id === a.group)?.title ?? ''
  // Тег уровня у скрытой функции: с какого уровня она есть; в «Своём» — что её нет в наборе.
  const tag = (a: Article): string =>
    prefs.can(a.feature) ? '' : prefs.level === 'custom' ? 'Не в «Своём»' : LEVEL_TITLE[minLevel(a.feature)]

  let box = $state<HTMLElement>()
  const rowEl = (id: string) => box?.querySelector<HTMLElement>(`[data-id="${id}"]`) ?? null

  // Выбранная строка — в поле зрения. Эффект читает список синхронно: монтаж (повторный F1) запускает его так же, как смена
  // выбора; раньше он следил только за выбором и при тех же запросе и выборе список вставал в начало (136, замер критика A).
  // Список разделов читаем (смена запроса), состояние свёрнутости — нет: щелчок по заголовку не должен дёргать прокрутку.
  $effect(() => {
    void sections
    const id = curId
    if (box && id) rowEl(id)?.scrollIntoView({ block: 'nearest' })
  })

  async function go(id: string, top = false) {
    guide.sel = id
    await tick() // раздел цели мог быть свёрнут: он откроется, когда станет разделом выбора
    rowEl(id)?.focus({ preventScroll: true })
    if (top && box) box.scrollTop = 0
  }

  // Из поля поиска ↓ входит в список: выбранная строка, а без выбора — первая.
  export function focusSel() {
    const id = curId ?? order[0]
    if (id) void go(id)
  }

  const hasPlace = (id: string): boolean => {
    const a = articleById(id)
    return !!a && placesOf(a).length > 0
  }
  function enter(id: string) {
    if (hasPlace(id)) void guide.show(id)
    else oncard()
  }

  function rowKey(e: KeyboardEvent, id: string) {
    if (e.key === 'Enter' && !e.ctrlKey && !e.altKey && !e.shiftKey && !e.metaKey) {
      e.preventDefault()
      enter(id)
      return
    }
    const k = navKey(e)
    if (!k) return
    e.preventDefault() // стрелка не должна прокручивать список сама
    if (k === 'ArrowUp' && id === order[0]) return onsearch()
    const to = stepSel(order, id, k)
    if (to) void go(to, k === 'Home')
  }

  function toggle(id: string) {
    if (guide.opened.has(id as GroupId)) guide.opened.delete(id as GroupId)
    else guide.opened.add(id as GroupId)
  }
</script>

<div class="list" bind:this={box} role="listbox" aria-label="Функции">
  {#each sections as s (s.id)}
    {@const shown = isOpen(s.id)}
    <section data-group={s.id}>
      {#if s.title}
        {#if fold}
          <button class="head" tabindex="-1" aria-expanded={shown} onclick={() => toggle(s.id)}>
            <svg class="chev" class:open={shown} viewBox="0 0 12 12" aria-hidden="true"><path d="M4 2.5 7.5 6 4 9.5" /></svg>
            <span class="ttl">{s.title}</span>
            <span class="n">{sectionMark(s.items, learned.ids)}</span>
          </button>
        {:else}
          <h3 class="head">
            <span class="ttl">{s.title}</span>
            <span class="n">{sectionMark(s.items, learned.ids)}</span>
          </h3>
        {/if}
      {/if}
      {#if shown}
        {#each s.items as a (a.id)}
          {@const on = a.id === curId}
          <button
            class="item"
            class:on
            role="option"
            aria-selected={on}
            tabindex={on ? 0 : -1}
            data-id={a.id}
            onclick={() => (guide.sel = a.id)}
            ondblclick={() => enter(a.id)}
            onkeydown={(e) => rowKey(e, a.id)}
          >
            <span class="ok" aria-hidden="true">{learned.has(a.id) ? '✓' : ''}</span>
            <span class="tx">
              <span class="t">{a.title}</span>
              {#if s.id === 'found'}<span class="g">{groupTitle(a)}</span>{/if}
              <!-- Скрытый текст вместо aria-label: имя строки = видимый текст + «, изучено» (aria-label затёр бы раздел в выдаче) -->
              {#if learned.has(a.id)}<span class="vh">, изучено</span>{/if}
            </span>
            {#if tag(a)}<span class="kt">{tag(a)}</span>{/if}
          </button>
        {/each}
      {/if}
    </section>
  {:else}
    {@render empty?.()}
  {/each}
</div>

<style>
  .list {
    min-height: 0;
    overflow-y: auto;
    scrollbar-width: thin;
  }
  /* Липкий заголовок раздела: шеврон, название, число статей справа (как заголовок группы карточки) */
  .head {
    all: unset;
    box-sizing: border-box;
    position: sticky;
    top: 0;
    z-index: 1;
    width: 100%;
    height: 24px;
    display: flex;
    align-items: center;
    gap: 6px;
    padding: 0 8px;
    background: var(--panel);
    font-size: 11px;
    font-weight: 600;
    color: var(--muted);
    text-transform: uppercase;
    letter-spacing: 0.04em;
  }
  button.head {
    cursor: pointer;
  }
  button.head:hover .ttl {
    color: var(--text);
  }
  button.head:focus-visible {
    outline: 2px solid var(--accent);
    outline-offset: -2px;
  }
  .chev {
    width: 12px;
    height: 12px;
    flex: none;
    fill: none;
    stroke: var(--muted);
    stroke-width: 1.6;
    stroke-linecap: round;
    stroke-linejoin: round;
    transition: transform var(--move);
  }
  .chev.open {
    transform: rotate(90deg);
  }
  .ttl {
    flex: 1;
    min-width: 0;
  }
  .n {
    font-weight: 400;
    letter-spacing: 0;
  }
  .item {
    width: 100%;
    height: auto;
    min-height: 28px;
    padding: 4px 8px;
    /* Выбранная строка не уходит под липкий заголовок */
    scroll-margin-top: 26px;
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 8px;
    text-align: left;
    background: transparent;
    white-space: normal;
    border-radius: 4px;
  }
  .item:hover {
    background: #38383c;
  }
  .item.on {
    background: #46464b;
  }
  .item:focus-visible {
    outline: 2px solid var(--accent);
    outline-offset: -2px;
  }
  /* Тихая ✓ у изученной; место под неё есть у каждой строки, чтобы названия не прыгали */
  .ok {
    flex: none;
    width: 12px;
    font-size: 11px;
    color: var(--muted);
  }
  .tx {
    position: relative; /* опора для скрытого текста .vh */
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
    align-items: flex-start;
  }
  .vh {
    position: absolute;
    width: 1px;
    height: 1px;
    overflow: hidden;
    clip-path: inset(50%);
    white-space: nowrap;
  }
  .g {
    font-size: 11px;
    color: var(--muted);
  }
  /* Тег уровня справа, как сочетание в строке «Горячих клавиш» */
  .kt {
    flex: none;
    font-size: 11px;
    color: var(--muted);
  }
</style>
