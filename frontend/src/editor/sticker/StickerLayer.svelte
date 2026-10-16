<script lang="ts">
  import type { SpreadView, StickerView } from '../../base/core/types'
  import { store } from '../../base/core/store.svelte'
  import { thumbUrl, previewUrl } from '../../base/core/media'
  import { prefs } from '../../base/prefs/prefs.svelte'
  import Icon from '../../base/ui/Icon.svelte'
  import { placesel } from '../places/placesel.svelte'
  import { moveSelected } from '../places/groupgesture'
  import { photoui } from '../photo/photoui.svelte'
  import { additive } from '../places/gesture'
  import { layerIndex } from '../places/layers'
  import { opacityOf } from '../backdrop/fade'
  import { STICKER_NEWER, artSrc, isLost, longSidePx, stickerPx } from './sticker'
  import { sticker, stickerFadeLive } from './sticker.svelte'

  // Стикеры листа (048 Ф2): картинки поверх мест, по Z из Go в общем стеке `.content`, рядом с местами и текстами.
  // Своя обёртка с z-index = 10 + z, поэтому стикер может лежать и под фото («Слой»). Рамка в процентах от мм листа,
  // поворот — вокруг центра; зеркало крутит только картинку (сначала зеркало, потом поворот, как в экспорте).
  // Процедурный рисует /media/gen (адрес без px, размер — от длинной стороны на экране, шаг 256); свой — как
  // фото (/media/p). Файл своего не найден — пунктирная рамка и значок «фото нет», как у фото. Генератор новее
  // программы — пунктир и подсказка. Выделение, перенос и меню — как у места (рамку и ручки рисует StickerOverlay).
  let { spread, mini = false, width }: { spread: SpreadView; mini?: boolean; width: number } = $props()

  const sw = $derived(spread.w)
  const ph = $derived(spread.h)
  const pct = (v: number, of: number) => (of > 0 ? `${(v / of) * 100}%` : '0%')
  const dpr = $derived(globalThis.devicePixelRatio || 1)

  function down(e: PointerEvent, st: StickerView) {
    if (mini || e.button !== 0) return
    e.stopPropagation()
    // Стикеры «Стандартного» уровня на «Простом»: видны и печатаются, но не выделяются (вопрос 5), только тост.
    if (prefs.blocked('sticker', () => placesel.pickSticker(st.id, false))) return
    // Shift/Ctrl+щелчок — в группу или из неё; перенос не начинается.
    if (additive(e)) return placesel.pickSticker(st.id, true)
    const root = (e.currentTarget as HTMLElement).closest<HTMLElement>('.spread')
    if (!root) return
    const solo = () => placesel.pickSticker(st.id, false)
    if (placesel.stickers.includes(st.id) && placesel.isGroup) {
      // Член группы: «Ручная» (и группа без мест и текстов) переносит всю группу, «Авто» — только этот стикер.
      const whole = spread.grid || (placesel.refs.length === 0 && placesel.texts.length === 0)
      if (whole) return moveSelected(e, root, solo)
      return sticker.move(e, root, st, solo)
    }
    if (!placesel.stickers.includes(st.id)) solo()
    sticker.move(e, root, st)
  }

  // Щелчок по стикеру не снимает выделение (его снимает щелчок по фону листа).
  const stop = (e: MouseEvent) => e.stopPropagation()
</script>

{#each spread.stickers ?? [] as st (st.id)}
  {@const b = sticker.boxOf(st)}
  {@const lost = isLost(st, store.missing)}
  {@const gone = lost || !st.known}
  <div
    class="stk"
    class:mini
    class:gone
    class:stretch={!!st.stretch}
    style:left={pct(b.x, sw)}
    style:top={pct(b.y, ph)}
    style:width={pct(b.w, sw)}
    style:height={pct(b.h, ph)}
    style:z-index={layerIndex(st.z)}
    style:transform={b.rot ? `rotate(${b.rot}deg)` : undefined}
    style:opacity={opacityOf(stickerFadeLive.of(spread.id, st.id, st.fade))}
    data-sticker={st.id}
    title={!st.known && !mini ? STICKER_NEWER : undefined}
    role="presentation"
    onpointerdown={(e) => down(e, st)}
    onclick={mini ? undefined : stop}
    oncontextmenu={mini ? undefined : (e) => photoui.openSticker(e, st.id)}
  >
    {#if lost}
      {#if !mini}<span class="miss" role="img" aria-label="Файл стикера не найден"><Icon name="photo-minus" size={18} stroke={2} /></span
        >{/if}
    {:else if st.known && st.gen && st.url}
      <img
        src={artSrc(st.url, stickerPx(longSidePx(st, sw, width), dpr))}
        style:transform={st.flipH || st.flipV ? `scale(${st.flipH ? -1 : 1}, ${st.flipV ? -1 : 1})` : undefined}
        alt=""
        draggable="false"
      />
    {:else if st.known && st.photo}
      <img
        src={mini ? thumbUrl(st.photo) : previewUrl(st.photo)}
        style:transform={st.flipH || st.flipV ? `scale(${st.flipH ? -1 : 1}, ${st.flipV ? -1 : 1})` : undefined}
        alt=""
        draggable="false"
      />
    {/if}
  </div>
{/each}

<style>
  .stk {
    position: absolute;
    box-sizing: border-box;
    pointer-events: auto;
    cursor: move;
    user-select: none;
  }
  .stk.mini {
    pointer-events: none;
    cursor: inherit;
  }
  img {
    display: block;
    width: 100%;
    height: 100%;
    max-width: none;
    object-fit: contain;
    pointer-events: none;
  }
  /* Плашка и волна-лента (049) вытянуты под рамку, без своей пропорции */
  .stretch img {
    object-fit: fill;
  }
  /* Нет файла или генератор новее программы: пунктир вместо картинки */
  .gone {
    display: grid;
    place-items: center;
    border: 1.5px dashed var(--muted);
    background: rgba(255, 255, 255, 0.55);
    color: #6b6b72;
  }
  .miss {
    display: grid;
    place-items: center;
    pointer-events: none;
  }
</style>
