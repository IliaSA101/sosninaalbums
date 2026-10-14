<script lang="ts">
  import { untrack } from 'svelte'
  import type { CoverView } from '../../base/core/types'
  import { coverGuideUrl } from '../../base/core/media'
  import { store } from '../../base/core/store.svelte'
  import { coverUi } from './coverui.svelte'

  // Разметка типографии (спека 032, п. 13): файл на весь лист, прозрачность — ползунок в карточке (только
  // экран). Лежит над макетом, чтобы сверять с ним, но под ручками; мышь не ловит. Нет файла на диске — ничего.
  let { cover }: { cover: CoverView } = $props()

  // Другое имя файла (выбрали, убрали, Undo, открыли альбом) — картинку надо перечитать: адрес один и тот же.
  let seen = ''
  $effect(() => {
    const name = cover.guide
    if (name === seen) return
    seen = name
    coverUi.guideRev = untrack(() => store.view.rev)
  })
</script>

{#if cover.guide && cover.guideOK}
  <img class="guide" src={coverGuideUrl(coverUi.guideRev)} alt="" draggable="false" style:opacity={coverUi.guideOpacity} />
{/if}

<style>
  .guide {
    position: absolute;
    inset: 0;
    width: 100%;
    height: 100%;
    z-index: 5;
    object-fit: fill;
    pointer-events: none;
    user-select: none;
  }
</style>
