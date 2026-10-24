<script lang="ts">
  import { slide } from 'svelte/transition'
  import { store } from '../../base/core/store.svelte'
  import { gallery } from './gallery.svelte'
  import CoverGridGallery from './CoverGridGallery.svelte'
  import GridGallery from './GridGallery.svelte'
  import VariantGallery from './VariantGallery.svelte'
  import VigGallery from '../vignette/VigGallery.svelte'
  import { isVignette } from '../vignette/vignette'
  import { COVER_ID } from '../../base/geom/sheet'

  // Рамка галереи (050): одна высота на всех вкладках, тело — по вкладке; шапку рисует само тело (GalleryHead).
  // Пока галерея уезжает после закрытия, gallery.cur уже null — держим последнюю.
  let last = $state(gallery.cur!)
  $effect.pre(() => {
    if (gallery.cur) last = gallery.cur
  })
  const g = $derived(gallery.cur ?? last)
  // Виньетка (решение 80): вместо раскладок «Авто» и шаблонов сетки — шаблоны виньетки.
  const vig = $derived.by(() => {
    const sp = store.view.spreads.find((x) => x.id === g.spread)
    return isVignette(sp) ? sp : null
  })
</script>

<section class="gallery" transition:slide={{ duration: 180 }}>
  {#if vig}
    <VigGallery sp={vig} />
  {:else if g.spread === COVER_ID}
    <CoverGridGallery {g} />
  {:else if g.tab === 'grids'}
    <GridGallery {g} />
  {:else}
    <VariantGallery {g} />
  {/if}
</section>

<style>
  /* Высота задана явно: при смене вкладки, фильтра или ряда галерея не прыгает; slide анимирует её */
  .gallery {
    flex: none;
    box-sizing: border-box;
    height: 154px;
    background: var(--panel2);
    border-top: 1px solid var(--line);
    border-bottom: 1px solid var(--line);
    padding: 6px 12px;
    overflow: hidden;
    /* Нужен для @container в GalleryHead: узкая шапка — короткая «Сохранить…» */
    container-type: inline-size;
  }
</style>
