<script lang="ts">
  import Modal from '../../base/modal/Modal.svelte'
  import type { OwnAsk } from './albumown'

  // Вопрос про развороты со своим цветом или фоном (115): правки ещё не было. Устроен как DropAskDialog (042):
  // безопасный выбор слева и в фокусе, главный справа; ×, Esc и щелчок мимо — правки нет (onclose).
  let { ask, onkeep, onall, onclose }: { ask: OwnAsk; onkeep: () => void; onall: () => void; onclose: () => void } = $props()
</script>

<Modal title={ask.title} {onclose} width={540}>
  <p class="ask">{ask.text}</p>
  {#if ask.note}<p class="note">{ask.note}</p>{/if}
  <div class="actions">
    <button class="ghost" data-autofocus onclick={onkeep}>{ask.keep}</button>
    <button class="primary" onclick={onall}>{ask.all}</button>
  </div>
</Modal>

<style>
  .ask {
    margin: 4px 0 6px;
    line-height: 1.45;
    font-size: 14px;
  }
  .note {
    margin: 0 0 6px;
    font-size: 12px;
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
