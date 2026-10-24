<script module lang="ts">
  export interface SegItem<T> {
    id: T
    label: string
    title?: string
    off?: boolean
  }
</script>

<script lang="ts" generics="T extends string | number">
  import { segOn } from './tplfilter'

  // Сегменты «один из N» (или набор values — нажато несколько, 065): единственная копия CSS `.seg` (050). Серый пункт — aria-disabled + .off,
  // а не disabled: на disabled-кнопке нет подсказки (образец — лента значков EditRail).
  let {
    items,
    value,
    values,
    onpick,
    label,
    tabs = false,
    look = 'pill',
    off = false,
    offTitle,
  }: {
    items: readonly SegItem<T>[]
    value?: T // выбранный пункт (один)
    values?: readonly T[] // набор: нажато несколько (065), value тогда не используется
    onpick: (id: T) => void
    label: string // aria-label группы
    tabs?: boolean // вкладки: role tab + aria-selected; иначе группа кнопок с aria-pressed
    look?: 'pill' | 'card' // card — переключатель карточки справа («Авто · Ручная»): кнопки на всю ширину
    off?: boolean // весь сегмент серый
    offTitle?: string
  } = $props()

  const on = (id: T) => segOn(id, value, values)

  function pick(it: SegItem<T>) {
    if (off || it.off) return
    onpick(it.id)
  }
</script>

<div class="seg" class:card={look === 'card'} role={tabs ? 'tablist' : 'group'} aria-label={label}>
  {#each items as it (it.id)}
    {@const grey = off || !!it.off}
    <button
      type="button"
      role={tabs ? 'tab' : undefined}
      class:on={on(it.id)}
      class:off={grey}
      aria-selected={tabs ? on(it.id) : undefined}
      aria-pressed={tabs ? undefined : on(it.id)}
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
    flex-wrap: wrap; /* узкое окно: пункты переносятся, прокрутки нет */
    max-width: 100%;
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
  /* Вид карточки справа (InspectorSpread .seg): на всю ширину, высота 26, без тёмной подложки. */
  .seg.card {
    gap: 0;
    padding: 0;
    background: transparent;
    border-radius: 5px;
    overflow: hidden;
  }
  .card button {
    flex: 1;
    min-width: 0;
    height: 26px;
    padding: 0 6px;
    border-radius: 0;
    background: #38383c;
    color: #cfcfd4;
  }
  .card button + button {
    margin-left: 1px;
  }
  .card button.on {
    background: var(--accent);
    color: #1e1e1e;
    font-weight: 400;
  }
  button.off {
    opacity: 0.45;
    cursor: default;
  }
</style>
