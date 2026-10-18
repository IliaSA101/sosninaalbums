<script lang="ts">
  import { keymap } from '../../base/keys/keymap.svelte'
  import { store } from '../../base/core/store.svelte'
  import { exportShut, jobs } from '../../base/core/jobs.svelte'
  import { prefs } from '../../base/prefs/prefs.svelte'
  import { modes } from '../../base/prefs/view.svelte'
  import { run } from './commands'
  import { countLabel } from './topbar'
  import MenuBar from './MenuBar.svelte'
  import LevelBadge from './LevelBadge.svelte'
  import logo from '../../assets/logo.png'
  import { hasSeam, isSheet, pageSize } from '../../base/geom/sheet'

  // Страниц: у листа со швом две, у отдельного листа (шва нет) одна.
  const pages = $derived(store.view.spreads.reduce((n, sp) => n + (hasSeam(sp) ? 2 : 1), 0))
  const page = $derived(pageSize(store.view.settings))
  // Сводка: у листа виньетки страниц нет, считаем листы.
  const count = $derived(countLabel(store.view.spreads.length, pages, store.view.settings))
  const usedCount = $derived(store.photos.filter((p) => store.used.has(p.id)).length)
</script>

<header>
  <div class="group">
    <img class="logo" src={logo} alt="Sosnina films" draggable="false" title={`Sosnina Albums ${store.version}`} />
    <!-- Версия видна на скриншотах — по ним понятно, какая сборка у пользователя -->
    {#if store.version}<span class="version">v{store.version}</span>{/if}
    <MenuBar />
    <span class="title" title={store.view.path || 'Проект ещё не сохранён в файл — работа хранится в черновике'}>
      {store.projectName}{#if store.view.dirty && store.view.path}<span class="dot" title="Есть несохранённые изменения">•</span>{/if}
    </span>
    <div class="sep"></div>
    <span class="pair" data-guide="topbar-undo">
      <button class="icon" disabled={!store.view.canUndo} onclick={() => run('undo')} title={keymap.tip('Отменить', 'undo')}>↶</button>
      <button class="icon" disabled={!store.view.canRedo} onclick={() => run('redo')} title={keymap.tip('Повторить', 'redo')}>↷</button>
    </span>
  </div>

  <div class="info" title="{count} · {usedCount} / {store.photos.length} фото в альбоме">
    {count} · {usedCount} / {store.photos.length} фото в альбоме
  </div>

  <div class="group right">
    <LevelBadge />
    <button class="ghost" onclick={() => modes.openSection('settings')} title="Настройки альбома в панели деталей">
      {isSheet(store.view.settings) ? 'Лист' : 'Формат'}
      {page.w / 10}×{page.h / 10} см
    </button>
    {#if prefs.can('timeline')}
      <button
        class="mode"
        data-guide="topbar-big"
        class:on={modes.big}
        aria-pressed={modes.big}
        onclick={() => modes.toggleBig()}
        title={keymap.tip('Крупный просмотр', 'toggleBig')}
        aria-label="Крупный просмотр"
      >
        <svg viewBox="0 0 20 20" aria-hidden="true">
          <path d="M3 8V3h5M12 3h5v5M17 12v5h-5M8 17H3v-5" />
        </svg>
      </button>
    {/if}
    <button
      class="mode"
      data-guide="topbar-details"
      class:on={modes.details}
      aria-pressed={modes.details}
      onclick={() => modes.toggleDetails()}
      title={keymap.tip('Панель деталей', 'toggleDetails')}
      aria-label="Панель деталей"
    >
      <svg viewBox="0 0 20 20" aria-hidden="true">
        <rect x="2.5" y="3.5" width="15" height="13" rx="2" />
        <path d="M12 3.5v13M14 7h1.5M14 10h1.5" />
      </svg>
    </button>
    <button class="accent" disabled={exportShut(store.sheets.length, !!jobs.exporting)} onclick={() => run('export')}>
      {#if jobs.exporting && jobs.exporting.total > 0}
        Экспорт {jobs.exporting.done} / {jobs.exporting.total}
      {:else if jobs.exporting}
        Экспорт…
      {:else}
        Экспорт
      {/if}
    </button>
  </div>
</header>

<style>
  header {
    flex: none;
    height: 48px;
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 0 12px;
    background: var(--panel2);
    border-bottom: 1px solid var(--line);
    gap: 8px;
  }
  .group {
    flex: none;
    display: flex;
    align-items: center;
    gap: 4px;
  }
  .pair {
    display: flex;
    gap: 4px;
  }
  .title {
    margin-left: 8px;
    font-weight: 600;
    margin-right: 10px;
    max-width: 130px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .logo {
    height: 28px;
    margin-right: 6px;
  }
  .version {
    margin-right: 12px;
    font-size: 11px;
    color: var(--muted);
    white-space: nowrap;
  }
  .dot {
    color: var(--accent);
    margin-left: 4px;
  }
  /* Сжимается с «…», чтобы шапка влезала в 1200px */
  .info {
    flex: 1 1 0;
    min-width: 0;
    text-align: center;
    font-size: 13px;
    color: var(--muted);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .sep {
    width: 1px;
    height: 22px;
    background: var(--line);
    margin: 0 6px;
  }
  header button {
    padding: 0 8px;
  }
  .icon {
    width: 32px;
    padding: 0;
    font-size: 18px;
  }
  .mode {
    width: 32px;
    height: 32px;
    padding: 0;
    border-radius: 50%;
    display: grid;
    place-items: center;
    color: var(--text);
  }
  .mode svg {
    width: 18px;
    height: 18px;
    fill: none;
    stroke: currentColor;
    stroke-width: 1.6;
    stroke-linecap: round;
    stroke-linejoin: round;
  }
  .mode.on,
  .mode.on:hover:not(:disabled) {
    background: var(--accent);
    color: #1e1e1e;
  }
</style>
