<script lang="ts">
  import Modal from '../../base/modal/Modal.svelte'
  import { agentAsk } from './agentask.svelte'
  import { ASK_HINT, CANCEL_LABEL } from './agentstext'

  // Вопрос агента «Создать альбом по просьбе агента?» / «Добавить папку…» (149). Устроен как AutoAskDialog и NewerAskDialog:
  // «Отмена» слева и в фокусе, действие справа. Отличие одно: первые 600 мс кнопки `aria-disabled` — вопрос пришёл не от щелчка
  // человека (149-25). Тексты — из Go (абзацы lines только текстом, без {@html}); имя папки-подсказки — в <bdi>: чужой текст
  // не должен перевернуть соседние слова.
  const a = $derived(agentAsk.cur)
  const armed = $derived(agentAsk.armed)

  $effect(() => agentAsk.watch())

  // Enter — всегда «Отмена»; пробел — «Отмена», кроме фокуса на «Выбрать папку…»: там это обычное нажатие кнопки (Tab + пробел).
  function key(e: KeyboardEvent) {
    const onOk = e.target instanceof Element && !!e.target.closest('[data-ok]')
    if (e.key === 'Enter' || (e.key === ' ' && !onOk)) {
      e.preventDefault()
      void agentAsk.cancel()
    }
  }
</script>

{#if a}
  <Modal
    title={a.title}
    wide
    onclose={() => void agentAsk.cancel()}
    onescape={() => {
      void agentAsk.cancel(true)
      return true
    }}
    onkey={key}
  >
    {#each a.lines as line, i (i)}
      <p class="ask">{line}</p>
      {#if i === 0 && a.hint}<p class="ask">{ASK_HINT} «<bdi data-hint>{a.hint}</bdi>».</p>{/if}
    {/each}
    {#if a.lines.length === 0 && a.hint}<p class="ask">{ASK_HINT} «<bdi data-hint>{a.hint}</bdi>».</p>{/if}
    <div class="actions">
      <button class="ghost" data-autofocus aria-disabled={!armed} onclick={() => void agentAsk.cancel()}>{CANCEL_LABEL}</button>
      <button class="primary" data-ok aria-disabled={!armed} onclick={() => void agentAsk.accept()}>{a.ok}</button>
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
  /* Не disabled: фокус остаётся на «Отмена». Выглядит неактивной только эти 600 мс */
  button[aria-disabled='true'] {
    opacity: 0.55;
    cursor: default;
  }
</style>
