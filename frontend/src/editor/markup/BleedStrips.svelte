<script lang="ts">
  import { store } from '../../base/core/store.svelte'
  import { modes } from '../../base/prefs/view.svelte'

  // Обрез (066): розовые полосы по внешним краям разворота, мм из Go (View.bleed). Показывает тот же флажок,
  // что и безопасную зону (G); мини-развороты и обложку Spread сюда не пускает.
  let { w, h }: { w: number; h: number } = $props()
  const strips = $derived(modes.safe ? (store.view.bleed ?? []) : [])
</script>

{#each strips as r, i (i)}
  <div
    class="bleed"
    style:left="{(r.x / w) * 100}%"
    style:top="{(r.y / h) * 100}%"
    style:width="{(r.w / w) * 100}%"
    style:height="{(r.h / h) * 100}%"
  ></div>
{/each}

<style>
  /* Розовое — как поля обреза в PSD типографий; рамки нет, пунктир безопасной зоны лежит поверх */
  .bleed {
    position: absolute;
    z-index: 5;
    pointer-events: none;
    background: rgba(255, 90, 90, 0.35);
  }
</style>
