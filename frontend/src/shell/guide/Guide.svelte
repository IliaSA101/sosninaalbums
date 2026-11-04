<script lang="ts">
  import Modal from '../../base/modal/Modal.svelte'
  import { prefs } from '../../base/prefs/prefs.svelte'
  import { shortOf } from '../../base/prefs/levelcustom'
  import { rowOf } from '../../base/prefs/levels'
  import { keysUi } from '../hotkeys/keysui.svelte'
  import { ARTICLES, placesOf } from '../guidetext/articles'
  import GuideCard from './GuideCard.svelte'
  import GuideList from './GuideList.svelte'
  import { countLine, progress, sectionsOf } from './guidelist'
  import { guideCount } from './inventory'
  import { selectQuery } from './queryfocus'
  import { guide } from './guidestate.svelte'
  import { learned } from './learned.svelte'
  import { search } from './search'
  import { videos } from './videos.svelte'

  // Справочник функций (спека 033, 136): поиск, список по разделам, справа карточка. Окно целиком работает
  // с клавиатуры: ↓ из поиска входит в список, ↑/↓/Home/End двигают выбор, Enter — «Показать», Esc закрывает (Modal).
  const base = $derived(guide.rowFilter ? ARTICLES.filter((a) => rowOf(a.feature) === guide.rowFilter) : ARTICLES)
  const searching = $derived(!!guide.query.trim())
  const found = $derived(search(guide.query))
  const sections = $derived.by(() => {
    if (!searching) return sectionsOf(base)
    const ids = new Set(base.map((a) => a.id))
    const items = found.hits.map((h) => h.a).filter((a) => ids.has(a.id))
    return items.length ? [{ id: 'found', title: '', items }] : []
  })
  const flat = $derived(sections.flatMap((s) => s.items))
  const cur = $derived(flat.find((a) => a.id === guide.sel) ?? flat[0])
  // Сворачиваются только разделы полного списка: при поиске список плоский, при фильтре строки уровней все открыты.
  const fold = $derived(!searching && !guide.rowFilter)
  const count = $derived(
    countLine({ shown: flat.length, total: ARTICLES.length, searching, filtered: !!guide.rowFilter, functions: guideCount() }),
  )

  // «Изучено n из m»: m — статьи, доступные на текущем уровне (prefs.can); без отметок строки нет.
  const prog = $derived(progress(ARTICLES, learned.ids, prefs.can))

  // Отметки читаются при каждом открытии окна (одно чтение файла).
  $effect(() => {
    if (guide.open) void learned.load()
  })

  // Ролики: список из кэша Go при открытии и ход скачивания, пока окно открыто.
  $effect(() => {
    if (!guide.open) return
    void videos.load()
    const off = videos.listen()
    return () => {
      off()
      videos.reset()
    }
  })

  let list = $state<GuideList>()
  let pane = $state<HTMLElement>()

  // Поиск: ↓ входит в список (на выбранную строку), Enter — «Показать» выбранной.
  function key(e: KeyboardEvent) {
    if (e.ctrlKey || e.altKey || e.shiftKey || e.metaKey || e.isComposing) return
    if (e.key === 'ArrowDown') {
      e.preventDefault()
      list?.focusSel()
    } else if (e.key === 'Enter' && cur && placesOf(cur).length > 0) {
      e.preventDefault()
      void guide.show(cur.id)
    }
  }

  // Ввод запроса пользователем: выбор переходит на первую строку выдачи (иначе прежняя статья «липнет»).
  // Прежние запрос и выбор при повторном F1 этим не затрагиваются: туда запрос не пишется.
  function typed(e: Event) {
    guide.query = (e.currentTarget as HTMLInputElement).value
    if (searching && flat[0]) guide.sel = flat[0].id
  }

  const focusSearch = () => document.querySelector<HTMLInputElement>('.guide .q')?.focus()
  const focusCard = () => pane?.querySelector<HTMLElement>('article')?.focus()

  const allKeys = () => keysUi.show()
  const levels = () => (prefs.compare = true)
</script>

{#snippet headExtra()}
  <span class="count">{count}</span>
{/snippet}

{#if guide.open}
  <Modal title="Справочник функций" width={760} onclose={() => (guide.open = false)} {headExtra}>
    <div class="guide">
      <input
        class="q"
        type="search"
        data-autofocus
        use:selectQuery
        placeholder="Что вы ищете? Например: виньетка, обложка, зеркало"
        aria-label="Поиск по функциям"
        value={guide.query}
        oninput={typed}
        onkeydown={key}
      />
      {#if guide.rowFilter}
        <p class="chip">
          Функции строки «{shortOf(guide.rowFilter)}»
          <button class="x" aria-label="Показать все функции" onclick={() => (guide.rowFilter = null)}>✕</button>
        </p>
      {/if}
      {#if found.from}<p class="from">Искали: «{found.from}»</p>{/if}
      {#if found.loose}<p class="from">Точных совпадений нет — похожие</p>{/if}
      <div class="cols">
        <GuideList bind:this={list} {sections} curId={cur?.id} {fold} onsearch={focusSearch} oncard={focusCard}>
          {#snippet empty()}
            <p class="empty">
              Ничего не нашлось. Попробуйте другое слово или откройте <button class="link" onclick={allKeys}>все клавиши</button>
            </p>
          {/snippet}
        </GuideList>
        <div class="pane" bind:this={pane}>
          {#if cur}{#key cur.id}<GuideCard a={cur} />{/key}{/if}
        </div>
      </div>
      <div class="foot">
        <button class="link" onclick={allKeys}>Все клавиши</button>
        {#if prog.n > 0}<span class="prog">Изучено {prog.n} из {prog.m}</span>{/if}
        <button class="link" onclick={levels}>Уровни и функции…</button>
      </div>
    </div>
  </Modal>
{/if}

<style>
  /* Высота окна — как у «Горячих клавиш»; колонки берут остаток после поиска и низа (Modal: рамка и отступы 28, шапка 36) */
  .guide {
    height: calc(min(724px, 100vh - 32px) - 64px);
    display: flex;
    flex-direction: column;
    gap: 8px;
  }
  .count {
    font-size: 12px;
    font-weight: 400;
    color: var(--muted);
  }
  .q {
    flex: none;
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
    flex: none;
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
    flex: 1 1 0;
    min-height: 0;
    display: grid;
    grid-template-columns: 260px 1fr;
    gap: 16px;
  }
  .pane {
    min-height: 0;
    overflow-y: auto;
    scrollbar-width: thin;
    padding-right: 4px;
  }
  .empty {
    margin: 8px;
    color: var(--muted);
  }
  .foot {
    flex: none;
    display: flex;
    justify-content: space-between;
    padding-top: 6px;
    border-top: 1px solid var(--line);
  }
  .prog {
    align-self: center;
    font-size: 12px;
    color: var(--muted);
  }
  .link {
    height: auto;
    padding: 2px 0;
    background: transparent;
    color: var(--accent);
    white-space: normal;
  }
</style>
