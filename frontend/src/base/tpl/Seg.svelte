<script module lang="ts">
  import type { IconName } from '../ui/Icon.svelte'

  export interface SegItem<T> {
    id: T
    label: string
    title?: string
    off?: boolean
    aria?: string // aria-label пункта (значок или глиф без слов); иначе имя — подпись
    icon?: IconName // значок перед подписью
  }
</script>

<script lang="ts" generics="T extends string | number">
  import type { Snippet } from 'svelte'
  import Icon from '../ui/Icon.svelte'
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
    grow = false,
    cols = 0,
    big = false,
    action = false,
    keepFocus = false,
    iconSize = 16,
    item,
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
    grow?: boolean // тянуться на остаток ряда (flex-родитель), пункты поровну
    cols?: number // сетка в N столбцов вместо одного ряда (пункты переносятся строками)
    big?: boolean // кегль 14 — для глифов (▭ ▢ ◯, Ж, К)
    action?: boolean // не переключатель, а ряд действий: без aria-pressed, с наведением
    keepFocus?: boolean // mousedown не отнимает фокус (курсор остаётся в поле текста)
    iconSize?: number
    item?: Snippet<[SegItem<T>]> // своё содержимое пункта вместо подписи
  } = $props()

  const on = (id: T) => segOn(id, value, values)

  function pick(it: SegItem<T>) {
    if (off || it.off) return
    onpick(it.id)
  }
</script>

<div
  class="seg"
  class:card={look === 'card'}
  class:grow
  class:big
  class:act={action}
  class:grid={cols > 0}
  style:grid-template-columns={cols > 0 ? `repeat(${cols}, 1fr)` : undefined}
  role={tabs ? 'tablist' : 'group'}
  aria-label={label}
>
  {#each items as it (it.id)}
    {@const grey = off || !!it.off}
    <button
      type="button"
      role={tabs ? 'tab' : undefined}
      class:on={on(it.id)}
      class:off={grey}
      class:ic={!!it.icon}
      aria-selected={tabs ? on(it.id) : undefined}
      aria-pressed={tabs || action ? undefined : on(it.id)}
      aria-disabled={grey ? 'true' : undefined}
      aria-label={it.aria}
      title={(off ? offTitle : undefined) ?? it.title}
      onmousedown={keepFocus ? (e) => e.preventDefault() : undefined}
      onclick={() => pick(it)}
      >{#if it.icon}<Icon name={it.icon} size={iconSize} />{/if}{#if item}{@render item(it)}{:else}{it.label}{/if}</button
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
  /* Вид карточки справа (образец — переключатель «Авто · Ручная» в InspectorSpread): на всю ширину, высота 26, без тёмной подложки. */
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
  /* Пункт со значком: значок и подпись в ряд. */
  button.ic {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 6px;
  }
  /* Сетка (LayerRow 2×2): шов 1px вместо поля слева у соседа. */
  .seg.grid {
    display: grid;
    gap: 1px;
  }
  .grid button + button {
    margin-left: 0;
  }
  /* На остаток ряда (пропорция портрета в VigDialog, «Форма» в карточке): пункты поровну, без переноса. */
  .seg.grow {
    flex: 1 1 auto;
    min-width: 0;
    flex-wrap: nowrap;
  }
  .grow button {
    flex: 1 1 0;
    min-width: 0;
    padding: 0 6px;
  }
  .big button {
    font-size: 14px;
    line-height: 1;
  }
  /* Ряд действий: подсветка наведения (в card у пунктов фон задан явно). */
  .act button:hover:not(.off) {
    background: #444449;
    color: #fff;
  }
  button.off {
    opacity: 0.45;
    cursor: default;
  }
</style>
