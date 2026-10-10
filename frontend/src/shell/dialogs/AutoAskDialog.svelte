<script lang="ts">
  import { gridAct } from '../../panels/layouts/gridact.svelte'
  import Modal from '../../base/ui/Modal.svelte'

  // Вопрос перед возвратом сетки к «Авто» (спека 008): места сбросятся. По умолчанию — «Отмена».
  // Открыт, пока gridAct.autoAsk не null; ответ — gridAct.confirmAuto / cancelAuto.
</script>

{#if gridAct.autoAsk}
  <Modal title="Вернуть авто?" onclose={() => gridAct.cancelAuto()}>
    <p class="ask">Ваши правки мест сбросятся. Вернуть авто?</p>
    <div class="actions">
      <button class="ghost" data-autofocus onclick={() => gridAct.cancelAuto()}>Отмена</button>
      <button class="primary" onclick={() => gridAct.confirmAuto()}>Вернуть авто</button>
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
