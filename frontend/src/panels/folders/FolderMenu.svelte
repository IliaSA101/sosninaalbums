<script lang="ts">
  import { store, Api } from '../../base/core/store.svelte'
  import { modes } from '../../base/prefs/view.svelte'
  import { errText } from '../../base/words/errtext'
  import Icon from '../../base/ui/Icon.svelte'
  import Popover from '../../base/ui/Popover.svelte'
  import { gallery } from '../gallery/gallery.svelte'
  import { folderName } from '../library/names'
  import { folderCounts, inFeedText } from './folders'
  import { folderfilter } from './folderfilter.svelte'

  // Папки проекта одной кнопкой (спека 024): без папок — прежняя «+ Папка с фото», с папками — окно со списком.
  const folders = $derived(store.view.folders)
  let open = $state(false)
  let btn = $state<HTMLButtonElement>()
  // 054: флажки «показывать в ленте» — при ≥ 2 папках; скрыты — кнопка жёлтая «Папки 2 из 3».
  const multi = $derived(folders.length >= 2)
  const hiddenN = $derived(multi ? folderfilter.hiddenIn(folders) : 0)
  const hiddenNames = $derived(folders.filter((f) => folderfilter.isHidden(f)).map(folderName))

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
  <button type="button" data-guide="folders" onclick={add}>+ Папка с фото</button>
{:else}
  <span class="folders">
    <button
      bind:this={btn}
      type="button"
      class="fbtn"
      class:active={hiddenN > 0}
      data-guide="folders"
      title={hiddenN > 0 ? 'Скрыты: ' + hiddenNames.join(', ') : 'Папки с фото: добавить, убрать, перечитать'}
      aria-haspopup="dialog"
      aria-expanded={open}
      onclick={() => (open = !open)}
    >
      <Icon name="folder" size={16} />
      {hiddenN > 0 ? `Папки ${folders.length - hiddenN} из ${folders.length}` : `Папки ${folders.length}`}
      <Icon name="caret-down" size={12} />
    </button>
    {#if hiddenN > 0}
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

<Popover anchor={btn} bind:open label="Папки с фото" kind="folders">
  <!-- Числа считаются только при открытом окне (Popover монтирует содержимое открытым) -->
  {@const counts = folderCounts(folders, store.photos)}
  <div class="box">
    <button type="button" class="add" onpointerdown={stop} onclick={add}>
      <Icon name="plus" size={14} /> Добавить папку…
    </button>
    <ul class:scroll={folders.length > 10}>
      {#each folders as f, i (f)}
        <li title={f} class:off={multi && folderfilter.isHidden(f)}>
          {#if multi}
            <label class="name" title="Показывать в ленте">
              <input
                type="checkbox"
                checked={!folderfilter.isHidden(f)}
                aria-label="Показывать «{folderName(f)}» в ленте"
                onchange={() => folderfilter.toggle(f)}
              />
              <span>{folderName(f)}</span>
            </label>
          {:else}
            <span class="name"><span>{folderName(f)}</span></span>
          {/if}
          <span class="n">{counts[i]}</span>
          <button
            type="button"
            class="ghost x"
            title="Убрать папку"
            aria-label="Убрать папку «{folderName(f)}»"
            onpointerdown={stop}
            onclick={() => store.run(Api.RemoveFolder(f))}><Icon name="x" size={12} stroke={2.4} /></button
          >
        </li>
      {/each}
    </ul>
    <!-- Строка состояния есть всегда: от неё зависит высота окна, а окно у нижней ленты растёт вверх, и строки уезжали бы из-под курсора -->
    {#if multi}
      <div class="foot">
        <span class="count" role="status">{inFeedText(store.inFolders.length, store.photos.length)}</span>
        <button type="button" class="ghost" disabled={hiddenN === 0} onpointerdown={stop} onclick={() => folderfilter.showAll()}
          >Показать все</button
        >
      </div>
    {/if}
    <button
      type="button"
      class="all"
      disabled={store.scan.active}
      title="Перечитать папки сейчас. Программа и сама замечает новые и пропавшие файлы, когда вы возвращаетесь в окно. Lightroom пишет отметки в файлы, если включена автозапись XMP или нажат Ctrl+S"
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
  .name {
    flex: 1;
    min-width: 0;
    display: flex;
    align-items: center;
    gap: 8px;
    overflow: hidden;
    white-space: nowrap;
  }
  label.name {
    cursor: pointer;
  }
  .name span {
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .n {
    color: var(--muted);
  }
  /* Скрытая папка бледнеет, число остаётся прежним */
  li.off .name span,
  li.off .n {
    opacity: 0.45;
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
  .box .x {
    width: 22px;
    height: 22px;
    padding: 0;
    display: grid;
    place-items: center;
    color: var(--muted);
  }
</style>
