<script lang="ts">
  import { autoAskText, gridAct, keepText } from '../../panels/layouts/gridact.svelte'
  import Modal from '../../base/modal/Modal.svelte'

  // Вопрос перед возвратом сетки к «Авто» (спека 008): места сбросятся. По умолчанию — «Отмена».
  // Открыт, пока gridAct.autoAsk не null; ответ — gridAct.confirmAuto / cancelAuto.
  // Тот же вопрос у ручной виньетки (смена шаблона, «Вернуть авто»): тексты — autoAskText.
  const t = $derived(autoAskText(gridAct.autoAsk?.vig))
  // 098: галочка про свой кадр — у страницы и только если правленые фото есть (у виньетки crops = 0)
  const crops = $derived(gridAct.autoAsk?.crops ?? 0)
</script>

{#if gridAct.autoAsk}
  <Modal title={t.title} onclose={() => gridAct.cancelAuto()}>
    <p class="ask">{t.ask}</p>
    {#if crops > 0}
      <label class="keep">
        <input type="checkbox" checked={gridAct.autoAsk?.keep} onchange={(e) => gridAct.setKeep(e.currentTarget.checked)} />
        {keepText(crops)}
      </label>
    {/if}
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
  .keep {
    display: flex;
    align-items: center;
    gap: 8px;
    margin: 6px 0 2px;
    font-size: 13px;
  }
  .actions {
    display: flex;
    justify-content: flex-end;
    gap: 8px;
    margin-top: 12px;
  }
</style>
