<script lang="ts">
  import type { SpreadView } from '../../base/core/types'
  import { previewUrl, thumbUrl } from '../../base/core/media'
  import { bgmode } from './bgmode.svelte'
  import { liveImg } from './bgmode'
  import { opacityOf } from './fade'
  import { fadeLive } from './fade.svelte'

  // Фото на фоне (спека 009, design-backdrop §8): область (обрезает) -> повёрнутая рамка (обрезает)
  // -> img. Прозрачность (017) — opacity на рамке, не на img: миниатюра под превью иначе просвечивала бы.
  // Всё в процентах от Go-шных мм, без расчётов. Мышь не ловит никогда: вне режима фона фон
  // не объект, щелчки и жесты проходят по полю.
  let { spread, mini = false }: { spread: SpreadView; mini?: boolean } = $props()

  const sw = $derived(spread.w)
  const pct = (v: number, of: number) => (of > 0 ? `${(v / of) * 100}%` : '0%')
  const flip = (h: boolean, v: boolean) => (h || v ? `scale(${h ? -1 : 1}, ${v ? -1 : 1})` : undefined)
  // Режим фона: пока тянут рамку или кадр, фон рисуется по предпросмотру (bgmode.live), не по View.
  const live = $derived(!mini && bgmode.live && bgmode.cur?.spread === spread.id ? { scope: bgmode.cur.scope, l: bgmode.live } : null)
  const shown = $derived(
    (spread.backdrops ?? []).map((b) => (live && live.scope === b.scope ? { ...b, box: { ...live.l.box }, img: liveImg(b, live.l) } : b)),
  )
</script>

{#each shown as b (b.scope)}
  {@const pos = `left:${pct(b.img.x, b.box.w)};top:${pct(b.img.y, b.box.h)};width:${pct(b.img.w, b.box.w)};height:${pct(b.img.h, b.box.h)}`}
  <div
    class="area"
    data-backdrop={b.scope}
    style:left={pct(b.area.x, sw)}
    style:top={pct(b.area.y, spread.h)}
    style:width={pct(b.area.w, sw)}
    style:height={pct(b.area.h, spread.h)}
  >
    <div
      class="box"
      style:left={pct(b.box.x - b.area.x, b.area.w)}
      style:top={pct(b.box.y - b.area.y, b.area.h)}
      style:width={pct(b.box.w, b.area.w)}
      style:height={pct(b.box.h, b.area.h)}
      style:transform={b.box.rot ? `rotate(${b.box.rot}deg)` : undefined}
      style:opacity={opacityOf(fadeLive.of(spread.id, b.scope, b.fade))}
    >
      <!-- Миниатюра подстилает превью, пока оно грузится (как в Cell) -->
      <img style={pos} style:transform={flip(b.flipH, b.flipV)} src={thumbUrl(b.photo)} alt="" draggable="false" />
      {#if !mini}<img
          style={pos}
          style:transform={flip(b.flipH, b.flipV)}
          src={previewUrl(b.photo)}
          alt=""
          draggable="false"
          loading="lazy"
        />{/if}
    </div>
  </div>
{/each}

<style>
  .area {
    position: absolute;
    overflow: hidden;
    pointer-events: none;
  }
  .box {
    position: absolute;
    overflow: hidden;
    transform-origin: 0 0;
    pointer-events: none;
  }
  img {
    position: absolute;
    max-width: none;
    pointer-events: none;
    user-select: none;
  }
</style>
