<script lang="ts">
  import type { SpreadView } from '../../base/core/types'
  import { drag } from '../../base/drag/drag.svelte'
  import { store } from '../../base/core/store.svelte'
  import { placesel } from '../places/placesel.svelte'
  import BackdropEditor from '../backdrop/BackdropEditor.svelte'
  import CellOverlay from '../photo/CellOverlay.svelte'
  import StraightenLayer from '../tilt/StraightenLayer.svelte'
  import { straighten } from '../tilt/straighten.svelte'
  import GroupFrame from '../places/GroupFrame.svelte'
  import SelFrame from '../places/SelFrame.svelte'
  import StickerOverlay from '../sticker/StickerOverlay.svelte'
  import NameEditor from '../captions/NameEditor.svelte'
  import { targetKey } from '../captions/nametarget'
  import TextOverlay from '../text/TextOverlay.svelte'
  import type { TextGestures } from '../text/textgesture.svelte'
  import type { NameTrack } from './nametrack.svelte'

  // Слои выделенного над стеком `.content` (вне его обрезки): рамка текста, ввод имени, контролы ячейки,
  // рамки мест, стикеров, группы и фона. Без обёртки — слои остаются прямыми детьми `.spread`.
  let {
    spread,
    width,
    mini,
    preview,
    bgEdit,
    tg,
    nt,
  }: { spread: SpreadView; width: number; mini: boolean; preview: boolean; bgEdit: boolean; tg: TextGestures; nt: NameTrack } = $props()

  const sw = $derived(spread.w)
  const sh = $derived(spread.h)
  const current = $derived(store.spread?.id === spread.id)

  // Рамка выделения: у текущего большого разворота, не в предпросмотре галереи и не во время переноса фото.
  // Контролы выделенной ячейки (✥, ползунок, «!») и подсветка цели броска — над стеком, вне обрезки (CellOverlay).
  const cellUi = $derived(!mini && !preview && !bgEdit)
  const framed = $derived(!mini && !preview && !drag.active && !bgEdit && current && placesel.selected.length > 0)

  // Рамка выделенных стикеров (048): те же условия, что у рамки места. Один — с ручками, в группе — обводки членов.
  const stickerFramed = $derived(!mini && !preview && !drag.active && !bgEdit && current && placesel.selectedStickers.length > 0)

  // Рамка группы — тот же набор условий, плюс выделено 2 и больше объектов (в том числе тексты).
  const grouped = $derived(!mini && !preview && !drag.active && !bgEdit && current && placesel.isGroup)
</script>

{#if !mini}<TextOverlay {spread} {width} {tg} />{/if}
{#if nt.named}
  <!-- key: при переходе на соседнюю подпись без зазора подсказка переставляется заново -->
  {#key targetKey(nt.named)}
    <div
      bind:this={nt.hint}
      class="namehint"
      style:left="{(nt.named.box.x / sw) * 100}%"
      style:top="{(nt.named.box.y / sh) * 100}%"
      style:width="{(nt.named.box.w / sw) * 100}%"
      style:height="{(nt.named.box.h / sh) * 100}%"
    ></div>
  {/key}
{/if}
{#if !mini}<NameEditor {spread} {width} />{/if}
{#if cellUi}<CellOverlay {spread} {width} />{/if}
{#if framed}<SelFrame {spread} {width} />{/if}
{#if stickerFramed}<StickerOverlay {spread} {width} />{/if}
{#if grouped}<GroupFrame {spread} {width} />{/if}
{#if bgEdit}<BackdropEditor {spread} {width} />{/if}
<!-- Режим «Выпрямить по линии» (045): слой ловит мышь над рамками выделения и фона -->
{#if !mini && !preview && straighten.cur?.spread === spread.id && current}<StraightenLayer {spread} />{/if}

<style>
  .namehint {
    position: absolute;
    pointer-events: none;
  }
</style>
