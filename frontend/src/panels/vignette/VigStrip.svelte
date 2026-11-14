<script lang="ts">
  import { tick } from 'svelte'
  import Icon from '../../base/ui/Icon.svelte'
  import { prefs } from '../../base/prefs/prefs.svelte'
  import VigCard from './VigCard.svelte'
  import { cardBox, gridStep, revealX, type ArrowKey } from './vigdialog'
  import { FORMS, tileFormOf, type Tile } from './vigform'
  import { canEditOwn, ownLabel } from './vigown'
  import { vigOwn } from './vigown.svelte'
  import { portraitText } from './vignette'

  // Плитки шаблонов виньетки сеткой под предпросмотром (031 dialog §3, 056): плитка = шаблон + форма, группы с заголовками
  // («Прямоугольные», «Скруглённые», «Круги», «Мои шаблоны») идут из Tile.sep. Карточки с названием переносятся рядами, без
  // стрелок и горизонтальной прокрутки; не влезло по высоте — прокручивается только сетка.
  // Выбор пишет вызывающий (onpick), «Применить» на уже выбранной плитке по Enter — onapply. cur — ключ выбранной плитки.
  let {
    tiles,
    cur,
    sheetW,
    sheetH,
    maxH,
    onpick,
    onapply,
  }: {
    tiles: Tile[]
    cur: string
    sheetW: number
    sheetH: number
    maxH: number
    onpick: (t: Tile) => void
    onapply: () => void
  } = $props()

  const card = $derived(cardBox(sheetW, sheetH))
  const canTpl = $derived(prefs.can('templates')) // ✎ и × — с уровня «Шаблоны»
  let grid = $state<HTMLElement>()

  // Roving tabindex: Tab заходит в сетку одной остановкой — на выбранную плитку (нет выбранной — на первую).
  const tabKey = $derived(tiles.some((t) => t.key === cur) ? cur : (tiles[0]?.key ?? ''))

  const cardEl = (key: string) => grid?.querySelector<HTMLElement>(`[data-tpl="${CSS.escape(key)}"]`)
  // Положение в сетке — у обёртки .cell (она position: relative для ✎ и ×), а не у кнопки внутри неё.
  const cellOf = (el: HTMLElement) => el.parentElement as HTMLElement

  // Колонок в сетке — по числу плиток в первом ряду (ширину решает CSS).
  function columns(): number {
    const all = grid?.querySelectorAll<HTMLElement>('[data-tpl]')
    if (!all?.length) return 1
    const top = cellOf(all[0]).offsetTop
    return Array.from(all).filter((el) => cellOf(el).offsetTop === top).length
  }

  async function onkey(e: KeyboardEvent, t: Tile) {
    if (e.key === 'Enter') {
      e.preventDefault()
      e.stopPropagation()
      if (t.c.over) return
      if (t.key === cur) onapply()
      else onpick(t)
      return
    }
    if (!['ArrowLeft', 'ArrowRight', 'ArrowUp', 'ArrowDown'].includes(e.key)) return
    e.preventDefault()
    const ids = tiles.map((x) => ({ id: x.key, over: x.c.over }))
    const next = tiles.find((x) => x.key === gridStep(ids, t.key, e.key as ArrowKey, columns()))
    if (!next) return
    onpick(next)
    await tick()
    cardEl(next.key)?.focus()
  }

  // Выбранная плитка — в поле зрения сетки (при открытии и после ответа Go).
  $effect(() => {
    const key = cur
    void tiles.length
    if (!key) return
    tick().then(() => {
      const el = cardEl(key)
      const c = el && cellOf(el)
      if (grid && c) grid.scrollTop = revealX(grid.scrollTop, c.offsetTop, c.offsetTop + c.offsetHeight, grid.clientHeight)
    })
  })
</script>

<div class="grid" bind:this={grid} style:max-height="{maxH}px" role="group" aria-label="Шаблоны">
  {#each tiles as t (t.key)}
    {@const c = t.c}
    <!-- Заголовок — не плитка: в tiles его нет, стрелки его не видят -->
    {#if t.sep}<div class="sect">{t.sep}</div>{/if}
    <div class="cell">
      <button
        type="button"
        class="tpl"
        class:on={cur === t.key}
        class:off={c.over}
        data-tpl={t.key}
        tabindex={t.key === tabKey ? 0 : -1}
        aria-pressed={cur === t.key}
        aria-disabled={c.over}
        title="{ownLabel(c)} · {FORMS[t.k].name} · {portraitText(c)}"
        onclick={() => !c.over && onpick(t)}
        onkeydown={(e) => onkey(e, t)}
      >
        <VigCard {c} w={card.w} h={card.h} form={tileFormOf(t)} />
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
