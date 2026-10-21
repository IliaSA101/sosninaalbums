<script lang="ts">
  import type { PickResult } from '../../base/core/hooks'
  import { store } from '../../base/core/store.svelte'
  import type { AutoPlan } from '../../base/core/types'
  import { altLabel, sourceLine } from './autobuild'

  // Строка источника окна «Автосборка» (034) со ссылкой на второй путь (080-3): «· Собрать свободные: N» в окне
  // пересборки, «· Пересобрать прошлую (N фото)» в обычном. Образец — «· Показать» в строке похожих кадров (AutoPhotos).
  let { d, plan }: { d: PickResult; plan: AutoPlan | null } = $props()
</script>

<p class="src">
  {sourceLine({ ...d, count: d.ids.length }, plan?.photos ?? null, plan?.similar ?? 0)}
  {#if d.alt}
    · <button type="button" class="link" disabled={store.autoWaiting} onclick={() => store.swapAuto()}>{altLabel(d)}</button>
  {/if}
</p>

<style>
  .src {
    margin: 0;
    color: var(--muted);
    font-size: 12px;
  }
  .link {
    padding: 0;
    background: transparent;
    border: 0;
    color: var(--accent);
    font-size: 12px;
    cursor: pointer;
  }
  .link:disabled {
    opacity: 0.5;
    cursor: default;
  }
</style>
