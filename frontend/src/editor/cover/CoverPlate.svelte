<script lang="ts">
  import type { CoverView } from '../../base/core/types'
  import { colorLive } from '../../base/color/colorlive.svelte'
  import { COVER_Z, boxCss, spineZone } from './cover'

  // Плашка корешка (спека 032, п. 12): цветной прямоугольник во весь корешок под местами и текстами, над фоном.
  // Не выделяется и мышь не ловит; в файл 00.jpg её рисует Go (render.Plates).
  let { cover }: { cover: CoverView } = $props()

  const z = $derived(spineZone(cover))
  // Живой показ окна цвета: плашка видна, даже если цвета ещё нет.
  const color = $derived(colorLive.color('spine', cover.spineBg))
</script>

{#if color && z}
  <div class="plate" style="{boxCss(z.rect, { w: cover.sheet.w, h: cover.sheet.h })};background:{color};z-index:{COVER_Z.plate}"></div>
{/if}

<style>
  .plate {
    position: absolute;
    pointer-events: none;
  }
</style>
