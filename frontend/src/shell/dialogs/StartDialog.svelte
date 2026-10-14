<script lang="ts">
  import Modal from '../../base/ui/Modal.svelte'
  import { Api, store } from '../../base/core/store.svelte'
  import { prefs } from '../../base/prefs/prefs.svelte'
  import { forgetRecent, openRecent } from '../menu/commands'
  import type { Level } from '../../base/prefs/levels'
  import { tick } from 'svelte'
  import LevelCards from './LevelCards.svelte'
  import LevelMatrix from './LevelMatrix.svelte'
  import NewProduct from '../newdoc/NewProduct.svelte'
  import { newDoc } from '../newdoc/newdoc.svelte'
  import { guide } from '../guide/guidestate.svelte'
  import { recentRows } from './recent'
  import { WHATS_NEW, WHATS_NEW_VERSION } from './whatsnew'

  // Стартовое окно (спеки 010, 011): поверх открытого альбома при запуске, пока стоит флажок
  // «Показывать при запуске» (первый запуск — всегда); потом — «Справка ▸ Стартовое окно».
  // Сверху уровни, матрица функций — по запросу. Открывается окно уровней — это окно прячется,
  // после закрытия оно возвращается.
  const rows = $derived(recentRows(store.recent, Date.now()))
  const name = $derived(store.projectName)

  // Окно открыто — список перечитывается (если альбом сохраняли «Сохранить как»).
  $effect(() => {
    if (prefs.startOpen) void store.refreshRecent()
  })

  const close = () => prefs.closeStart()

  // Матрица и «Что нового» свёрнуты при каждом открытии окна, состояние не запоминается.
  let all = $state(false)
  let newsOpen = $state(false)
  let newsEl = $state<HTMLElement>()

  $effect(() => {
    if (!prefs.startOpen) {
      all = false
      newsOpen = false
      newDoc.kind = 'album' // окно снова открывается на альбоме
    }
  })

  // «Справка ▸ Что нового»: раскрыть список и показать его.
  $effect(() => {
    if (prefs.startOpen && prefs.news) {
      newsOpen = true
      void tick().then(() => newsEl?.scrollIntoView({ block: 'nearest' }))
    }
  })

  // «Свой» разворачивает матрицу: набор настраивают сразу.
  function pickLevel(l: Level) {
    void prefs.set(l)
    if (l === 'custom') all = true
  }

  // Диалог выбора файла нативный: окно остаётся, пока он открыт, и закрывается, только если альбом открылся (не отмена).
  async function open() {
    const seq = store.openSeq
    await store.replaceState(Api.OpenProject())
    if (store.openSeq !== seq) close()
  }

  async function recent(path: string) {
    if (await openRecent(path)) close()
  }
</script>

{#if prefs.startOpen && !prefs.compare}
  <Modal title="Sosnina Albums" onclose={close} wide>
    <div class="start">
      <h3 class="first">Уровень интерфейса</h3>
      <LevelCards onpick={pickLevel} />
      <p class="note">
        Уровень прячет инструменты, а не данные
        <button class="link" aria-expanded={all} onclick={() => (all = !all)}>{all ? 'Свернуть ▴' : 'Все функции ▾'}</button>
      </p>
      {#if all}
        <LevelMatrix />
      {/if}

      <div class="actions">
        <button class="accent" data-autofocus onclick={close} title={name}>Продолжить: {name}</button>
        <button onclick={open}>Открыть…</button>
      </div>

      <h3>Последние альбомы</h3>
      {#if rows.length === 0}
        <p class="empty">Пока пусто: здесь появятся сохранённые альбомы</p>
      {:else}
        <ul class="recent">
          {#each rows as r (r.path)}
            <li class:missing={r.missing}>
              <button class="row" aria-disabled={r.missing} title={r.tip} onclick={() => !r.missing && recent(r.path)}>
                <span class="nm">{r.name}</span>
                <span class="dt">{r.missing ? 'файл не найден' : r.date}</span>
              </button>
              <button class="forget" title="Убрать из списка" aria-label="Убрать {r.name} из списка" onclick={() => forgetRecent(r.path)}
                >✕</button
              >
            </li>
          {/each}
        </ul>
      {/if}

      <h3>Новое изделие</h3>
      <NewProduct oncreated={close} />

      <div class="foot">
        <span class="links">
          <button class="link" aria-expanded={newsOpen} onclick={() => (newsOpen = !newsOpen)}>
            Что нового в {store.version || WHATS_NEW_VERSION}
            {newsOpen ? '▾' : '▸'}
          </button>
          <!-- Справочник открывается поверх этого окна (стек Modal) -->
          <button class="link" onclick={() => guide.openFresh()}>Справочник функций · F1</button>
        </span>
        <label class="show">
          <input type="checkbox" checked={prefs.showStart} onchange={(e) => prefs.setShowStart(e.currentTarget.checked)} />
          Показывать при запуске
        </label>
      </div>
      {#if newsOpen}
        <ul class="news" bind:this={newsEl}>
          {#each WHATS_NEW as line (line)}
            <li>{line}</li>
          {/each}
        </ul>
      {/if}
    </div>
  </Modal>
{/if}

<style>
  /* Окно шире обычного: четыре карточки уровней в ряд и матрица */
  :global(.box.wide):has(.start) {
    width: 760px;
  }
  .start {
    display: flex;
    flex-direction: column;
    gap: 8px;
  }
  h3 {
    margin: 8px 0 0;
    font-size: 12px;
    font-weight: 600;
    color: var(--muted);
    text-transform: uppercase;
    letter-spacing: 0.04em;
  }
  h3.first {
    margin-top: 0;
  }
  .note {
    margin: 0;
    display: flex;
    align-items: baseline;
    gap: 12px;
    color: var(--muted);
    font-size: 12px;
  }
  .actions {
    display: flex;
    gap: 8px;
  }
  .actions .accent {
    flex: 1;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .recent {
    margin: 0;
    padding: 0;
    list-style: none;
    display: flex;
    flex-direction: column;
    gap: 2px;
  }
  .recent li {
    display: flex;
    align-items: center;
    gap: 4px;
  }
  .row {
    flex: 1;
    min-width: 0;
    height: auto;
    min-height: 28px;
    padding: 4px 8px;
    display: flex;
    justify-content: space-between;
    gap: 12px;
    text-align: left;
    background: transparent;
    border-radius: 4px;
  }
  .row:hover:not([aria-disabled='true']) {
    background: #38383c;
  }
  .nm {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .dt {
    flex: none;
    color: var(--muted);
    font-size: 12px;
  }
  .missing .row {
    opacity: 0.45;
    cursor: default;
  }
  .forget {
    flex: none;
    width: 26px;
    height: 26px;
    padding: 0;
    background: transparent;
    color: var(--muted);
  }
  .forget:hover {
    color: var(--text);
  }
  .empty {
    margin: 0;
    color: var(--muted);
  }
  .link {
    height: auto;
    padding: 2px 0;
    background: transparent;
    color: var(--accent);
  }
  .foot {
    display: flex;
    align-items: center;
    justify-content: space-between;
  }
  .links {
    display: flex;
    gap: 16px;
  }
  .show {
    display: flex;
    align-items: center;
    gap: 6px;
  }
  .news {
    margin: 0;
    padding-left: 18px;
    color: var(--muted);
    line-height: 1.5;
  }
</style>
