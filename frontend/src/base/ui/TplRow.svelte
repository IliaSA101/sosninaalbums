<script lang="ts">
  import type { Snippet } from 'svelte'
  import Icon from './Icon.svelte'

  // Ряд плиток галереи шаблонов: подпись слева, стрелки и горизонтальная прокрутка. Общий для галереи сеток
  // страницы и галереи шаблонов виньетки; сами плитки рисует вызывающий (TplItem). count = 0 — вместо ряда
  // подсказка empty (пустая строка — ряд ещё считается). watch — при смене ряд прокручивается к `.current`.
  let {
    label,
    count,
    empty = '',
    tall = false,
    inert = false,
    watch,
    children,
  }: { label: string; count: number; empty?: string; tall?: boolean; inert?: boolean; watch?: unknown; children: Snippet } = $props()

  let scroll = $state<HTMLElement>()
  $effect(() => {
    void [watch, count]
    queueMicrotask(() => {
      const cur = scroll?.querySelector<HTMLElement>('.current')
      const el = cur?.closest<HTMLElement>('.item') ?? cur
      if (el && scroll) scroll.scrollLeft = el.offsetLeft - (scroll.clientWidth - el.offsetWidth) / 2
    })
  })

  function scrollBy(dir: number) {
    scroll?.scrollBy({ left: dir * scroll.clientWidth * 0.8, behavior: 'smooth' })
  }
  function wheel(e: WheelEvent) {
    if (scroll && Math.abs(e.deltaY) > Math.abs(e.deltaX)) scroll.scrollLeft += e.deltaY
  }
</script>

<div class="line">
  <div class="label">{label}</div>
  <div class="track" class:tall>
    {#if count === 0}
      <div class="empty">{empty}</div>
    {:else}
      <button class="nav" onclick={() => scrollBy(-1)} title="Прокрутить влево" aria-label="Прокрутить влево"
        ><Icon name="chevron-left" size={18} /></button
      >
      <div class="scroll" bind:this={scroll} onwheel={wheel} {inert}>
        {@render children()}
      </div>
      <button class="nav" onclick={() => scrollBy(1)} title="Прокрутить вправо" aria-label="Прокрутить вправо"
        ><Icon name="chevron-right" size={18} /></button
      >
    {/if}
  </div>
</div>

<style>
  .line {
    display: flex;
    align-items: center;
    gap: 8px;
    margin-top: 4px;
  }
  .label {
    flex: none;
    width: 110px;
    font-size: 12px;
    color: var(--muted);
  }
  .track {
    flex: 1;
    min-width: 0;
    display: flex;
    align-items: center;
    gap: 4px;
    min-height: 68px; /* карточка + рамка при наведении */
  }
  .track.tall {
    min-height: 98px; /* + имя в две строки под плиткой */
  }
  .scroll {
    flex: 1;
    min-width: 0;
    display: flex;
    gap: 10px;
    padding: 4px;
    overflow-x: auto;
    overflow-y: hidden;
    scrollbar-width: none;
  }
  .nav {
    flex: none;
    width: 24px;
    height: 56px;
    padding: 0;
    display: grid;
    place-items: center;
    background: transparent;
  }
  .empty {
    color: var(--muted);
    font-size: 13px;
    padding-left: 28px;
  }
</style>
