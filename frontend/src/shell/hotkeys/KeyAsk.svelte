<script lang="ts">
  import Modal from '../../base/modal/Modal.svelte'
  import { keysUi } from './keysui.svelte'

  // «Заменить?» (074): сочетание уже у другой команды той же области. «Отмена» слева и с фокусом, главная справа —
  // как вопрос автосборки (074-27): Enter сразу после записи не должен молча отнять клавишу.
  const a = $derived(keysUi.ask)
</script>

{#if a}
  <Modal title="Заменить?" onclose={() => keysUi.cancelAsk()}>
    <p class="ask">{keysUi.askText(a)}</p>
    <div class="actions">
      <button class="ghost" data-autofocus onclick={() => keysUi.cancelAsk()}>Отмена</button>
      <button class="primary" onclick={() => keysUi.confirmAsk()}>Заменить</button>
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
