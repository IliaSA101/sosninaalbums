<script lang="ts">
  import type { CoverView } from '../../base/core/types'
  import { COVER_Z, boxCss, spineZone } from './cover'

  // Плашка корешка (спека 032, п. 12): цветной прямоугольник во весь корешок под местами и текстами, над фоном.
  // Не выделяется и мышь не ловит; в файл 00.jpg её рисует Go (render.Plates).
  let { cover }: { cover: CoverView } = $props()

  const z = $derived(spineZone(cover))
</script>

{#if cover.spineBg && z}
  <div
    class="plate"
    style="{boxCss(z.rect, { w: cover.sheet.w, h: cover.sheet.h })};background:{cover.spineBg};z-index:{COVER_Z.plate}"
  ></div>
{/if}

<style>
  .plate {
    position: absolute;
    pointer-events: none;
  }
</style>
