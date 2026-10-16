<script lang="ts">
  import { tick } from 'svelte'
  import type { VigChoice } from '../../base/core/types'
  import Icon from '../../base/ui/Icon.svelte'
  import { prefs } from '../../base/prefs/prefs.svelte'
  import VigCard from './VigCard.svelte'
  import { cardBox, gridStep, revealX, type ArrowKey } from './vigdialog'
  import { OWN_TITLE, canEditOwn, firstOwn, ownLabel, tileForm } from './vigown'
  import { vigOwn } from './vigown.svelte'
  import { portraitText } from './vignette'

  // Шаблоны виньетки сеткой под предпросмотром (031 dialog §3): карточки с названием переносятся рядами, без
  // стрелок и горизонтальной прокрутки; не влезло по высоте — прокручивается только сетка.
  // Выбор пишет вызывающий (onpick), «Применить» на уже выбранной карточке по Enter — onapply.
  let {
    items,
    cur,
    sheetW,
    sheetH,
    maxH,
    onpick,
    onapply,
    form,
  }: {
    items: VigChoice[]
    cur: string
    sheetW: number
    sheetH: number
    maxH: number
    onpick: (id: string) => void
    onapply: () => void
    form?: { ratio: string; shape?: string; round?: number } // форма окна: плитка рисует то, что станет после щелчка (tileForm)
  } = $props()

  const card = $derived(cardBox(sheetW, sheetH))
  const ownAt = $derived(firstOwn(items)) // перед этой карточкой — заголовок «Мои шаблоны»
  const canTpl = $derived(prefs.can('templates')) // ✎ и × — с уровня «Шаблоны»
  let grid = $state<HTMLElement>()

  // Roving tabindex: Tab заходит в сетку одной остановкой — на выбранную карточку (нет выбранной — на первую).
  const tabId = $derived(items.some((c) => c.id === cur) ? cur : (items[0]?.id ?? ''))

  const cardEl = (id: string) => grid?.querySelector<HTMLElement>(`[data-tpl="${CSS.escape(id)}"]`)
  // Положение в сетке — у обёртки .cell (она position: relative для ✎ и ×), а не у кнопки внутри неё.
  const cellOf = (el: HTMLElement) => el.parentElement as HTMLElement

  // Колонок в сетке — по числу карточек в первом ряду (ширину решает CSS).
  function columns(): number {
    const all = grid?.querySelectorAll<HTMLElement>('[data-tpl]')
    if (!all?.length) return 1
    const top = cellOf(all[0]).offsetTop
    return Array.from(all).filter((el) => cellOf(el).offsetTop === top).length
  }

  async function onkey(e: KeyboardEvent, c: VigChoice) {
    if (e.key === 'Enter') {
      e.preventDefault()
      e.stopPropagation()
      if (c.over) return
      if (c.id === cur) onapply()
      else onpick(c.id)
      return
    }
    if (!['ArrowLeft', 'ArrowRight', 'ArrowUp', 'ArrowDown'].includes(e.key)) return
    e.preventDefault()
    const id = gridStep(items, c.id, e.key as ArrowKey, columns())
    if (!id) return
    onpick(id)
    await tick()
    cardEl(id)?.focus()
  }

  // Выбранный шаблон — в поле зрения сетки (при открытии и после ответа Go).
  $effect(() => {
    const id = cur
    void items.length
    if (!id) return
    tick().then(() => {
      const el = cardEl(id)
      const c = el && cellOf(el)
      if (grid && c) grid.scrollTop = revealX(grid.scrollTop, c.offsetTop, c.offsetTop + c.offsetHeight, grid.clientHeight)
    })
  })
</script>

<div class="grid" bind:this={grid} style:max-height="{maxH}px" role="group" aria-label="Шаблоны">
  {#each items as c, i (c.id)}
    <!-- Заголовок — не карточка: в items его нет, стрелки его не видят -->
    {#if i === ownAt}<div class="sect">{OWN_TITLE}</div>{/if}
    <div class="cell">
      <button
        type="button"
        class="tpl"
        class:on={cur === c.id}
        class:off={c.over}
        data-tpl={c.id}
        tabindex={c.id === tabId ? 0 : -1}
        aria-pressed={cur === c.id}
        aria-disabled={c.over}
        title="{ownLabel(c)} · {portraitText(c)}"
        onclick={() => !c.over && onpick(c.id)}
        onkeydown={(e) => onkey(e, c)}
      >
        <VigCard {c} w={card.w} h={card.h} form={form && tileForm(items, cur, c.id, form)} />
        <span class="name">{ownLabel(c)}</span>
      </button>
      {#if canEditOwn(c, canTpl)}
        <div class="tools">
          <button type="button" onclick={() => vigOwn.rename(c)} title="Переименовать" aria-label="Переименовать шаблон">✎</button>
          <button type="button" onclick={() => vigOwn.del(c)} title="Удалить шаблон" aria-label="Удалить шаблон"
            ><Icon name="x" size={12} /></button
          >
        </div>
      {/if}
    </div>
  {/each}
</div>

<style>
  .grid {
    position: relative;
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(176px, 1fr));
    gap: 6px;
    align-content: start;
    justify-items: center;
    padding: 2px;
    overflow-x: hidden;
    overflow-y: auto;
    scrollbar-width: thin;
  }
  .cell {
    position: relative;
  }
  /* Заголовок раздела — на всю ширину сетки */
  .sect {
    grid-column: 1 / -1;
    justify-self: stretch;
    padding: 6px 4px 0;
    border-top: 1px solid var(--line);
    font-size: 12px;
    font-weight: 600;
    color: var(--muted);
  }
  /* ✎ и × — над превью, видны при наведении и фокусе (с клавиатуры достижимы) */
  .tools {
    position: absolute;
    left: 8px;
    top: 8px;
    display: flex;
    gap: 2px;
    opacity: 0;
  }
  .cell:hover .tools,
  .cell:focus-within .tools {
    opacity: 1;
  }
  .tools button {
    width: 20px;
    height: 20px;
    padding: 0;
    display: grid;
    place-items: center;
    border-radius: 4px;
    background: rgba(17, 17, 17, 0.8);
  }
  .tpl {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 4px;
    width: 176px;
    height: auto;
    padding: 4px;
    white-space: normal;
    background: transparent;
    border: 2px solid transparent;
    border-radius: 6px;
    cursor: pointer;
  }
  .tpl:hover:not(:disabled) {
    background: transparent; /* общий hover кнопок красит фон — здесь только рамка */
  }
  .tpl:hover:not(.off) {
    border-color: #8a8a90;
  }
  .tpl.on,
  .tpl.on:hover:not(.off),
  .tpl:focus-visible {
    border-color: var(--accent);
  }
  .tpl.off {
    cursor: not-allowed;
  }
  /* Название — мелко и по центру; длинное переносится, а не обрезается */
  .name {
    max-width: 100%;
    font-size: 11px;
    line-height: 13px;
    text-align: center;
    color: var(--muted);
    overflow-wrap: anywhere;
  }
  .tpl.on .name {
    color: var(--text);
  }
  .tpl.off .name {
    opacity: 0.5;
  }
</style>
