<script lang="ts">
  import { bgmode } from './bgmode.svelte'
  import { store } from '../../base/core/store.svelte'
  import { SCOPE_BAR } from './bgmode'
  import { fitBackdrop, flipBackdrop } from './backdropact'
  import Icon from '../../base/ui/Icon.svelte'

  // Полоса режима фона (спека 009, design-backdrop §6.2) вместо «Раскладка: …»: область, dpi, зеркало,
  // «По области», «Убрать», «Готово». Видна всегда, пока режим включён.
  const b = $derived(bgmode.view)
  const c = $derived(bgmode.cur)
  const stop = (e: PointerEvent) => e.stopPropagation()
</script>

{#if b && c}
  <div class="bar" role="toolbar" aria-label="Режим фона">
    <div class="group">
      <span class="tag">Фон: {SCOPE_BAR[c.scope]}</span>
      {#if b.dpi > 0}<span class="dpi" class:low={b.dpi < store.view.minDPI} title="Разрешение фото фона в печати"
          >{Math.round(b.dpi)} dpi</span
        >{/if}
      <span class="sep"></span>
      <button
        class:on={b.flipH}
        aria-pressed={b.flipH}
        onpointerdown={stop}
        onclick={() => flipBackdrop(c.spread, c.scope, 'h')}
        title="Зеркало по горизонтали"><Icon name="flip-h" size={14} />↔</button
      >
      <button
        class:on={b.flipV}
        aria-pressed={b.flipV}
        onpointerdown={stop}
        onclick={() => flipBackdrop(c.spread, c.scope, 'v')}
        title="Зеркало по вертикали"><Icon name="flip-v" size={14} />↕</button
      >
      <button onpointerdown={stop} onclick={() => fitBackdrop(c.spread, c.scope)} title="Рамка во всю область, без поворота, кадр сброшен"
        >По области</button
      >
      <button onpointerdown={stop} onclick={() => bgmode.remove()} title="Убрать фото фона (Del)">Убрать</button>
      <button class="done" onpointerdown={stop} onclick={() => bgmode.exit()} title="Выйти из режима фона">Готово (Esc)</button>
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
  .dpi {
    padding-right: 4px;
    font-size: 12px;
    color: var(--muted);
  }
  .dpi.low {
    color: #d9382c;
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
    gap: 3px;
    background: transparent;
    color: #cfcfd4;
    border-radius: 4px;
    font-size: 12px;
  }
  button.on {
    color: var(--accent);
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
