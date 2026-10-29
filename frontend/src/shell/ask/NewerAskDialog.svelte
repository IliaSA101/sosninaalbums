<script lang="ts">
  import Modal from '../../base/modal/Modal.svelte'
  import { newerAsk } from './newerask.svelte'
  import { newerAskText } from './newerask'

  // Альбом (или черновик) сохранён более новой версией (112). Правки не было: «Закрыть», × и Esc просто закрывают.
  // Устроен как CopyAskDialog: безопасная кнопка слева и в фокусе, главная справа — «Открыть в X», если новая
  // версия уже есть на компьютере, иначе «Скачать».
  const a = $derived(newerAsk.cur)
  const t = $derived(a ? newerAskText(a) : null)
</script>

{#if a && t}
  <Modal title={t.title} onclose={() => newerAsk.close()} width={540}>
    <p class="ask">{t.lead}</p>
    <p class="note">{t.next}</p>
    <div class="actions">
      <button class="ghost" data-autofocus onclick={() => newerAsk.close()}>{t.close}</button>
      {#if t.open}
        <button class="ghost" onclick={() => newerAsk.download()}>{t.download}</button>
        <button class="primary" disabled={newerAsk.busy} onclick={() => newerAsk.openInNewer()}>{t.open}</button>
      {:else}
        <button class="primary" onclick={() => newerAsk.download()}>{t.download}</button>
      {/if}
    </div>
  </Modal>
{/if}

<style>
  .ask {
    margin: 4px 0 6px;
    line-height: 1.45;
    font-size: 14px;
  }
  .note {
    margin: 0 0 6px;
    font-size: 13px;
    color: var(--muted);
  }
  .actions {
    display: flex;
    flex-wrap: wrap;
    justify-content: flex-end;
    gap: 8px;
    margin-top: 12px;
  }
</style>
