<script lang="ts">
  import Icon from '../../base/ui/Icon.svelte'
  import { countParts, removeLabel, removeTitle, rowTitle, type Check, type TreeNode } from './tree'

  // Одна строка дерева «Папки» (093; ветка, добавление и файл отдельных — 097): ▸, флажок с именем, «только», число, ✕.
  // Список плоский — без рекурсии.
  let {
    node,
    depth,
    check,
    checks,
    only,
    open,
    flash,
    sep = false,
    ontoggle,
    onopen,
    ononly,
    onremove,
  }: {
    node: TreeNode
    depth: number
    check: Check
    checks: boolean // флажки есть, когда строк с фото больше одной
    only: boolean // «только» имеет смысл
    open: boolean
    flash: boolean
    sep?: boolean // черта над веткой «Фото отдельно» после папок
    ontoggle: () => void
    onopen: () => void
    ononly: () => void
    onremove: () => void
  } = $props()

  const count = $derived(countParts(node))
  const kids = $derived(node.kids.length > 0)

  // Кнопки внутри окна не начинают перенос ленты.
  const stop = (e: PointerEvent) => e.stopPropagation()

  // «—» — свойство, а не атрибут: ставим его на каждое изменение состояния.
  const mixed = (el: HTMLInputElement) => {
    el.indeterminate = check === 'mixed'
  }

  // Подсвеченная строка (добавлена или уже была) прокручивается в вид.
  const seen = (el: HTMLElement) => {
    if (flash) el.scrollIntoView?.({ block: 'nearest' })
  }
</script>

<li
  class:off={checks && check === 'off'}
  class:flash
  class:sep
  class:missing={node.missing || !!node.bad}
  style:padding-left="{4 + depth * 14}px"
  title={rowTitle(node)}
  {@attach seen}
>
  <button
    type="button"
    class="ghost car"
    class:open
    class:none={!kids}
    tabindex={kids ? 0 : -1}
    aria-label={open ? 'Свернуть' : 'Раскрыть'}
    aria-expanded={kids ? open : undefined}
    onpointerdown={stop}
    onclick={onopen}><Icon name="chevron-right" size={12} stroke={2.4} /></button
  >
  {#if checks}
    <label class="name">
      <input
        type="checkbox"
        checked={check === 'on'}
        disabled={node.total === 0}
        aria-label="Показывать «{node.name}» в ленте"
        onchange={ontoggle}
        {@attach mixed}
      />
      <span class="t">{node.name}</span>
      {#if node.hint}<span class="src">{node.hint}</span>{/if}
    </label>
  {:else}
    <span class="name"
      ><span class="t">{node.name}</span>{#if node.hint}<span class="src">{node.hint}</span>{/if}</span
    >
  {/if}
  {#if only}
    <button type="button" class="ghost only" title="Показать в ленте только «{node.name}»" onpointerdown={stop} onclick={ononly}
      >только</button
    >
  {/if}
  {#if node.missing || node.bad}
    <span class="n">{node.bad ? 'не читается' : 'нет на диске'}</span>
  {:else}
    <span class="n"
      >{count.own}{#if count.all}&nbsp;<span class="all">{count.all}</span>{/if}</span
    >
  {/if}
  {#if node.root}
    <button type="button" class="ghost x" title={removeTitle(node)} aria-label={removeLabel(node)} onpointerdown={stop} onclick={onremove}
      ><Icon name="x" size={12} stroke={2.4} /></button
    >
  {:else}
    <!-- Место ✕ держится: числа стоят столбиком -->
    <span class="xs"></span>
  {/if}
</li>

<style>
  li {
    display: flex;
    align-items: center;
    gap: 4px;
    padding: 2px 0;
    min-height: 26px;
    border-radius: 4px;
  }
  li.flash {
    animation: flash 2.4s ease-out;
  }
  @keyframes flash {
    0%,
    40% {
      background: color-mix(in srgb, var(--accent) 22%, transparent);
    }
    100% {
      background: transparent;
    }
  }
  li button {
    font-size: 12px;
  }
  .car {
    flex: none;
    width: 18px;
    height: 22px;
    padding: 0;
    display: grid;
    place-items: center;
    color: var(--muted);
  }
  .car :global(svg) {
    transition: transform 0.12s;
  }
  .car.open :global(svg) {
    transform: rotate(90deg);
  }
  .car.none {
    visibility: hidden;
  }
  .name {
    flex: 1;
    min-width: 0;
    display: flex;
    align-items: center;
    gap: 7px;
    overflow: hidden;
    white-space: nowrap;
  }
  label.name {
    cursor: pointer;
  }
  .name input {
    margin: 0;
    flex: none;
  }
  .name span {
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .name .src {
    flex: none;
    max-width: 40%;
    color: var(--muted);
    font-size: 11px;
  }
  li.sep {
    border-top: 1px solid var(--line);
    margin-top: 3px;
    padding-top: 4px;
  }
  /* Файла нет на диске (097 п. 9): имя бледное курсивом, в числа не входит */
  li.missing .name .t {
    color: var(--muted);
    font-style: italic;
  }
  /* Скрытая папка бледнеет, число остаётся прежним */
  li.off .name span,
  li.off .n {
    opacity: 0.45;
  }
  .n {
    color: var(--muted);
    white-space: nowrap;
    font-variant-numeric: tabular-nums;
  }
  .n .all {
    opacity: 0.75;
  }
  .only {
    height: 20px;
    padding: 0 6px;
    font-size: 11px;
    color: var(--muted);
    visibility: hidden;
  }
  li:hover .only,
  li:focus-within .only {
    visibility: visible;
  }
  @media (hover: none) {
    .only {
      visibility: visible;
    }
  }
  .x,
  .xs {
    flex: none;
    width: 22px;
    height: 22px;
    padding: 0;
    display: grid;
    place-items: center;
    color: var(--muted);
  }
</style>
