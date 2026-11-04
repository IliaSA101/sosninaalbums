<script lang="ts">
  import { answerDrop, cancelDrop, dropAsk, dropAskText } from '../../base/drag/dropgrid.svelte'
  import { store } from '../../base/core/store.svelte'
  import { unitOf } from '../../base/geom/sheet'
  import Modal from '../../base/modal/Modal.svelte'

  // Вопрос после броска на ручной разворот (042): мест меньше, чем фото. Правки ещё не было;
  // ответ — второй вызов Go с rest, × и Esc — бросок отменён. Устроен как AutoAskDialog: безопасный выбор слева и в фокусе.
  // Подписи кнопок длинные (183 + 284 px): окно 540 px, как у «Автосборки», а на узком экране кнопки переносятся.
  const a = $derived(dropAsk.cur)
  const t = $derived(a ? dropAskText(a, unitOf(store.view.settings)) : null)
</script>

{#if a && t}
  <Modal title={t.title} onclose={cancelDrop} width={540}>
    <p class="ask">{t.ask}</p>
    <div class="actions">
      <button class="ghost" data-autofocus onclick={() => answerDrop('skip')}>{t.skip}</button>
      <button class="primary" onclick={() => answerDrop('add')}>{t.add}</button>
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
    flex-wrap: wrap;
    justify-content: flex-end;
    gap: 8px;
    margin-top: 12px;
  }
</style>
