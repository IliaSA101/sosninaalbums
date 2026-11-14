<script lang="ts">
  import { store } from '../../base/core/store.svelte'
  import { eventChord } from '../../base/keys/chord'
  import { keymap } from '../../base/keys/keymap.svelte'
  import Icon from '../../base/ui/Icon.svelte'
  import { nameSearch } from './namefind.svelte'

  // Поиск фото по имени в ленте (0.26-124): ⌕ в шапке; щелчок или Ctrl+F раскрывает поле. Образец — фильтр отметок
  // (MarkFilter): кнопка с подсветкой при действии и «Сбросить» рядом. N из M: M — лента до поиска (store.marked).
  const idle = $derived(store.photos.length === 0)

  function key(e: KeyboardEvent) {
    // Esc не должен снимать выделение и закрывать окна под полем: свернуть или очистить здесь.
    if (e.key === 'Escape') {
      e.preventDefault()
      e.stopPropagation()
      void nameSearch.escape()
    } else if (e.key === 'Enter') {
      nameSearch.commit()
    } else if (keymap.index.get(eventChord(e) ?? '')?.some((c) => c.id === 'findPhotos')) {
      // Ctrl+F в самом поле: клавиши программы в полях молчат, поиск браузера (стенд) не нужен — выделяем набранное.
      e.preventDefault()
      e.stopPropagation()
      nameSearch.field?.select()
    }
  }

  function reset() {
    nameSearch.clear()
    nameSearch.field?.focus()
  }
</script>

<span class="ns" data-guide="name-search">
  {#if nameSearch.held}
    <input
      bind:this={nameSearch.field}
      class="q"
      type="search"
      maxlength="2000"
      placeholder="Имя или номера"
      aria-label="Поиск по имени файла"
      title="Имя файла или номера кадров: 798, 801, IMG_0805"
      value={nameSearch.q}
      oninput={(e) => nameSearch.type(e.currentTarget.value)}
      onkeydown={key}
      onblur={() => nameSearch.blur()}
    />
    {#if nameSearch.active}
      <span class="found" role="status">
        Найдено {store.visible.length} из {store.marked.length}{#if nameSearch.missText.text}<span
            class="miss"
            title={nameSearch.missText.title}>&nbsp;{nameSearch.missText.text}</span
          >{/if}{#if nameSearch.hiddenText.text}<span class="miss" title={nameSearch.hiddenText.title}
            >&nbsp;{nameSearch.hiddenText.text}</span
          >{/if}
      </span>
      <button class="ghost reset" type="button" title="Сбросить поиск" aria-label="Сбросить поиск" onclick={reset}>
        <Icon name="x" size={12} />
      </button>
    {/if}
  {:else}
    <button
      bind:this={nameSearch.button}
      class="sbtn"
      type="button"
      disabled={idle}
      title={keymap.tip('Найти фото по имени', 'findPhotos')}
      aria-label="Найти фото по имени"
      onclick={() => void nameSearch.focus()}
    >
      <Icon name="search" size={16} />
    </button>
  {/if}
</span>

<style>
  /* Корень живёт и в строке панели, и в окне «⋯»: не переносится, без внешних отступов */
  .ns {
    display: inline-flex;
    align-items: center;
    flex-wrap: nowrap;
    white-space: nowrap;
    gap: 4px;
  }
  .sbtn {
    width: 30px;
    padding: 0;
    display: grid;
    place-items: center;
  }
  .q {
    width: 160px;
    height: 30px;
    padding: 0 8px;
    font: inherit;
    color: var(--text);
    background: #3a3a3e;
    border: 1px solid var(--line);
    border-radius: 6px;
    user-select: text;
  }
  .q:focus-visible {
    outline: 2px solid var(--accent);
    outline-offset: 0;
  }
  /* Своя ✕ ниже; родной крестик поля не нужен */
  .q::-webkit-search-cancel-button {
    appearance: none;
  }
  /* Как кнопка фильтра, когда он действует: поиск сужает ленту, это видно */
  .found {
    display: inline-flex;
    align-items: center;
    height: 30px;
    padding: 0 8px;
    border-radius: 6px;
    background: var(--accent);
    color: #1e1e1e;
    font-weight: 600;
  }
  .miss {
    font-weight: 400;
  }
  .reset {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 26px;
    height: 26px;
    padding: 0;
  }
</style>
