<script lang="ts">
  import { colorWords, eyeHint } from '../../base/color/colorwords'
  import { eyedrop } from '../../base/color/eyedrop.svelte'
  import { store } from '../../base/core/store.svelte'

  // Полоса режима «Взять цвет с фото» по образцу StraightenBar: подсказка по центру, «Отмена» справа.
  // Показывает Stage, пока режим включён.
  const hint = $derived(eyeHint(colorWords(store.view.settings, eyedrop.sheet)))
  const stop = (e: PointerEvent) => e.stopPropagation()
</script>

<div class="bar" role="toolbar" aria-label="Режим «Взять цвет с фото»">
  <div class="group">
    <span class="tag">{hint}</span>
    <span class="sep"></span>
    <button class="done" onpointerdown={stop} onclick={() => eyedrop.cancel()} title="Выйти из режима (Esc)">Отмена</button>
  </div>
</div>

<style>
  .bar {
    position: relative;
    height: 32px; /* = BAR в inspector/panels.ts */
  }
  .group {
    position: absolute;
    left: 50%;
    top: 0;
    bottom: 0;
    transform: translateX(-50%);
    display: flex;
    align-items: center;
    gap: 2px;
    padding: 0 6px;
    white-space: nowrap;
  }
  .tag {
    padding: 0 6px;
    font-size: 12px;
    font-weight: 600;
    color: var(--accent);
  }
  .sep {
    width: 1px;
    height: 16px;
    margin: 0 4px;
    background: var(--line);
  }
  button {
    height: 26px;
    padding: 0 7px;
    display: flex;
    align-items: center;
    background: transparent;
    color: #cfcfd4;
    border-radius: 4px;
    font-size: 12px;
  }
  .done {
    margin-left: 4px;
    background: var(--accent);
    color: #1e1e1e;
    font-weight: 600;
  }
  .done:hover:not(:disabled) {
    background: #ffe53a;
  }
</style>
