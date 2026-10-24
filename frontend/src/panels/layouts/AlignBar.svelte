<script lang="ts">
  import { align } from './align.svelte'
  import { ALIGN_OPS, ALIGN_TO, alignBlock, alignToBlock } from './align'
  import Icon from '../../base/ui/Icon.svelte'

  // Полоса выравнивания (спека 010 U7) вместо «Раскладка: …», пока есть выделение («Полный»; решает LayoutBar).
  // Неактивное — aria-disabled, а не disabled: у disabled нет подсказки «Выделите N объектов».
  // data-text-ui: щелчок не снимает выделение одиночного текста.
  const n = $derived(align.ids.length)
  const stop = (e: PointerEvent) => e.stopPropagation()
</script>

<div class="group" role="toolbar" aria-label="Выравнивание" data-text-ui data-guide="align-bar">
  <span class="tag">Выровнять:</span>
  {#each ALIGN_OPS as a (a.op)}
    {@const why = alignBlock(a.op, align.to, n)}
    {#if a.op === 'distH'}<span class="sep"></span><span class="lbl">Распределить:</span>{/if}
    <button
      class="ico"
      class:off={!!why}
      aria-disabled={!!why}
      aria-label={a.label}
      title={why ? `${a.label}: ${why.toLowerCase()}` : a.label}
      onpointerdown={stop}
      onclick={() => !why && align.run(a.op)}><Icon name={a.icon} /></button
    >
  {/each}
  <span class="sep"></span>
  <span class="lbl">Относительно:</span>
  {#each ALIGN_TO as t (t.to)}
    {@const why = alignToBlock(t.to, n)}
    <button
      class="to"
      class:on={align.to === t.to}
      class:off={!!why}
      aria-pressed={align.to === t.to}
      aria-disabled={!!why}
      title={why ?? t.hint}
      onpointerdown={stop}
      onclick={() => !why && align.setTo(t.to)}>{t.label}</button
    >
  {/each}
</div>

<style>
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
  .lbl {
    padding: 0 4px 0 2px;
    font-size: 12px;
    color: var(--muted);
  }
  .sep {
    width: 1px;
    height: 16px;
    margin: 0 4px;
    background: var(--line);
  }
  button {
    height: 26px;
    padding: 0;
    display: grid;
    place-items: center;
    background: transparent;
    color: #cfcfd4;
    border-radius: 4px;
  }
  .ico {
    width: 26px;
  }
  .ico.off {
    opacity: 0.4;
    cursor: default;
  }
  .ico.off:hover {
    background: transparent;
  }
  .to {
    padding: 0 7px;
    font-size: 12px;
  }
  .to.off {
    opacity: 0.4;
    cursor: default;
  }
  .to.off:hover {
    background: transparent;
  }
  .to.on {
    background: var(--accent);
    color: #1e1e1e;
    font-weight: 600;
  }
  .to.on:hover {
    background: #ffe53a;
  }
</style>
