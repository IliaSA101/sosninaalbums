<script lang="ts">
  import { keymap } from '../../base/keys/keymap.svelte'
  import { store } from '../../base/core/store.svelte'
  import { gallery } from '../gallery/gallery.svelte'
  import Icon from '../../base/ui/Icon.svelte'
  import { MANUAL_TITLE, NO_OTHER, type BarGroup } from './coverbar'

  // Группы «‹ ▦ ›» над задней и лицевой сторонкой (127), образец — группы страниц LayoutBar.
  // Какие группы и где стоят — решает LayoutBar через barGroups (уровень «Варианты», зум, выравнивание).
  let { groups }: { groups: BarGroup[] } = $props()

  const stop = (e: PointerEvent) => e.stopPropagation()
  const isOpen = (scope: string) => gallery.cur?.scope === scope && gallery.cur.tab === 'grids' && gallery.cur.spread === store.spread?.id
  // Клавиша Пробела листает только лицевую; у задней её в подсказке нет.
  const tip = (label: string, scope: string, n: number) => (scope === 'right' ? keymap.tip(label, 'spaceDown', n) : label)

  // ‹ › : решение (шаг, ▦ или тост) принимает gallery.coverStep по виду Go.
  const arrow = (g: BarGroup, delta: number) => void gallery.coverStep(g.side.scope, delta)
</script>

{#each groups as g (g.side.scope)}
  <div class="group" style:left="{g.left}%" role="group" aria-label="{g.side.label} сторонка">
    <button
      class="arrow"
      disabled={g.still}
      onpointerdown={stop}
      onclick={() => arrow(g, -1)}
      title={g.still ? NO_OTHER : tip(`Предыдущая раскладка ${g.who}`, g.side.scope, 1)}
      aria-label="Предыдущая раскладка {g.who}"><Icon name="chevron-left" /></button
    >
    <button
      class="grid"
      class:active={isOpen(g.side.scope)}
      onpointerdown={stop}
      onclick={() => gallery.open(g.side.scope, 'grids')}
      title="Раскладки {g.who}"
      aria-label="Раскладки {g.who}"
      aria-pressed={isOpen(g.side.scope)}><Icon name="grid" size={14} /></button
    >
    {#if g.manual}<span class="tag" title={MANUAL_TITLE}>· Ручная ·</span>{/if}
    <button
      class="arrow"
      disabled={g.still}
      onpointerdown={stop}
      onclick={() => arrow(g, 1)}
      title={g.still ? NO_OTHER : tip(`Следующая раскладка ${g.who}`, g.side.scope, 0)}
      aria-label="Следующая раскладка {g.who}"><Icon name="chevron-right" /></button
    >
  </div>
{/each}

<style>
  /* Те же правила, что у групп страниц в LayoutBar: появляются при наведении на обложку (.visible у полосы). */
  .group {
    position: absolute;
    top: 0;
    bottom: 0;
    transform: translateX(-50%);
    display: flex;
    align-items: center;
    gap: 2px;
    padding: 0 6px;
    opacity: 0;
    visibility: hidden;
    transition:
      opacity 0.15s,
      visibility 0s linear 0.15s;
  }
  :global(.layout-bar.visible) .group,
  :global(.layout-bar[data-spot]) .group {
    opacity: 1;
    visibility: visible;
    transition:
      opacity 0.15s,
      visibility 0s;
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
  .arrow {
    width: 22px;
  }
  .grid {
    width: 28px;
  }
  .grid.active {
    background: var(--accent);
    color: #1e1e1e;
  }
  .grid.active:hover:not(:disabled) {
    background: #ffe53a;
  }
  .tag {
    padding: 0 0 0 6px;
    white-space: nowrap;
    font-size: 12px;
    font-weight: 600;
    color: var(--accent);
  }
  /* Стрелки появляются над своей группой; место держат, чтобы ▦ не сдвигался. */
  .arrow {
    visibility: hidden;
  }
  :global(.layout-bar.visible) .group:hover .arrow {
    visibility: visible;
  }
</style>
