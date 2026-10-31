<script lang="ts">
  import { store } from '../../base/core/store.svelte'
  import { BORDER_NOTE, hasBorders, sheetIn, unitOf } from '../../base/geom/sheet'
  import Modal from '../../base/modal/Modal.svelte'
  import { NAME_MAX, SAVE_HINT, deleteText } from './vigown'
  import { vigOwn } from './vigown.svelte'

  // Окна своих шаблонов виньетки (038): «Сохранить как шаблон», «Переименовать», «Удалить шаблон?».
  // Повторяют окна сеток (GridGallery): поле с выделением, Enter — «Сохранить», Esc и фон — отмена.
  const ask = $derived(vigOwn.ask)
  const close = () => vigOwn.close()
  const commit = () => void vigOwn.commit()

  // Enter в поле — как кнопка окна.
  function enter(e: KeyboardEvent) {
    if (e.key === 'Enter') {
      e.preventDefault()
      commit()
    }
  }
  // Имя при переименовании выделено целиком: набор заменяет его.
  const selectAll = (e: FocusEvent) => (e.currentTarget as HTMLInputElement).select()
</script>

{#if ask?.kind === 'save'}
  <Modal title="Сохранить как шаблон" onclose={close}>
    <label class="field"
      >Название
      <input type="text" maxlength={NAME_MAX} data-autofocus placeholder="Шаблон N" bind:value={vigOwn.name} onkeydown={enter} /></label
    >
    <p class="hint">{SAVE_HINT}</p>
    {#if hasBorders(sheetIn(store.view, ask.spread))}<p class="hint">{BORDER_NOTE}</p>{/if}
    <div class="actions">
      <button class="ghost" onclick={close}>Отмена</button>
      <button class="primary" onclick={commit}>Сохранить</button>
    </div>
  </Modal>
{:else if ask?.kind === 'rename'}
  <Modal title="Переименовать шаблон" onclose={close}>
    <input
      type="text"
      maxlength={NAME_MAX}
      data-autofocus
      bind:value={vigOwn.name}
      onkeydown={enter}
      onfocus={selectAll}
      aria-label="Имя шаблона"
    />
    <div class="actions">
      <button class="ghost" onclick={close}>Отмена</button>
      <button class="primary" onclick={commit}>Сохранить</button>
    </div>
  </Modal>
{:else if ask?.kind === 'delete'}
  <Modal title="Удалить шаблон?" onclose={close}>
    <p>{deleteText(ask.name, unitOf(store.view.settings).cap)}</p>
    <div class="actions">
      <button class="ghost" onclick={close}>Отмена</button>
      <button class="primary" onclick={commit}>Удалить</button>
    </div>
  </Modal>
{/if}

<style>
  input[type='text'] {
    width: 100%;
    height: 30px;
    padding: 0 8px;
    font: inherit;
    background: #3a3a3e;
    color: var(--text);
    border: 1px solid var(--line);
    border-radius: 6px;
    user-select: text;
  }
  .field {
    display: flex;
    flex-direction: column;
    gap: 4px;
    margin-bottom: 10px;
    color: var(--muted);
    font-size: 12px;
  }
  p {
    margin: 0 0 10px;
    line-height: 1.4;
  }
  .hint {
    color: var(--muted);
    font-size: 12px;
  }
  .actions {
    display: flex;
    justify-content: flex-end;
    gap: 8px;
    margin-top: 12px;
  }
</style>
