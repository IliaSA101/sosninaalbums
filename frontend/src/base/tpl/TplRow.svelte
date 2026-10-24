<script lang="ts">
  import type { Snippet } from 'svelte'
  import Icon from '../ui/Icon.svelte'
  import { sameWatch } from './watch'

  // Ряд плиток галереи: слева столбец `side` (переключатель SideList, 150 px), стрелки и горизонтальная прокрутка.
  // Общий для галереи вариантов, шаблонов и виньетки; сами плитки рисует вызывающий (TplItem). Тело — 104 px
  // на всех вкладках (плитка 80 + строка 16). count = 0 — вместо ряда подсказка empty (пустая строка — ряд ещё
  // считается). watch — при смене значений (и при первом заполнении 0→N) ряд прокручивается к `.current`, нет `.current` —
  // в начало. Смена числа плиток при заполненном ряде (раскрытие «Похожие», «Ещё», удаление шаблона) положение не трогает.
  let {
    count,
    empty = '',
    inert = false,
    dim = false,
    watch,
    side,
    children,
  }: {
    count: number
    empty?: string
    inert?: boolean
    dim?: boolean // ряд пересчитывается (065): плитки бледные
    watch?: unknown
    side?: Snippet
    children: Snippet
  } = $props()

  let scroll = $state<HTMLElement>()
  const filled = $derived(count > 0)
  let seen: [unknown, boolean] | null = null // прежние значения: родитель отдаёт новый массив watch при каждой перерисовке
  $effect(() => {
    const now: [unknown, boolean] = [watch, filled]
    if (seen && sameWatch(seen[0], now[0]) && seen[1] === now[1]) return
    seen = now
    queueMicrotask(() => {
      if (!scroll) return
      const cur = scroll.querySelector<HTMLElement>('.current')
      const el = cur?.closest<HTMLElement>('.item') ?? cur
      scroll.scrollLeft = el ? el.offsetLeft - (scroll.clientWidth - el.offsetWidth) / 2 : 0
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
  {#if side}
    <div class="side">{@render side()}</div>
  {/if}
  <div class="track">
    {#if count === 0}
      <div class="empty">{empty}</div>
    {:else}
      <button class="nav" onclick={() => scrollBy(-1)} title="Прокрутить влево" aria-label="Прокрутить влево"
        ><Icon name="chevron-left" size={18} /></button
      >
      <div class="scroll" class:dim bind:this={scroll} onwheel={wheel} {inert}>
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
    align-items: flex-start;
    gap: 8px;
    height: 104px;
    margin-top: 4px;
  }
  .side {
    flex: none;
    width: 150px;
  }
  .track {
    flex: 1;
    min-width: 0;
    height: 100%;
    display: flex;
    align-items: flex-start;
    gap: 4px;
  }
  .scroll {
    position: relative; /* offsetLeft плиток — от ряда: так прокрутка к .current точна */
    flex: 1;
    min-width: 0;
    display: flex;
    align-items: flex-start;
    gap: 10px;
    padding: 4px;
    overflow-x: auto;
    overflow-y: hidden;
    scrollbar-width: none;
    transition: opacity 0.12s;
  }
  .scroll.dim {
    opacity: 0.45;
  }
  .nav {
    flex: none;
    width: 24px;
    height: 80px;
    margin-top: 4px;
    padding: 0;
    display: grid;
    place-items: center;
    background: transparent;
  }
  .empty {
    color: var(--muted);
    font-size: 13px;
    padding: 30px 0 0 28px;
  }
</style>
