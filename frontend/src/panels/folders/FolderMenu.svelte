<script lang="ts">
  import { store, Api } from '../../base/core/store.svelte'
  import { modes } from '../../base/prefs/view.svelte'
  import { errText } from '../../base/words/errtext'
  import Icon from '../../base/ui/Icon.svelte'
  import Popover from '../../base/ui/Popover.svelte'
  import { gallery } from '../gallery/gallery.svelte'
  import { inFeedText } from './folders'
  import { folderfilter } from './folderfilter.svelte'
  import { foldersUi } from './foldersui.svelte'
  import FolderRow from './FolderRow.svelte'
  import { buttonText, buttonTitle, checkState, photoRows, rows, visibleCounts } from './tree'

  // Папки проекта одной кнопкой (024): без папок и отдельных фото — прежняя «+ Папка с фото», иначе — окно с деревом
  // (093); отдельные фото (097) — ветка «Фото отдельно» в том же окне.
  const folders = $derived(store.view.folders)
  const sources = $derived(folders.length > 0 || (store.view.loose ?? []).length > 0)
  let btn = $state<HTMLButtonElement>()
  // Фильтр виден по ленте, а не по набору ключей: скрытые ключи без фото (идёт чтение) ленту не режут.
  const filtered = $derived(folderfilter.active && store.inFolders.length < store.photos.length)

  // Лента свёрнута (крупный просмотр, галерея) — окну не к чему быть привязанным.
  $effect(() => {
    if (modes.big || gallery.cur || !sources) foldersUi.open = false
  })

  // Дерево и строки — только при открытом окне (как числа 024): на 20 тыс. фото это десятки мс.
  const forest = $derived(foldersUi.open ? foldersUi.forest() : [])
  const vis = $derived(visibleCounts(forest, folderfilter.hidden))
  const list = $derived(rows(forest, (n) => folderfilter.isOpen(n)))
  const checks = $derived(photoRows(forest) > 1)
  const grand = $derived(forest.reduce((s, n) => s + n.total, 0))

  // Перечитать папки и отметки; ход чтения приходит событиями scan, как при «+ Папка».
  async function rescan() {
    try {
      await Api.Rescan()
    } catch (e) {
      store.notify(errText(e), 'error')
    }
  }

  // Кнопки внутри панели не гасят фокус и не начинают перенос ленты.
  const stop = (e: PointerEvent) => e.stopPropagation()
</script>

{#if !sources}
  <button type="button" data-guide="folders" onclick={() => store.run(Api.ChooseFolder())}>+ Папка с фото</button>
{:else}
  <span class="folders">
    <button
      bind:this={btn}
      type="button"
      class="fbtn"
      class:active={filtered}
      data-guide="folders"
      title={buttonTitle(store.inFolders.length, store.photos.length, filtered)}
      aria-haspopup="dialog"
      aria-expanded={foldersUi.open}
      onclick={() => (foldersUi.open = !foldersUi.open)}
    >
      <Icon name="folder" size={16} />
      {buttonText(folders.length, filtered)}
      <Icon name="caret-down" size={12} />
    </button>
    {#if filtered}
      <button
        class="ghost reset"
        type="button"
        title="Показать все папки"
        aria-label="Показать все папки"
        onclick={() => folderfilter.showAll()}><Icon name="x" size={12} /></button
      >
    {/if}
  </span>
{/if}

<Popover anchor={btn} bind:open={foldersUi.open} label="Папки с фото" kind="folders">
  <div class="box">
    <div class="adds">
      <button type="button" class="add" onpointerdown={stop} onclick={() => foldersUi.add()}>
        <Icon name="plus" size={14} /> Добавить папку…
      </button>
      <button
        type="button"
        class="add"
        title="Выбрать фото из любых папок: они встанут в ленту без папки целиком"
        onpointerdown={stop}
        onclick={() => foldersUi.addFiles()}
      >
        <Icon name="plus" size={14} /> Добавить фото…
      </button>
    </div>
    <ul class="tree">
      {#each list as r, i (r.node.key)}
        {@const v = vis.get(r.node) ?? 0}
        <FolderRow
          node={r.node}
          depth={r.depth}
          check={checkState(r.node, v)}
          {checks}
          only={checks && r.node.total > 0 && r.node.total < grand}
          open={folderfilter.isOpen(r.node)}
          flash={foldersUi.flash === r.node.key}
          sep={r.node.kind === 'group' && i > 0}
          ontoggle={() => folderfilter.toggle(r.node)}
          onopen={() => folderfilter.toggleOpen(r.node)}
          ononly={() => folderfilter.only(forest, r.node)}
          onremove={() => foldersUi.remove(r.node)}
        />
      {/each}
    </ul>
    <!-- Строка состояния есть всегда при флажках: от неё зависит высота окна, а окно у нижней ленты растёт вверх -->
    {#if checks}
      <div class="foot">
        <span class="count" role="status">{inFeedText(store.inFolders.length, store.photos.length)}</span>
        <button type="button" class="ghost" disabled={!filtered} onpointerdown={stop} onclick={() => folderfilter.showAll()}
          >Показать все</button
        >
      </div>
    {/if}
    <button
      type="button"
      class="all"
      disabled={store.scan.active}
      title="Перечитать папки и отдельные фото сейчас. Программа и сама замечает новые и пропавшие файлы, когда вы возвращаетесь в окно. Lightroom пишет отметки в файлы, если включена автозапись XMP или нажат Ctrl+S"
      onpointerdown={stop}
      onclick={rescan}>Обновить все</button
    >
  </div>
</Popover>

<style>
  .fbtn {
    display: inline-flex;
    align-items: center;
    gap: 6px;
  }
  .box {
    display: flex;
    flex-direction: column;
    gap: 6px;
    width: 330px;
    max-width: calc(100vw - 40px);
  }
  .box button {
    height: 26px;
    font-size: 12px;
    padding: 0 8px;
  }
  .adds {
    display: flex;
    gap: 6px;
  }
  .add {
    flex: 1;
    min-width: 0;
    display: inline-flex;
    align-items: center;
    gap: 6px;
    justify-content: flex-start;
  }
  ul.tree {
    list-style: none;
    margin: 0;
    padding: 0;
    border-top: 1px solid var(--line);
    border-bottom: 1px solid var(--line);
    max-height: 330px;
    overflow-y: auto;
  }
  .folders {
    display: inline-flex;
    align-items: center;
    gap: 4px;
  }
  .reset {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 26px;
    height: 26px;
    padding: 0;
  }
  .fbtn.active {
    background: var(--accent);
    color: #1e1e1e;
    font-weight: 600;
  }
  .fbtn.active:hover:not(:disabled) {
    background: #ffe53a;
  }
  .foot {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 8px;
  }
  .count {
    font-size: 12px;
    color: var(--muted);
  }
</style>
