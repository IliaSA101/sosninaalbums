<script lang="ts">
  import type { SpreadView } from '../../base/core/types'
  import { drag } from '../../base/drag/drag.svelte'
  import { store } from '../../base/core/store.svelte'
  import { prefs } from '../../base/prefs/prefs.svelte'
  import { framelive } from './framelive.svelte'
  import { liveImage } from '../tilt/tiltlive'
  import { DEFAULT_FRAME, hasSlack } from '../../base/geom/frame'
  import { aabbOf, uprightAt } from '../../base/geom/rotbox'
  import { markAt, outlineOf } from '../../base/geom/shape'
  import DpiMark from './DpiMark.svelte'
  import FrameControls from './FrameControls.svelte'
  import { tiltmode } from '../tilt/tiltmode.svelte'
  import { miniSpot } from '../places/minispot.svelte'
  import { NO_ROOM } from '../places/minipanel'

  // Над стеком разворота (спека 010 U6): то, что у фото обязано быть доступно и не обрезано, даже если оно
  // лежит под другим слоем, — ✥ сдвига кадра, ползунок масштаба и «!» dpi выделенной ячейки, а также подсветка
  // принимающего бросок фото (обмен, черта вставки). Внутри `.content` их перекрывал бы верхний слой, а у повёрнутых
  // `overflow: clip` резал бы у края разворота. Обёртка повторяет слот (место, поворот), поэтому оси те же, что у Cell.
  let { spread, width }: { spread: SpreadView; width: number } = $props()

  const sw = $derived(spread.w)
  const ph = $derived(spread.h)
  const ppm = $derived(width / sw) // px на мм

  type Mark = 'swap' | 'before' | 'after' | null

  // Цель броска по ключу drag.over (как раньше решал Cell): у места сетки — только обмен.
  function markOf(over: string | null, id: string, placeId: string | undefined): Mark {
    if (!over) return null
    if (placeId) return over === `place:${placeId}` ? 'swap' : null
    if (over === `cell:${id}:center`) return 'swap'
    if (over === `cell:${id}:before`) return 'before'
    return over === `cell:${id}:after` ? 'after' : null
  }

  // Кому нужна обёртка: выделенное фото и цель броска.
  const items = $derived.by(() => {
    const out: { i: number; id: string; selected: boolean; mark: Mark }[] = []
    spread.photos.forEach((id, i) => {
      const selected = store.selectedCell === id
      const place = spread.grid ? spread.places?.find((p) => p.photo === id) : undefined
      const mark = markOf(drag.over, id, place?.id)
      if ((selected || mark) && spread.rects[i]) out.push({ i, id, selected, mark })
    })
    return out
  })

  const pct = (v: number, of: number) => `${(v / of) * 100}%`
</script>

{#each items as it (it.id)}
  {@const r = spread.rects[it.i]}
  {@const fv = spread.frames?.[it.i]}
  {@const rot = fv?.cellRot ?? 0}
  {@const frame = fv ? { s: fv.s, x: fv.x, y: fv.y, r: fv.r } : DEFAULT_FRAME}
  {@const cur = framelive.of(it.id) ?? frame}
  {@const contain = !!spread.grid && spread.places?.find((p) => p.photo === it.id)?.fit === 'contain'}
  {@const missing = store.missing.includes(it.id)}
  <!-- Мелкая ячейка (< 48 px на экране): ✥ и ползунок не помещаются, они есть в карточке справа -->
  {@const big = it.selected && !missing && Math.min(r.w, r.h) * ppm >= 48}
  {@const ok = big && prefs.can('frame')}
  <!-- Дуга наклона (063): место ей считает мини-панель (miniSpot), порог ячейки не нужен -->
  {@const room = it.selected && !missing && tiltmode.on && prefs.can('tilt') && miniSpot.photo === it.id ? miniSpot.arc : NO_ROOM}
  <!-- Ход кадра — по живому углу и размеру: пока тянут наклон, фото на экране уже другое (057) -->
  {@const outline = outlineOf(fv?.shape, fv?.radius)}
  {@const img = fv && framelive.of(it.id) ? liveImage(r, fv.img.h > 0 ? fv.img.w / fv.img.h : r.w / r.h, cur, contain, outline) : fv?.img}
  {@const slack = !!img && !contain && hasSlack(r, img, cur.r, outline)}
  <div
    class="cov"
    data-cov
    style:left={pct(r.x, sw)}
    style:top={pct(r.y, ph)}
    style:width={pct(r.w, sw)}
    style:height={pct(r.h, ph)}
    style:transform={rot ? `rotate(${rot}deg)` : undefined}
  >
    {#if it.mark === 'swap'}<span class="swap"></span>{/if}
    {#if it.mark === 'before' || it.mark === 'after'}<span class="ins {it.mark}"></span>{/if}
    {#if it.selected}
      <FrameControls id={it.id} {spread} {r} {fv} {frame} {cur} movable={ok && slack} zoomable={ok && !contain} {room} />
      {#if !missing}
        <DpiMark
          dpi={fv?.dpi ?? 0}
          minDPI={store.view.minDPI}
          at={markAt(r, rot, fv?.shape, fv?.radius, 6) ?? (rot ? uprightAt(r, rot, aabbOf({ ...r, rot }), 6) : undefined)}
        />
      {/if}
    {/if}
  </div>
{/each}

<style>
  /* Вровень с рамкой выделения (7): над стеком `.content` (3), притемнением (5) и безопасной зоной (6);
     тело мышь не ловит, её берут только ✥, ползунок и метка (у них `pointer-events: auto`: свойство наследуется). */
  .cov {
    position: absolute;
    z-index: 7;
    pointer-events: none;
    /* При смене раскладки «Авто» слот переезжает плавно — обёртка за ним */
    transition:
      left var(--move),
      top var(--move),
      width var(--move),
      height var(--move);
  }
  .swap {
    position: absolute;
    inset: 0;
    box-shadow: 0 0 0 4px var(--accent);
  }
  .ins {
    position: absolute;
    top: -4px;
    bottom: -4px;
    width: 5px;
    border-radius: 3px;
    background: var(--accent);
  }
  .ins.before {
    left: -6px;
  }
  .ins.after {
    right: -6px;
  }
</style>
