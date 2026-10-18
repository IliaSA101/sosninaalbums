<script lang="ts">
  import { store } from '../../base/core/store.svelte'
  import { modes } from '../../base/prefs/view.svelte'
  import { gallery } from '../gallery/gallery.svelte'
  import { prefs } from '../../base/prefs/prefs.svelte'
  import Icon from '../../base/ui/Icon.svelte'
  import Popover from '../../base/ui/Popover.svelte'
  import { libfilter } from './libfilter.svelte'
  import { LIB_SORTS } from '../library/sortphotos'
  import { filterLabel, LABEL_VIEW } from './markbar'
  import type { FlagFilter } from './marks'

  // Фильтр ленты (спеки 004 п. 14–15, 024): кнопка со счётом и окошко. «Все фото / Не в альбоме» — всегда,
  // условия отметок — по уровню. M — сколько фото в ленте до фильтра по отметкам.
  const total = $derived(store.beforeMarks.length)
  const label = $derived(filterLabel(store.filter, total, store.visible.length, libfilter.active))
  const marks = $derived(prefs.can('marks'))

  const RATINGS: { v: number; text: string; title: string }[] = [
    { v: 0, text: 'Любая', title: 'Любая оценка' },
    ...[1, 2, 3, 4, 5].map((n) => ({ v: n, text: '≥' + '★'.repeat(n), title: `Оценка не ниже ${n}` })),
    { v: -1, text: 'Без оценки', title: 'Только фото без оценки' },
  ]
  const FLAGS: { v: FlagFilter; text: string }[] = [
    { v: 'all', text: 'Все' },
    { v: 'picked', text: 'Отобранные' },
    { v: 'notRejected', text: 'Без отклонённых' },
    { v: 'rejected', text: 'Отклонённые' },
  ]

  let open = $state(false)
  let btn = $state<HTMLButtonElement>()

  function show() {
    open = true
  }

  function close() {
    open = false
    btn?.focus()
  }

  // Лента свёрнута (крупный просмотр, галерея) — окно не к чему привязывать.
  $effect(() => {
    if (modes.big || gallery.cur) open = false
  })
</script>

<span class="filter" data-guide="mark-filter">
  <button
    bind:this={btn}
    class="fbtn"
    class:active={libfilter.active}
    type="button"
    title={libfilter.active ? 'Фильтр: ' + libfilter.summary : 'Какие фото показывать в ленте'}
    aria-haspopup="dialog"
    aria-expanded={open}
    onclick={() => (open ? close() : show())}
  >
    {label}<Icon name="caret-down" size={14} />
  </button>
  {#if marks && libfilter.active}
    <button
      class="ghost reset"
      type="button"
      title="Сбросить фильтр по отметкам"
      aria-label="Сбросить фильтр по отметкам"
      onclick={() => libfilter.reset()}><Icon name="x" size={12} /></button
    >
  {/if}
</span>

<Popover anchor={btn} bind:open label="Фильтр ленты" kind="filter">
  <div class="pop">
    <div class="cap">Порядок</div>
    <div class="row">
      {#each LIB_SORTS as s (s.v)}
        <button
          type="button"
          class="opt"
          class:on={modes.libSort === s.v}
          title={s.title}
          aria-pressed={modes.libSort === s.v}
          onclick={() => modes.setLibSort(s.v)}>{s.text}</button
        >
      {/each}
    </div>

    <div class="cap">Показывать</div>
    <div class="row">
      <button
        type="button"
        class="opt"
        class:on={store.filter === 'all'}
        aria-pressed={store.filter === 'all'}
        onclick={() => (store.filter = 'all')}>Все фото ({store.inFolders.length})</button
      >
      <button
        type="button"
        class="opt"
        class:on={store.filter === 'unused'}
        aria-pressed={store.filter === 'unused'}
        onclick={() => (store.filter = 'unused')}>Не в альбоме ({store.free.length})</button
      >
    </div>

    {#if marks}
      <div class="cap">Оценка</div>
      <div class="row">
        {#each RATINGS as r (r.v)}
          <button
            type="button"
            class="opt"
            class:on={libfilter.minRating === r.v}
            title={r.title}
            aria-pressed={libfilter.minRating === r.v}
            onclick={() => (libfilter.minRating = r.v)}>{r.text}</button
          >
        {/each}
      </div>

      <div class="cap">Цвет метки <span class="hint">(ничего не выбрано — любой)</span></div>
      <div class="row">
        {#each LABEL_VIEW as c (c.id)}
          <button
            type="button"
            class="opt dot"
            class:on={libfilter.colors.has(c.id)}
            style:--c="var(--lbl-{c.id})"
            title={c.name[0].toUpperCase() + c.name.slice(1)}
            aria-label={c.name}
            aria-pressed={libfilter.colors.has(c.id)}
            onclick={() => libfilter.toggleColor(c.id)}
          ></button>
        {/each}
        <button
          type="button"
          class="opt"
          class:on={libfilter.colors.has('')}
          title="Фото без цветной метки"
          aria-pressed={libfilter.colors.has('')}
          onclick={() => libfilter.toggleColor('')}>Без метки</button
        >
      </div>

      <div class="cap">Флажок</div>
      <div class="row">
        {#each FLAGS as f (f.v)}
          <button
            type="button"
            class="opt"
            class:on={libfilter.flag === f.v}
            aria-pressed={libfilter.flag === f.v}
            onclick={() => (libfilter.flag = f.v)}>{f.text}</button
          >
        {/each}
      </div>

      <div class="foot">
        <span class="count" role="status">Показано {store.visible.length} из {total}</span>
        <button type="button" class="ghost" disabled={!libfilter.active} onclick={() => libfilter.reset()}>Сбросить</button>
      </div>
    {/if}
  </div>
</Popover>

<style>
  /* Корень живёт и в строке панели, и в окне «⋯»: не переносится, без внешних отступов */
  .filter {
    display: inline-flex;
    align-items: center;
    flex-wrap: nowrap;
    white-space: nowrap;
    gap: 4px;
  }
  .fbtn {
    display: inline-flex;
    align-items: center;
    gap: 4px;
  }
  .fbtn.active {
    background: var(--accent);
    color: #1e1e1e;
    font-weight: 600;
  }
  .fbtn.active:hover:not(:disabled) {
    background: #ffe53a;
  }
  .count {
    font-size: 12px;
    color: var(--muted);
    white-space: nowrap;
  }
  .reset {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 26px;
    height: 26px;
    padding: 0;
  }
  /* Рамка, фон и позиция — у Popover; здесь только ширина содержимого */
  .pop {
    width: 278px; /* вместе с рамкой и отступами Popover — 300 */
  }
  .cap {
    margin: 10px 0 6px;
    color: var(--muted);
  }
  .cap:first-child {
    margin-top: 0;
  }
  .hint {
    opacity: 0.7;
  }
  .row {
    display: flex;
    flex-wrap: wrap;
    gap: 4px;
  }
  .opt {
    height: 26px;
    padding: 0 8px;
    font-size: 12px;
    border-color: var(--line);
  }
  .opt.on {
    background: #48484d;
    border-color: var(--accent);
  }
  .dot {
    position: relative;
    width: 26px;
    padding: 0;
  }
  .dot::before {
    content: '';
    position: absolute;
    inset: 6px;
    border-radius: 50%;
    background: var(--c);
  }
  .foot {
    display: flex;
    align-items: center;
    justify-content: space-between;
    margin-top: 12px;
    padding-top: 8px;
    border-top: 1px solid var(--line);
  }
  .foot button {
    height: 26px;
    padding: 0 8px;
    font-size: 12px;
  }
</style>
