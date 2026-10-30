<script lang="ts">
  import Modal from '../../base/modal/Modal.svelte'
  import { agentAsk } from './agentask.svelte'
  import { agents } from './agents.svelte'
  import { KEY_PLACES, placesShown, rowKey } from './agentperms'
  import {
    ASK_BAD_HINT,
    ASK_HINT,
    ASK_LINK,
    ASK_NEVER_LINK,
    ASK_PLACE_LINK,
    ASK_PLACE_WHY,
    ASK_WHY,
    CANCEL_LABEL,
    PLACES_NONE_SHORT,
  } from './agentstext'

  // Вопрос агента «Создать альбом по просьбе агента?» / «Добавить папку…» (149; 150: полный путь, «Выбрать другую…», вид edit,
  // причина «Всегда, а тихо нельзя» и ссылка на строку разрешений). Устроен как AutoAskDialog и NewerAskDialog:
  // «Отмена» слева и в фокусе, действие справа. Отличие одно: первые 600 мс кнопки `aria-disabled` — вопрос пришёл не от щелчка
  // человека (149-25). Тексты — из Go (абзацы lines только текстом, без {@html}); имя папки-подсказки — в <bdi>: чужой текст
  // не должен перевернуть соседние слова.
  const a = $derived(agentAsk.cur)
  const armed = $derived(agentAsk.armed)

  $effect(() => agentAsk.watch())

  // Места съёмок для причины «place»: не больше двух путей.
  const placeDirs = $derived(placesShown(agents.info.places).shown)

  // Enter — всегда «Отмена»; пробел — «Отмена», кроме фокуса на «Выбрать папку…»: там это обычное нажатие кнопки (Tab + пробел).
  function key(e: KeyboardEvent) {
    const onOk = e.target instanceof Element && !!e.target.closest('[data-ok]')
    if (e.key === 'Enter' || (e.key === ' ' && !onOk)) {
      e.preventDefault()
      void agentAsk.cancel()
    }
  }
</script>

{#snippet target()}
  {#if a?.path}
    <p class="ask">
      <bdi class="path" data-path>{a.path}</bdi>{#if a.link}
        <span class="note">{ASK_LINK}</span>{/if}
    </p>
  {:else if a?.hint}
    <p class="ask">{ASK_HINT} «<bdi data-hint>{a.hint}</bdi>».</p>
  {/if}
{/snippet}

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
      {#if i === 0}{@render target()}{/if}
    {/each}
    {#if a.lines.length === 0}{@render target()}{/if}
    {#if a.why === 'bad'}
      <p class="ask warn">{ASK_BAD_HINT}</p>
    {:else if a.why === 'place'}
      <p class="ask note">
        {ASK_PLACE_WHY}
        {#each placeDirs as d, i (d)}{i > 0 ? ', ' : ' '}<bdi class="path">{d}</bdi>{:else}{PLACES_NONE_SHORT}{/each}.
        <button class="link" onclick={() => void agents.show(KEY_PLACES)}>{ASK_PLACE_LINK}</button>
      </p>
    {:else if a.why && ASK_WHY[a.why]}
      <p class="ask note">{ASK_WHY[a.why]}</p>
    {:else if a.tool}
      <p class="ask note"><button class="link" onclick={() => void agents.show(rowKey(a.tool))}>{ASK_NEVER_LINK}</button></p>
    {/if}
    <div class="actions">
      <button class="ghost" data-autofocus aria-disabled={!armed} onclick={() => void agentAsk.cancel()}>{CANCEL_LABEL}</button>
      {#if a.alt}<button data-ok aria-disabled={!armed} onclick={() => void agentAsk.other()}>{a.alt}</button>{/if}
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
  .note {
    color: var(--muted);
    font-size: 12px;
  }
  .warn {
    color: #ff8a8a;
  }
  /* Настоящий путь папки: моноширинно, переносится по любому знаку, выделяется для копирования */
  .path {
    padding: 2px 6px;
    background: var(--panel2);
    border-radius: 4px;
    font-family: Consolas, monospace;
    font-size: 13px;
    overflow-wrap: anywhere;
    user-select: text;
  }
  .link {
    all: unset;
    cursor: pointer;
    color: var(--muted);
    font-size: 12px;
    text-decoration: underline dotted;
    text-underline-offset: 3px;
  }
  .link:hover,
  .link:focus-visible {
    text-decoration-style: solid;
  }
  .link:focus-visible {
    outline: 2px solid var(--accent);
    outline-offset: 2px;
  }
  /* Не disabled: фокус остаётся на «Отмена». Выглядит неактивной только эти 600 мс */
  button[aria-disabled='true'] {
    opacity: 0.55;
    cursor: default;
  }
</style>
