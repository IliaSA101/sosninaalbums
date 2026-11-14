<script lang="ts">
  import Modal from '../../base/modal/Modal.svelte'
  import { applyTitle } from './looks'
  import { looks } from './looks.svelte'
  import LookForm from './LookForm.svelte'

  // Окно «Применить макет «…»» поверх крупного просмотра (163-4 а), образец — вопрос 067 «Поставить разворот?»:
  // форма 131, строка предупреждения (если 131 спрашивает), «Отмена» · «Применить». «Отмена», Esc и ✕ — назад в просмотр.
  // Свежий план изменил предупреждение — окно покажет новый текст и дождётся второго нажатия (163-27, looks.apply).
  const name = $derived(looks.item?.name ?? '')

  // Enter — «Применить»; в поле числа он только принимает число (его обработчик гасит событие), на кнопке — её щелчок.
  // Автоповтор Enter гасим сами: иначе он нажал бы «Применить», на которую фокус перешёл с «Применить…» просмотра.
  function onkey(e: KeyboardEvent) {
    if (e.key !== 'Enter') return
    if (e.repeat) {
      e.preventDefault()
      return
    }
    if (e.defaultPrevented || !(e.target instanceof Element) || e.target.closest('input, select, textarea, button, [role="radio"]')) return
    e.preventDefault()
    void looks.apply()
  }
</script>

<Modal title={applyTitle(name)} wide onclose={() => looks.dismiss()} {onkey}>
  <LookForm />
  {#if looks.warn}<p class="warn" role="status">{looks.warn}</p>{/if}
  <div class="actions">
    <button class="ghost" onclick={() => looks.dismiss()}>Отмена</button>
    <button class="primary" data-autofocus disabled={!looks.ready || looks.busy} onclick={() => looks.apply()}>Применить</button>
  </div>
</Modal>

<style>
  .warn {
    margin: 12px 0 0;
    line-height: 1.4;
  }
  .actions {
    display: flex;
    justify-content: flex-end;
    gap: 8px;
    margin-top: 12px;
  }
</style>
