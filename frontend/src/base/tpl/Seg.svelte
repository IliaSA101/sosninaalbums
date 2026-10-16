<script module lang="ts">
  export interface SegItem<T> {
    id: T
    label: string
    title?: string
    off?: boolean
  }
</script>

<script lang="ts" generics="T extends string | number">
  // Сегменты «один из N»: единственная копия CSS `.seg` (050). Серый пункт — aria-disabled + .off,
  // а не disabled: на disabled-кнопке нет подсказки (образец — лента значков EditRail).
  let {
    items,
    value,
    onpick,
    label,
    tabs = false,
    off = false,
    offTitle,
  }: {
    items: readonly SegItem<T>[]
    value: T
    onpick: (id: T) => void
    label: string // aria-label группы
    tabs?: boolean // вкладки: role tab + aria-selected; иначе группа кнопок с aria-pressed
    off?: boolean // весь сегмент серый
    offTitle?: string
  } = $props()

  function pick(it: SegItem<T>) {
    if (off || it.off) return
    onpick(it.id)
  }
</script>

<div class="seg" role={tabs ? 'tablist' : 'group'} aria-label={label}>
  {#each items as it (it.id)}
    {@const grey = off || !!it.off}
    <button
      type="button"
      role={tabs ? 'tab' : undefined}
      class:on={value === it.id}
      class:off={grey}
      aria-selected={tabs ? value === it.id : undefined}
      aria-pressed={tabs ? undefined : value === it.id}
      aria-disabled={grey ? 'true' : undefined}
      title={(off ? offTitle : undefined) ?? it.title}
      onclick={() => pick(it)}>{it.label}</button
    >
  {/each}
</div>

<style>
  .seg {
    display: flex;
    flex: none;
    gap: 1px;
    padding: 2px;
    background: #1b1b1d;
    border-radius: 7px;
  }
  button {
    height: 24px;
    min-width: 28px;
    padding: 0 8px;
    font-size: 12px;
    background: transparent;
    border-radius: 5px;
    white-space: nowrap;
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
