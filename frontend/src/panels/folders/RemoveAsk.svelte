<script lang="ts">
  import Modal from '../../base/modal/Modal.svelte'
  import { foldersUi } from './foldersui.svelte'

  // «Убрать папку?» / «Убрать фото?» (093, 097): как «Убрать из библиотеки?» (FontPicker), но фокус на «Отмена» —
  // фото альбома уходят из ленты.
</script>

{#if foldersUi.ask}
  <!-- data-barpop на всём вопросе (фон, заголовок, ✕, поля окна): щелчок по нему не закрывает окно «Папки» под ним -->
  <div class="wrap" data-barpop="folders-ask">
    <Modal title={foldersUi.ask.title} onclose={() => foldersUi.cancel()}>
      <p class="ask">{foldersUi.ask.text}</p>
      <div class="actions">
        <button type="button" class="ghost" data-autofocus onclick={() => foldersUi.cancel()}>Отмена</button>
        <button type="button" class="primary" onclick={() => foldersUi.confirm()}>Убрать</button>
      </div>
    </Modal>
  </div>
{/if}

<style>
  .wrap {
    display: contents;
  }
  .ask {
    margin: 4px 0 6px;
    line-height: 1.45;
    font-size: 14px;
  }
  .actions {
    display: flex;
    justify-content: flex-end;
    gap: 8px;
    margin-top: 12px;
  }
</style>
