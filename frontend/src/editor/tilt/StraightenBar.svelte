<script lang="ts">
  import { straighten } from './straighten.svelte'

  // Полоса режима «Выпрямить по линии» (спека 045, решение №26) по образцу BackdropBar: подсказка по центру,
  // «Отмена» — кнопка выхода справа. Видна, пока режим включён.
  const stop = (e: PointerEvent) => e.stopPropagation()
</script>

{#if straighten.cur}
  <div class="bar" role="toolbar" aria-label="Режим «Выпрямить по линии»">
    <div class="group">
      <span class="tag">Проведите линию вдоль горизонта или вертикали</span>
      <span class="sep"></span>
      <button class="done" onpointerdown={stop} onclick={() => straighten.exit()} title="Выйти из режима (Esc)">Отмена</button>
    </div>
  </div>
{/if}

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
