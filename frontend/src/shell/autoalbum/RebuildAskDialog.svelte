<script lang="ts">
  import Modal from '../../base/modal/Modal.svelte'
  import { ALBUM_ASK_TITLE } from './autoalbum'
  import { rebuildAsk } from './rebuildask.svelte'

  // Вопрос перед первой пересборкой всего альбома (спека 107): раскладки голых разворотов сменятся. По умолчанию — «Отмена».
  // Образец — AutoAskDialog («Вернуть авто?»); цепочка «Пересобрать…» уже подтверждена и вопроса не задаёт.
</script>

{#if rebuildAsk.ask}
  <Modal title={ALBUM_ASK_TITLE} onclose={() => rebuildAsk.cancel()}>
    {#each rebuildAsk.ask.lines as line (line)}
      <p class="ask">{line}</p>
    {/each}
    <div class="actions">
      <button class="ghost" data-autofocus onclick={() => rebuildAsk.cancel()}>Отмена</button>
      <button class="primary" onclick={() => rebuildAsk.confirm()}>Пересобрать</button>
    </div>
  </Modal>
{/if}

<style>
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
