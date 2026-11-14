<script lang="ts">
  import Modal from '../../base/modal/Modal.svelte'
  import { CONSENT_TEXT, CONSENT_TITLE, START_LABEL_ASK } from './packnet'

  // Вопрос «Обратиться к GitHub?» (067 Ф3, 140): первое сетевое действие — «Обновить», «Скачать» набор, «Скачать» ролик.
  // Без согласия программа в сеть не ходит. Порядок кнопок — «Отмена» слева, действие справа. Одно окно для «Наборов» и F1.
  let {
    action,
    start = $bindable(false),
    onconfirm,
    oncancel,
  }: { action: string; start?: boolean; onconfirm: () => void; oncancel: () => void } = $props()
</script>

<Modal title={CONSENT_TITLE} onclose={oncancel} wide>
  <p>{CONSENT_TEXT}</p>
  <label class="chk"><input type="checkbox" bind:checked={start} />{START_LABEL_ASK}</label>
  <div class="actions">
    <button class="ghost" onclick={oncancel}>Отмена</button>
    <button class="primary" data-autofocus onclick={onconfirm}>{action}</button>
  </div>
</Modal>

<style>
  p {
    margin: 0 0 10px;
    line-height: 1.4;
  }
  .chk {
    display: flex;
    align-items: center;
    gap: 6px;
    margin: 0 0 10px;
  }
  .actions {
    display: flex;
    justify-content: flex-end;
    gap: 8px;
    margin-top: 12px;
  }
</style>
