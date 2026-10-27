<script lang="ts">
  import { tick } from 'svelte'
  import Modal from '../../base/modal/Modal.svelte'
  import { prefs } from '../../base/prefs/prefs.svelte'
  import { shortOf } from '../../base/prefs/levelcustom'
  import { rowOf } from '../../base/prefs/levels'
  import { keysUi } from '../hotkeys/keysui.svelte'
  import { ARTICLES, GUIDE_GROUPS, placesOf, type Article } from '../guidetext/articles'
  import GuideCard from './GuideCard.svelte'
  import { selectQuery } from './queryfocus'
  import { guide } from './guidestate.svelte'
  import { grouped, search, type Section } from './search'

  // Справочник функций (спека 033): поиск, список по разделам, справа карточка. Окно целиком работает
  // с клавиатуры: ↑/↓ двигают выбор, Enter — «Показать», Esc закрывает (Modal).
  const base = $derived(guide.rowFilter ? ARTICLES.filter((a) => rowOf(a.feature) === guide.rowFilter) : ARTICLES)
  const found = $derived(search(guide.query))
  const sections = $derived.by<Section[]>(() => {
    if (found.hits.length === 0 && guide.query.trim()) return []
    if (!guide.query.trim()) return grouped(base)
    const ids = new Set(base.map((a) => a.id))
    return [{ id: 'found', title: '', items: found.hits.map((h) => h.a).filter((a) => ids.has(a.id)) }]
  })
  const flat = $derived(sections.flatMap((s) => s.items))
  const cur = $derived(flat.find((a) => a.id === guide.sel) ?? flat[0])
  const groupTitle = (a: Article) => GUIDE_GROUPS.find((g) => g.id === a.group)?.title ?? ''

  function move(delta: number) {
    if (flat.length === 0) return
    const at = cur ? flat.indexOf(cur) : -1
    guide.sel = flat[Math.max(0, Math.min(at + delta, flat.length - 1))].id
  }

  function key(e: KeyboardEvent) {
    if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
      e.preventDefault()
      move(e.key === 'ArrowDown' ? 1 : -1)
    } else if (e.key === 'Enter' && cur && placesOf(cur).length > 0) {
      e.preventDefault()
      void guide.show(cur.id)
    }
  }

  // Выбранный пункт всегда в поле зрения списка.
  let list = $state<HTMLElement>()
  $effect(() => {
    const id = cur?.id
    if (id) void tick().then(() => list?.querySelector(`[data-id="${id}"]`)?.scrollIntoView({ block: 'nearest' }))
  })

  const allKeys = () => keysUi.show()
  const levels = () => (prefs.compare = true)
</script>

{#if guide.open}
  <Modal title="Справочник функций" onclose={() => (guide.open = false)} wide>
    <div class="guide">
      <input
        class="q"
        type="search"
        data-autofocus
        use:selectQuery
        placeholder="Что вы ищете? Например: виньетка, обложка, зеркало"
        aria-label="Поиск по функциям"
        bind:value={guide.query}
        onkeydown={key}
      />
      {#if guide.rowFilter}
        <p class="chip">
          Функции строки «{shortOf(guide.rowFilter)}»
          <button class="x" aria-label="Показать все функции" onclick={() => (guide.rowFilter = null)}>✕</button>
        </p>
      {/if}
      {#if found.from}<p class="from">Искали: «{found.from}»</p>{/if}
      <div class="cols">
        <div class="list" bind:this={list} role="listbox" aria-label="Функции">
          {#each sections as s (s.id)}
            {#if s.title}<h3>{s.title}</h3>{/if}
            {#each s.items as a (a.id)}
              <button
                class="item"
                class:on={a.id === cur?.id}
                role="option"
                aria-selected={a.id === cur?.id}
                data-id={a.id}
                onclick={() => (guide.sel = a.id)}
                ondblclick={() => placesOf(a).length > 0 && guide.show(a.id)}
              >
                <span class="t">{a.title}</span>
                {#if s.id === 'found'}<span class="g">{groupTitle(a)}</span>{/if}
              </button>
            {/each}
          {/each}
          {#if flat.length === 0}
            <p class="empty">
              Ничего не нашлось. Попробуйте другое слово или откройте <button class="link" onclick={allKeys}>все клавиши</button>
            </p>
          {/if}
        </div>
        <div class="pane">
          {#if cur}{#key cur.id}<GuideCard a={cur} />{/key}{/if}
        </div>
      </div>
      <div class="foot">
        <button class="link" onclick={allKeys}>Все клавиши</button>
        <button class="link" onclick={levels}>Уровни и функции…</button>
      </div>
    </div>
  </Modal>
{/if}

<style>
  /* Окно шире обычного: список слева и карточка справа */
  :global(.box.wide):has(.guide) {
    width: 760px;
  }
  .guide {
    display: flex;
    flex-direction: column;
    gap: 8px;
  }
  .q {
    width: 100%;
    height: 32px;
    padding: 0 10px;
    font: inherit;
    color: var(--text);
    background: #3a3a3e;
    border: 1px solid var(--line);
    border-radius: 6px;
  }
  .q:focus-visible {
    outline: 2px solid var(--accent);
    outline-offset: 0;
  }
  .chip,
  .from {
    margin: 0;
    color: var(--muted);
    font-size: 12px;
  }
  .x {
    height: auto;
    padding: 0 4px;
    background: transparent;
    color: var(--muted);
  }
  .cols {
    display: grid;
    grid-template-columns: 260px 1fr;
    gap: 16px;
    height: min(420px, calc(100vh - 220px));
    min-height: 200px;
  }
  .list,
  .pane {
    min-height: 0;
    overflow-y: auto;
    scrollbar-width: thin;
  }
  .pane {
    padding-right: 4px;
  }
  h3 {
    margin: 10px 0 2px;
    font-size: 11px;
    font-weight: 600;
    color: var(--muted);
    text-transform: uppercase;
    letter-spacing: 0.04em;
  }
  h3:first-child {
    margin-top: 0;
  }
  .item {
    width: 100%;
    height: auto;
    min-height: 28px;
    padding: 4px 8px;
    display: flex;
    flex-direction: column;
    align-items: flex-start;
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
  .g {
    font-size: 11px;
    color: var(--muted);
  }
  .empty {
    margin: 8px;
    color: var(--muted);
  }
  .foot {
    display: flex;
    justify-content: space-between;
    padding-top: 6px;
    border-top: 1px solid var(--line);
  }
  .link {
    height: auto;
    padding: 2px 0;
    background: transparent;
    color: var(--accent);
    white-space: normal;
  }
</style>
