<script lang="ts">
  import { untrack } from 'svelte'
  import type { GuideView } from '../../base/core/types'
  import { store } from '../../base/core/store.svelte'
  import { modes } from '../../base/prefs/view.svelte'
  import { hasImage, linePositions, markupUrl, type MarkupKind } from './markup'
  import { markupUi } from './markupui.svelte'

  // Разметка типографии (032, 066): картинка шаблона на весь лист (белое прозрачно — multiply) и направляющие
  // PSD голубыми линиями. Прозрачность — ползунок в карточке (только экран). Над макетом, под ручками; мышь не
  // ловит. Нет файла на диске или выключено «Вид ▸ Разметка типографии» — ничего. w, h — размер листа, мм.
  let { kind, guide, w, h }: { kind: MarkupKind; guide: GuideView | null; w: number; h: number } = $props()

  // До отрисовки, чтобы первый запрос ушёл уже с новым номером, а не дважды.
  $effect.pre(() => {
    const key = guide?.key ?? ''
    untrack(() => markupUi.track(kind, key, store.view.rev))
  })

  const on = $derived(modes.markup && !!guide && guide.ok)
  const lines = $derived(on ? linePositions(guide, w, h) : [])
  const opacity = $derived(markupUi.opacity[kind])
</script>

<!-- Картинка не в общей группе с линиями: multiply смешивается с макетом под ней, а группа с z-index его бы изолировала -->
{#if on && hasImage(guide)}
  <img class="img" src={markupUrl(kind, markupUi.rev[kind])} alt="" draggable="false" style:opacity />
{/if}
{#if lines.length > 0}
  <div class="lines" style:opacity>
    {#each lines as l, i (i)}
      <i class={l.v ? 'v' : 'h'} style:left={l.v ? `${l.pct}%` : null} style:top={l.v ? null : `${l.pct}%`}></i>
    {/each}
  </div>
{/if}

<style>
  .img {
    position: absolute;
    inset: 0;
    width: 100%;
    height: 100%;
    z-index: 5;
    object-fit: fill;
    mix-blend-mode: multiply;
    pointer-events: none;
    user-select: none;
  }
  .lines {
    position: absolute;
    inset: 0;
    z-index: 5;
    pointer-events: none;
  }
  /* Цвет — как у направляющих выравнивания (SelFrame) */
  i {
    position: absolute;
    background: #4aa3ff;
  }
  i.v {
    top: 0;
    bottom: 0;
    width: 1px;
    margin-left: -0.5px;
  }
  i.h {
    left: 0;
    right: 0;
    height: 1px;
    margin-top: -0.5px;
  }
</style>
