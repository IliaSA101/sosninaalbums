<script lang="ts">
  import { store, Api } from '../../base/core/store.svelte'
  import { modes } from '../../base/prefs/view.svelte'
  import { errText } from '../../base/core/errtext'
  import Icon from '../../base/ui/Icon.svelte'
  import Popover from '../../base/ui/Popover.svelte'
  import { gallery } from '../layouts/gallery.svelte'
  import { folderName } from './names'
  import { folderCounts } from './folders'

  // Папки проекта одной кнопкой (спека 024): без папок — прежняя «+ Папка с фото», с папками — окно со списком.
  const folders = $derived(store.view.folders)
  let open = $state(false)
  let btn = $state<HTMLButtonElement>()

  // Лента свёрнута (крупный просмотр, галерея) — окну не к чему быть привязанным.
  $effect(() => {
    if (modes.big || gallery.cur || folders.length === 0) open = false
  })

  function add() {
    open = false
    store.run(Api.ChooseFolder())
  }

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

{#if folders.length === 0}
  <button type="button" onclick={add}>+ Папка с фото</button>
{:else}
  <button
    bind:this={btn}
    type="button"
    class="fbtn"
    title="Папки с фото: добавить, убрать, перечитать"
    aria-haspopup="dialog"
    aria-expanded={open}
    onclick={() => (open = !open)}
  >
    <Icon name="folder" size={16} />
    Папки {folders.length}
    <Icon name="caret-down" size={12} />
  </button>
{/if}

<Popover anchor={btn} bind:open label="Папки с фото" kind="folders">
  <!-- Числа считаются только при открытом окне (Popover монтирует содержимое открытым) -->
  {@const counts = folderCounts(folders, store.photos)}
  <div class="box">
    <button type="button" class="add" onpointerdown={stop} onclick={add}>
      <Icon name="plus" size={14} /> Добавить папку…
    </button>
    <ul class:scroll={folders.length > 10}>
      {#each folders as f, i (f)}
        <li title={f}>
          <span class="name">{folderName(f)}</span>
          <span class="n">{counts[i]}</span>
          <button
            type="button"
            class="ghost x"
            title="Убрать папку из проекта"
            aria-label="Убрать папку «{folderName(f)}» из проекта"
            onpointerdown={stop}
            onclick={() => store.run(Api.RemoveFolder(f))}><Icon name="x" size={12} stroke={2.4} /></button
          >
        </li>
      {/each}
    </ul>
    <button
      type="button"
      class="all"
      disabled={store.scan.active}
      title="Перечитать папки и отметки. Lightroom пишет отметки в файлы, если включена автозапись XMP или нажат Ctrl+S"
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
    width: 300px;
  }
  .box button {
    height: 26px;
    font-size: 12px;
    padding: 0 8px;
  }
  .add {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    justify-content: flex-start;
  }
  ul {
    list-style: none;
    margin: 0;
    padding: 0;
    border-top: 1px solid var(--line);
    border-bottom: 1px solid var(--line);
  }
  ul.scroll {
    max-height: 260px;
    overflow-y: auto;
  }
  li {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 3px 0 3px 8px;
  }
  .name {
    flex: 1;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .n {
    color: var(--muted);
  }
  .box .x {
    width: 22px;
    height: 22px;
    padding: 0;
    display: grid;
    place-items: center;
    color: var(--muted);
  }
</style>
