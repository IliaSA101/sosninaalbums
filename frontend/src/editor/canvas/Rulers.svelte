<script lang="ts">
  import { store } from '../../base/core/store.svelte'
  import { placesel } from '../places/placesel.svelte'
  import { selSpan, ticks } from './ruler'
  import { BAR, M, railCols, railWidth } from '../inspector/panels'

  // Линейки (спека 010, design-ui §3): мм сверху и слева от разворота, ноль — левый верхний угол.
  // left/top/width — где разворот стоит в окне холста (Stage уже считает это для зума), w/h — размер Stage.
  // В зуме полосы прилипают к краю окна холста (левая — правее ленты). Пропускают мышь: холст под ними живой.
  let {
    stage,
    left,
    top,
    width,
    w,
    h,
  }: { stage: HTMLElement | undefined; left: number; top: number; width: number; w: number; h: number } = $props()

  const T = 18 // толщина полосы, px
  const sw = $derived(store.spread?.w ?? 0)
  const ph = $derived(store.spread?.h ?? 0)
  const ppm = $derived(sw > 0 ? width / sw : 0) // px на мм
  const height = $derived(ppm * ph)
  const sy = $derived(top + BAR) // верх разворота в координатах Stage
  const railR = $derived(M + railWidth(railCols(h - BAR)))

  // Полосы: верхняя над разворотом, левая слева; общий угол T×T не занят.
  const lx = $derived(Math.max(left - T, railR, 0))
  const ty = $derived(Math.max(sy - T, BAR))
  const tx0 = $derived(Math.max(left, lx + T, 0))
  const tx1 = $derived(Math.min(left + width, w))
  const ly0 = $derived(Math.max(sy, ty + T))
  const ly1 = $derived(Math.min(sy + height, h))

  // Деления только в пределах разворота и видимой части полосы.
  const clampMm = (v: number) => Math.max(0, Math.min(sw, v))
  const topTicks = $derived(tx1 > tx0 ? ticks(clampMm((tx0 - left) / ppm), clampMm((tx1 - left) / ppm), ppm) : [])
  const leftTicks = $derived(ly1 > ly0 ? ticks(clampMm((ly0 - sy) / ppm), clampMm((ly1 - sy) / ppm), ppm) : [])

  // Выделенное: места (с предпросмотром жеста), тексты группы и одиночный текст; описанный прямоугольник.
  const sel = $derived(placesel.boxOf(placesel.targetIds))
  const selX = $derived(sel ? selSpan(sel.x, sel.x + sel.w, sw) : null)
  const selY = $derived(sel ? selSpan(sel.y, sel.y + sel.h, ph) : null)

  // Метка курсора — в px Stage; за пределами Stage пропадает.
  let cur = $state<{ x: number; y: number } | null>(null)
  $effect(() => {
    const node = stage
    if (!node) return
    const move = (e: PointerEvent) => {
      const r = node.getBoundingClientRect()
      cur = { x: e.clientX - r.left, y: e.clientY - r.top }
    }
    const leave = () => (cur = null)
    node.addEventListener('pointermove', move)
    node.addEventListener('pointerleave', leave)
    return () => {
      node.removeEventListener('pointermove', move)
      node.removeEventListener('pointerleave', leave)
    }
  })
</script>

{#if ppm > 0}
  <div class="rulers" data-chrome>
    {#if tx1 > tx0}
      <div class="strip h" data-mini-edge="top" data-guide="rulers" style:left="{tx0}px" style:top="{ty}px" style:width="{tx1 - tx0}px">
        {#if selX}<i class="sel" style:left="{left + selX[0] * ppm - tx0}px" style:width="{(selX[1] - selX[0]) * ppm}px"></i>{/if}
        {#each topTicks as t (t.at)}
          <i class="t" class:major={t.major} style:left="{left + t.at * ppm - tx0}px"
            >{#if t.major}<b>{t.label}</b>{/if}</i
          >
        {/each}
        {#if cur && cur.x >= tx0 && cur.x <= tx1}<i class="mark" style:left="{cur.x - tx0}px"></i>{/if}
      </div>
    {/if}
    {#if ly1 > ly0}
      <div class="strip v" data-mini-edge="left" data-guide="rulers" style:left="{lx}px" style:top="{ly0}px" style:height="{ly1 - ly0}px">
        {#if selY}<i class="sel" style:top="{sy + selY[0] * ppm - ly0}px" style:height="{(selY[1] - selY[0]) * ppm}px"></i>{/if}
        {#each leftTicks as t (t.at)}
          <i class="t" class:major={t.major} style:top="{sy + t.at * ppm - ly0}px"
            >{#if t.major}<b>{t.label}</b>{/if}</i
          >
        {/each}
        {#if cur && cur.y >= ly0 && cur.y <= ly1}<i class="mark" style:top="{cur.y - ly0}px"></i>{/if}
      </div>
    {/if}
  </div>
{/if}

<style>
  /* Над разворотом и рамкой выделения (10), под лентой, карточкой и полосой (20, 30) */
  .rulers {
    position: absolute;
    inset: 0;
    z-index: 12;
    pointer-events: none;
  }
  .strip {
    position: absolute;
    overflow: hidden;
    background: rgba(32, 32, 34, 0.92);
    color: var(--muted);
    font-size: 9px;
    line-height: 1;
    font-variant-numeric: tabular-nums;
  }
  .strip.h {
    height: 18px;
    border-bottom: 1px solid var(--line);
  }
  .strip.v {
    width: 18px;
    border-right: 1px solid var(--line);
  }
  /* Деление: контейнер на всю толщину полосы, черта рисуется градиентом у края, подпись — сверху */
  .t {
    position: absolute;
    --len: 5px;
    --c: #5a5a60;
  }
  .t.major {
    --len: 10px;
  }
  .h .t {
    top: 0;
    bottom: 0;
    width: 1px;
    background: linear-gradient(var(--c), var(--c)) bottom / 1px var(--len) no-repeat;
  }
  .v .t {
    left: 0;
    right: 0;
    height: 1px;
    background: linear-gradient(var(--c), var(--c)) right / var(--len) 1px no-repeat;
  }
  .t b {
    position: absolute;
    font-weight: 400;
    white-space: nowrap;
  }
  .h .t b {
    left: 3px;
    top: 2px;
  }
  /* Подпись левой линейки читается снизу вверх, как в привычных редакторах */
  .v .t b {
    left: 2px;
    top: 3px;
    writing-mode: vertical-rl;
    transform: rotate(180deg);
  }
  .sel {
    position: absolute;
    background: rgba(245, 217, 10, 0.3);
  }
  .h .sel {
    top: 0;
    bottom: 0;
  }
  .v .sel {
    left: 0;
    right: 0;
  }
  .mark {
    position: absolute;
    background: var(--accent);
  }
  .h .mark {
    top: 0;
    bottom: 0;
    width: 1px;
  }
  .v .mark {
    left: 0;
    right: 0;
    height: 1px;
  }
</style>
