<script lang="ts">
  import { autoAskText, gridAct } from '../../panels/layouts/gridact.svelte'
  import Modal from '../../base/ui/Modal.svelte'

  // Вопрос перед возвратом сетки к «Авто» (спека 008): места сбросятся. По умолчанию — «Отмена».
  // Открыт, пока gridAct.autoAsk не null; ответ — gridAct.confirmAuto / cancelAuto.
  // Тот же вопрос у ручной виньетки (смена шаблона, «Вернуть авто»): тексты — autoAskText.
  const t = $derived(autoAskText(gridAct.autoAsk?.vig))
</script>

{#if gridAct.autoAsk}
  <Modal title={t.title} onclose={() => gridAct.cancelAuto()}>
    <p class="ask">{t.ask}</p>
    <div class="actions">
      <button class="ghost" data-autofocus onclick={() => gridAct.cancelAuto()}>Отмена</button>
      <button class="primary" onclick={() => gridAct.confirmAuto()}>{t.ok}</button>
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
