<script lang="ts">
  import { setTile, tileOf, tileRange, type GalleryKind } from './tilesize'

  // Ползунок «Размер миниатюр» окон-галерей (133, 163): пределы и память — по виду окна, Ctrl+колесо над плитками делает то же.
  let { kind }: { kind: GalleryKind } = $props()

  const range = $derived(tileRange(kind))
</script>

<!-- 133: ползунок и слова — те же, что у ленты фото (053) -->
<input
  class="size"
  type="range"
  min={range.lo}
  max={range.hi}
  value={tileOf(kind)}
  oninput={(e) => setTile(kind, Number(e.currentTarget.value))}
  title="Размер миниатюр"
  aria-label="Размер миниатюр"
  onchange={(e) => e.currentTarget.blur()}
/>

<style>
  .size {
    width: 72px;
  }
</style>
