<script module lang="ts">
  export interface SideItem<T> {
    id: T
    label: string
    count?: number
    title?: string
    off?: boolean
  }
</script>

<script lang="ts" generics="T extends string | number">
  // Переключатель «источник слева» (050): вертикальный сегмент, у пункта число. Он же — список тем окна фонов
  // и стикеров (048). Серый пункт — как у Seg: aria-disabled + .off, подсказка с причиной остаётся.
  let {
    items,
    value,
    onpick,
    label,
    scroll = false,
  }: {
    items: readonly SideItem<T>[]
    value: T
    onpick: (id: T) => void
    label: string
    scroll?: boolean // высоту задаёт родитель, список прокручивается (048)
  } = $props()
</script>

<div class="vseg" class:scroll role="group" aria-label={label}>
  {#each items as it (it.id)}
    <button
      type="button"
      class:on={value === it.id}
      class:off={it.off}
      aria-pressed={value === it.id}
      aria-disabled={it.off ? 'true' : undefined}
      title={it.title}
      onclick={() => !it.off && onpick(it.id)}
    >
      <span class="name">{it.label}</span>{#if it.count !== undefined}<em>{it.count}</em>{/if}
    </button>
  {/each}
</div>

<style>
  .vseg {
    display: flex;
    flex-direction: column;
    gap: 2px;
    padding: 2px;
    background: #1b1b1d;
    border-radius: 7px;
  }
  .vseg.scroll {
    height: 100%;
    overflow-y: auto;
  }
  button {
    flex: none;
    height: 24px;
    padding: 0 8px;
    font-size: 12px;
    text-align: left;
    background: transparent;
    border-radius: 5px;
    white-space: nowrap;
    display: flex;
    align-items: center;
  }
  .name {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  em {
    flex: none;
    margin-left: 4px;
    font-style: normal;
    opacity: 0.65;
  }
  button.on {
    background: var(--accent);
    color: #1e1e1e;
    font-weight: 600;
  }
  button.off {
    opacity: 0.45;
    cursor: default;
  }
</style>
