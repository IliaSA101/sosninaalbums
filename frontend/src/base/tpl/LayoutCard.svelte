<script lang="ts">
  import { cardClasses, cardWidth, cellBox, type CardRect, type CardSheet } from './layoutcard'

  // Плитка раскладки: схема ячеек на белом листе. Одна на полосу «Варианты» и окно «Автосборка» (065), чтобы они не расходились.
  // С onclick — кнопка (рамка при наведении), без — просто картинка. fallback — пунктир («запасная»: разрешённых семейств нет).
  // page — галерея страницы: чужая половина затемнена.
  let {
    rects,
    sheet,
    h,
    current = false,
    fallback = false,
    page = null,
    title,
    onclick,
    onmouseenter,
    onmouseleave,
  }: {
    rects: readonly CardRect[]
    sheet: CardSheet
    h: number
    current?: boolean
    fallback?: boolean
    page?: 'left' | 'right' | null
    title?: string
    onclick?: () => void
    onmouseenter?: () => void
    onmouseleave?: () => void
  } = $props()

  const cls = $derived(cardClasses({ current, fallback, clickable: !!onclick }).join(' '))
</script>

{#snippet inner()}
  {#each rects as r, i (i)}
    {@const b = cellBox(r, sheet)}
    <span class="cell" style:left="{b.left}%" style:top="{b.top}%" style:width="{b.width}%" style:height="{b.height}%"></span>
  {/each}
  <span class="spine"></span>
  {#if page}
    <span class="other" style:left={page === 'left' ? '50%' : '0'}></span>
  {/if}
{/snippet}

{#if onclick}
  <button class={cls} {title} style:width="{cardWidth(h, sheet)}px" style:height="{h}px" {onmouseenter} {onmouseleave} {onclick}>
    {@render inner()}
  </button>
{:else}
  <div
    class={cls}
    {title}
    style:width="{cardWidth(h, sheet)}px"
    style:height="{h}px"
    {onmouseenter}
    {onmouseleave}
    role="img"
    aria-label={title}
  >
    {@render inner()}
  </div>
{/if}

<style>
  .card {
    all: unset;
    display: block;
    flex: none;
    position: relative;
    background: #fff;
    outline: 2px solid transparent;
    outline-offset: 2px;
    transition: outline-color 0.15s;
  }
  button.card {
    cursor: pointer;
  }
  .card.hov:hover:not(.current) {
    outline-color: #8a8a90;
  }
  .card.current {
    outline-color: var(--accent);
  }
  /* Запасная: разрешённых семейств на развороте нет, взята лучшая раскладка */
  .card.fallback {
    outline: 1px dashed var(--muted);
    outline-offset: 2px;
  }
  .cell {
    position: absolute;
    background: #d8d8d8;
    box-shadow: inset 0 0 0 1px #a8a8a8;
  }
  .spine {
    position: absolute;
    left: 50%;
    top: 0;
    bottom: 0;
    width: 1px;
    background: rgba(0, 0, 0, 0.25);
  }
  /* Галерея страницы: чужая половина не участвует */
  .other {
    position: absolute;
    top: 0;
    bottom: 0;
    width: 50%;
    background: #555;
  }
</style>
