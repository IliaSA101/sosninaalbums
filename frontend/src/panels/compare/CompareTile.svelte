<script lang="ts">
  import { untrack } from 'svelte'
  import { store } from '../../base/core/store.svelte'
  import { prefs } from '../../base/prefs/prefs.svelte'
  import { markOf } from '../marks/marks'
  import { originalUrl, previewUrl } from '../../base/core/media'
  import { cancelOnUnmount } from '../../base/ui/imgcancel'
  import { keymap } from '../../base/keys/keymap.svelte'
  import Icon from '../../base/ui/Icon.svelte'
  import MissPhoto from '../../base/ui/MissPhoto.svelte'
  import PreviewMarks from '../../editor/photo/PreviewMarks.svelte'
  import { missTip } from '../library/missing'
  import { autocheck } from '../folders/autocheck'
  import { CAP, compare } from './compare.svelte'
  import { cellCaption } from './compareset'

  // Ячейка окна сравнения (138): кадр (превью, поверх него оригинал выше 1:1), подпись «имя · N из M», отметки,
  // «Лучший» и ✕. Положение кадра задаёт общая лупа (syncloupe.svelte.ts); рамка активной — обводка вида.
  let { i, id }: { i: number; id: string } = $props()

  const lp = compare.loupe
  const cell = $derived(compare.cells[i])
  const at = $derived(lp.at(i))
  const photo = $derived(store.byId.get(id))
  const mark = $derived(photo ? markOf(photo, store.marks[photo.id]) : null)
  const active = $derived(compare.active === i)
  const gone = $derived(compare.gone(id))
  const orig = $derived(compare.orig(id))
  const nat = $derived(compare.nat(id))
  const ratio = $derived(photo && photo.w > 0 && photo.h > 0 ? photo.w / photo.h : 1.5)
  const caption = $derived(compare.st ? cellCaption(compare.st, i, photo?.name ?? '') : '')
  const failTip = $derived(missTip(id, { [id]: photo?.name ?? '' }, store.unread))
  const zs = $derived(lp.scale(i))

  // Зум выше 1:1 по превью устоялся — запрашиваем оригинал этой ячейки (предел и бюджет — compare.max).
  $effect(() => {
    const want = !gone && compare.max(id) > 1 && zs > 1 + 1e-6
    const o = orig
    const k = id
    return untrack(() => o.sync(k, want))
  })

  const natNow = (e: Event) => ({
    w: (e.currentTarget as HTMLImageElement).naturalWidth,
    h: (e.currentTarget as HTMLImageElement).naturalHeight,
  })
  function onFail() {
    compare.fail(id)
    autocheck.soon(true)
  }
</script>

{#if cell}
  <!-- Ячейка фокусируема (Tab её выбирает, design §6): кнопки внутри вне порядка Tab -->
  <!-- svelte-ignore a11y_no_noninteractive_tabindex -->
  <div
    class="cell"
    class:active
    role="group"
    aria-label={caption}
    tabindex="0"
    data-i={i}
    style:left="{cell.left}px"
    style:top="{cell.top}px"
    style:width="{cell.w}px"
    style:height="{cell.h + CAP}px"
    onpointerdown={() => compare.pick(i)}
  >
    <div
      class="cview"
      class:canzoom={lp.canZoom(i) && !lp.zoomed(i)}
      class:zoomed={lp.zoomed(i)}
      class:panning={lp.pressed === i}
      role="presentation"
      data-cview={i}
      style:height="{cell.h}px"
      onpointerdown={(e) => lp.down(i, e)}
      onpointermove={(e) => lp.move(i, e)}
      onpointerup={(e) => lp.up(i, e)}
      onpointercancel={() => lp.cancel()}
    >
      {#if gone}
        <div class="gone" style:--a={ratio}><MissPhoto size={48} tip={failTip} /></div>
      {:else}
        <!-- Рамка кадра: размер и сдвиг из лупы (width/height и сдвиг, не scale — браузер пересчитывает картинку чётко) -->
        <div class="frame" style:width="{at.w}px" style:height="{at.h}px" style:transform="translate({at.left}px, {at.top}px)">
          <img
            class="ph"
            src={previewUrl(id)}
            alt={photo?.name ?? ''}
            draggable="false"
            onload={(e) => compare.loaded(id, natNow(e))}
            onerror={onFail}
            {@attach cancelOnUnmount}
          />
          {#if orig.live(id)}
            <!-- Оригинал поверх превью в той же рамке; виден по load, если пропорции те же -->
            <img
              class="ph orig"
              class:ready={orig.ready(id)}
              src={originalUrl(id)}
              alt=""
              draggable="false"
              onload={(e) => orig.loaded(id, natNow(e), nat)}
              onerror={() => orig.error(id)}
              {@attach cancelOnUnmount}
            />
          {/if}
        </div>
      {/if}
    </div>
    <div class="ccap">
      <span class="nm">{caption}</span>
      {#if mark && prefs.can('marks')}<PreviewMarks {mark} />{/if}
      <span class="acts">
        {#if compare.canBest}
          <button
            class="accent"
            tabindex="-1"
            title="Лучший: отобрать этот, отклонить остальные (Enter)"
            onclick={() => {
              compare.pick(i)
              void compare.best()
            }}>Лучший</button
          >
        {/if}
        <button
          class="sq"
          tabindex="-1"
          title={keymap.tip('Убрать из сравнения', 'binRemove')}
          aria-label="Убрать из сравнения"
          onclick={() => compare.remove(i)}
        >
          <Icon name="x" size={14} />
        </button>
      </span>
    </div>
  </div>
{/if}

<style>
  .cell {
    position: absolute;
    border-radius: 4px;
    outline: none;
  }
  .cell:focus-visible {
    outline: 2px solid var(--accent);
    outline-offset: 1px;
  }
  .cview {
    position: relative;
    overflow: hidden;
    background: rgba(255, 255, 255, 0.035);
    border-radius: 3px 3px 0 0;
    touch-action: none;
  }
  .cell.active .cview {
    outline: 2px solid var(--accent);
    outline-offset: -1px;
  }
  .cview.canzoom {
    cursor: zoom-in;
  }
  .cview.zoomed {
    cursor: grab;
  }
  .cview.panning {
    cursor: grabbing;
  }
  .frame {
    position: absolute;
    left: 0;
    top: 0;
  }
  .ph {
    display: block;
    width: 100%;
    height: 100%;
    object-fit: contain;
    user-select: none;
    -webkit-user-drag: none;
  }
  /* Оригинал: поверх превью, скрыт до load и пока пропорции не сверены */
  .ph.orig {
    position: absolute;
    left: 0;
    top: 0;
    visibility: hidden;
  }
  .ph.orig.ready {
    visibility: visible;
  }
  /* Заглушка «нет файла» в пропорциях фото; вписана в ячейку */
  .gone {
    position: absolute;
    inset: 0;
    margin: auto;
    height: 90%;
    max-width: 90%;
    aspect-ratio: var(--a, 1);
  }
  .ccap {
    display: flex;
    align-items: center;
    gap: 6px;
    height: 34px;
    padding: 0 2px 0 4px;
    color: var(--muted);
    font-size: 12px;
  }
  .nm {
    flex: 1 1 0;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .cell.active .nm {
    color: var(--text);
  }
  /* PreviewMarks растягивается (flex: 1) — в подписи ему хватает своей ширины */
  .ccap :global(.mk) {
    flex: none;
  }
  .acts {
    display: flex;
    align-items: center;
    gap: 4px;
    opacity: 0;
    transition: opacity 0.12s;
  }
  .cell.active .acts,
  .cell:hover .acts,
  .cell:focus-within .acts {
    opacity: 1;
  }
  .ccap button {
    height: 24px;
    font-size: 12px;
    padding: 0 9px;
  }
  .sq {
    width: 24px;
    padding: 0;
    display: grid;
    place-items: center;
    background: transparent;
    color: var(--text);
  }
</style>
