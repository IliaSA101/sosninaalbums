<script lang="ts">
  import { screenDpr } from '../../base/ui/dpr.svelte'
  import type { SpreadView } from '../../base/core/types'
  import { colorLive } from '../../base/color/colorlive.svelte'
  import { opacityOf } from '../backdrop/fade'
  import { albumFade, patFadeLive } from '../backdrop/fade.svelte'
  import { layerPx, patternSrc } from './pattern'

  // Узор программы (048): прозрачный PNG от Go на своей области, под фото фона. Размер запроса — от ширины области
  // на экране (шаг 256), поэтому зум чётко перерисовывает узор, но не на каждый пиксель. Прозрачность — opacity,
  // как у фото фона (оно с цветом страницы даёт ту же картинку, что экспорт). Мышь не ловит: фон — не объект.
  // Узор новее программы (known = false) Go присылает без адреса: рисуется пусто.
  let { spread, width }: { spread: SpreadView; width: number } = $props()

  const sw = $derived(spread.w)
  const pct = (v: number, of: number) => (of > 0 ? `${(v / of) * 100}%` : '0%')
  const dpr = $derived(screenDpr.value)
</script>

{#each spread.patterns ?? [] as p (p.scope)}
  {#if p.known && p.url}
    <img
      class="pat"
      data-pattern={p.album ? undefined : p.scope}
      data-album-bg={p.album ? p.scope : undefined}
      src={patternSrc(p.album ? p.url : colorLive.inkUrl(`pat:${spread.id}:${p.scope}`, p.url), layerPx(p.area.w, sw, width, dpr))}
      alt=""
      draggable="false"
      style:left={pct(p.area.x, sw)}
      style:top={pct(p.area.y, spread.h)}
      style:width={pct(p.area.w, sw)}
      style:height={pct(p.area.h, spread.h)}
      style:opacity={opacityOf(p.album ? albumFade(p.pat.fade ?? 0) : patFadeLive.of(spread.id, p.scope, p.pat.fade ?? 0))}
    />
  {/if}
{/each}

<style>
  .pat {
    position: absolute;
    max-width: none;
    pointer-events: none;
    user-select: none;
  }
</style>
