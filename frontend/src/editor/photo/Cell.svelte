<script lang="ts">
  import { screenDpr } from '../../base/ui/dpr.svelte'
  import { primaryPress } from '../../base/core/platform'
  import { watchSize } from '../../base/ui/size'
  import type { Frame, Rect, SpreadView } from '../../base/core/types'
  import { drag } from '../../base/drag/drag.svelte'
  import { store } from '../../base/core/store.svelte'
  import { thumbUrl, previewUrl, wantPreview } from '../../base/core/media'
  import { cancelOnUnmount } from '../../base/ui/imgcancel'
  import { colorBadge } from './color'
  import { DEFAULT_FRAME } from '../../base/geom/frame'
  import { liveImage, liveTransform } from '../tilt/tiltlive'
  import { framelive } from './framelive.svelte'
  import { adjlive } from './adjlive.svelte'
  import { aabbOf, uprightAt } from '../../base/geom/rotbox'
  import { cornerInset, markAt, outlineOf, shapeCss } from '../../base/geom/shape'
  import { opacityOf } from '../backdrop/fade'
  import BorderOverlay from '../border/BorderOverlay.svelte'
  import { borderPx, borderSrc, maskUrlCss, watchMask, windowOf, winStyle } from '../border/border'
  import { borderTip } from '../border/borderstate'
  import { borderlive } from '../border/borderlive.svelte'
  import { placesel } from '../places/placesel.svelte'
  import { prefs } from '../../base/prefs/prefs.svelte'
  import { photoui } from './photoui.svelte'
  import { gestureClickAllowed } from '../places/placegesture'
  import { moveSelected } from '../places/groupgesture'
  import { additive, bodyAction } from '../places/gesture'
  import MissPhoto from '../../base/ui/MissPhoto.svelte'
  import DpiMark from './DpiMark.svelte'
  import { missTip } from '../../panels/library/missing'
  import { autocheck } from '../../panels/folders/autocheck'

  // Ячейка фото: рамка из раскладки Go, внутри кадр (спека 006). Геометрию картинки
  // присылает Go (frames[i].img); локально считаем только предпросмотр до отпускания.
  let {
    spread,
    i,
    r,
    mini = false,
    preview = false,
    width = 0,
  }: {
    spread: SpreadView
    i: number
    r: Rect // ячейка, мм разворота (при предпросмотре галереи — не из View)
    mini?: boolean
    preview?: boolean // r взят из карточки галереи: картинку считаем сами
    width?: number // ширина листа на экране, px: от неё размер PNG рамочки (079)
  } = $props()

  const NINTHS = [1, 2, 3, 4, 5, 6, 7, 8] // линии сетки наклона 9×9 (059)

  const id = $derived(spread.photos[i])
  // Место сетки этого фото (спека 007): цель drag «place» вместо «cell»; contain — картинка вписана целиком.
  const place = $derived(spread.grid ? spread.places?.find((p) => p.photo === id) : undefined)
  const contain = $derived(place?.fit === 'contain')
  // У contain миниатюра-подложка просвечивала бы сквозь прозрачные места картинки: после загрузки превью убираем.
  let previewReady = $state(false)
  const fv = $derived(spread.frames?.[i])
  const frame = $derived<Frame>(fv ? { s: fv.s, x: fv.x, y: fv.y, r: fv.r } : DEFAULT_FRAME)
  // Повёрнутое место (слот в Spread крутится вокруг центра): значок dpi стоит у угла описанного прямоугольника.
  const rot = $derived(preview ? 0 : (fv?.cellRot ?? 0))
  // Форма места (044): в галерее (preview) места не из View — формы нет. Радиус в мм приходит из Go.
  const shape = $derived(preview ? undefined : fv?.shape)
  const radius = $derived(preview ? 0 : placesel.liveRadius([place?.id, id], fv?.radius ?? 0, r.w, r.h))
  const radiusCss = $derived(shapeCss(shape, radius, r.w, r.h))
  // Рамочка места (079): фото сидит в окне внутри места; форма окна и кадр считаются по нему. Без рамочки окно = место.
  const border = $derived(preview ? undefined : place?.border)
  // Ползунок «Толщина» / «Размер» в карточке ведёт отступы окна на ходу (borderlive), Go зовётся на отпускании.
  const liveIns = $derived(border ? borderlive.insOf(spread.id, [place?.id, id], border) : undefined)
  const w = $derived(windowOf(r, fv, radius, liveIns ?? border?.ins, preview, liveIns !== undefined))
  const outline = $derived(outlineOf(w.shape, w.radius))
  const clipCss = $derived(shapeCss(w.shape, w.radius, w.win.w, w.win.h))
  const ws = $derived(winStyle(r, w))
  // Значки в углу вписанного в окно прямоугольника: круг не должен их обрезать.
  const inset = $derived(cornerInset(w.shape, w.radius, w.win.w, w.win.h))
  // Прозрачность — у фото (у .clip), рамочка поверх не прозрачнеет.
  const fade = $derived(preview ? 0 : borderlive.fadeOf(spread.id, id, fv?.fade ?? 0))
  // Рамочка пропала или новее программы (U5): подсказка на всей ячейке, как у стикера; у пропавшего фото своя (MissPhoto).
  const borderNote = $derived(border && !mini ? borderTip(border, store.missing, store.missingNames) : undefined)
  const bpx = $derived(border ? borderPx(r.w, spread.w, width, screenDpr.value) : 0)
  // Маска окна рамочки-картинки и мягкого края: ставится, когда PNG открылся (иначе фото пропало бы целиком).
  const maskSrc = $derived(border?.maskUrl ? borderSrc(border.maskUrl, bpx) : '')
  let maskReady = $state('')
  $effect(() => watchMask(maskSrc, (u) => (maskReady = u)))
  const dpiAt = $derived(markAt(r, rot, shape, radius, 6) ?? (rot ? uprightAt(r, rot, aabbOf({ ...r, rot }), 6) : undefined))
  // Пропорции файла: img лежит в осях фото, поэтому и при повороте на 90° это они, а не рамки.
  const aspect = $derived(fv && fv.img.h > 0 ? fv.img.w / fv.img.h : r.w / r.h)

  // Кадр, который тянут (✥, ползунки на ячейке и в карточке справа — всё через framelive), до отпускания.
  const pre = $derived(mini ? null : framelive.of(id))
  const cur = $derived(pre ?? frame)
  const img = $derived(pre || preview || !fv || liveIns ? liveImage(w.win, aspect, cur, contain, outline) : fv.img)

  // Ч/б и тон (068): строка filter из Go; пока тянут ползунок карточки — живая. Фильтр на самой картинке: поле страницы не задет.
  const tone = $derived(adjlive.filterOf(id, fv))

  const selected = $derived(!mini && store.selectedCell === id)
  const missing = $derived(store.missing.includes(id))

  let el = $state<HTMLDivElement>()
  // Размер ячейки на экране (CSS px, с учётом зума холста): на 17+ фото крупное превью — только крупным ячейкам.
  let cw = $state(0)
  let ch = $state(0)
  const longSide = $derived(Math.max((img.w / r.w) * cw, (img.h / r.h) * ch))
  const hiRes = $derived(!mini && wantPreview(spread.photos.length, longSide, screenDpr.value))

  // Тело (спека 009, gestures.md): «Авто» — перенос фото (обмен, вставка, другой разворот, лента);
  // «Ручная» — перенос места: выделяет фото и двигает рамку, у члена группы — всю группу. Кадр двигает ✥.
  function startCell(e: PointerEvent) {
    const root = el?.closest<HTMLElement>('.spread')
    const here = !!root && spread.id === store.spread?.id
    const inGroup = placesel.isGroup && placesel.refs.includes(id)
    const act = bodyAction({ grid: !!spread.grid && prefs.can('manual'), selected, inGroup })
    if (act === 'drag' || !here || !root || !primaryPress(e)) {
      drag.start(e, () => ({ kind: 'photos', ids: [id], fromSpread: spread.id }))
      return
    }
    if (multi(e)) return // Shift/Ctrl добавляют в выделение на click
    if (!placesel.refs.includes(id)) placesel.pick(id, false)
    moveSelected(e, root, () => placesel.select([id]))
  }

  // Shift/Ctrl собирают группу с «Стандартного»; ниже — обычный щелчок.
  const multi = (e: MouseEvent | PointerEvent) => additive(e) && prefs.can('groupSelect')

  // Щелчок выделяет; Shift/Ctrl добавляют в выделение или убирают из него (группа).
  function select(e: MouseEvent) {
    e.stopPropagation()
    if (!drag.clickAllowed() || !gestureClickAllowed()) return
    if (multi(e)) placesel.pick(id, true)
    else store.selectedCell = id
  }

  const pct = (v: number, of: number) => `${(v / of) * 100}%`
</script>

<div
  bind:this={el}
  {@attach (n) => (mini ? undefined : watchSize(n, (w, h) => ((cw = w), (ch = h))))}
  class="cell"
  style:border-radius={radiusCss}
  title={borderNote}
  class:selected
  class:missing
  class:lifted={!mini && drag.active && drag.payload?.kind === 'photos' && drag.payload.fromSpread && drag.payload.ids[0] === id}
  data-drop={mini ? undefined : place ? 'place' : 'cell'}
  data-spread={spread.id}
  data-index={place ? undefined : i}
  data-place={place?.id}
  data-photo={id}
  role="presentation"
  onpointerdown={mini ? undefined : startCell}
  onclick={mini ? undefined : select}
  oncontextmenu={mini || preview ? undefined : (e) => photoui.open(e, id)}
>
  <div
    class="clip"
    class:contain
    class:win={w.windowed}
    class:masked={maskReady !== ''}
    style:left={ws?.left}
    style:top={ws?.top}
    style:width={ws?.width}
    style:height={ws?.height}
    style:border-radius={clipCss}
    style:opacity={opacityOf(fade)}
    style:--m={maskUrlCss(maskReady)}
  >
    {#if missing}
      <MissPhoto size={mini ? 12 : 28} tip={missTip(id, store.missingNames)} />
    {:else}
      {@const pos = `left:${pct(img.x - w.win.x, w.win.w)};top:${pct(img.y - w.win.y, w.win.h)};width:${pct(img.w, w.win.w)};height:${pct(img.h, w.win.h)}`}
      <img
        class="ph"
        class:gone={contain && previewReady}
        style={pos}
        style:transform={liveTransform(cur, fv)}
        style:filter={tone}
        src={thumbUrl(id)}
        {@attach cancelOnUnmount}
        alt=""
        draggable="false"
        onerror={() => autocheck.soon(true)}
      />
      {#if hiRes}<img
          class="ph"
          style={pos}
          style:transform={liveTransform(cur, fv)}
          style:filter={tone}
          src={previewUrl(id)}
          alt=""
          draggable="false"
          loading="lazy"
          {@attach cancelOnUnmount}
          onload={() => (previewReady = true)}
        />{/if}
    {/if}
    {#if framelive.tilting(id)}
      <!-- Наклон (с дуги и из карточки): сетка 9×9, трети ярче — горизонт ровняют по частым линиям (059) -->
      {#each NINTHS as k (k)}
        <span class="v" class:third={k % 3 === 0} class:ninth={k % 3 !== 0} style:left="{(k * 100) / 9}%"></span>
        <span class="h" class:third={k % 3 === 0} class:ninth={k % 3 !== 0} style:top="{(k * 100) / 9}%"></span>
      {/each}
    {:else if framelive.panning(id)}
      <!-- Сетка третей: пока идёт сдвиг кадра -->
      <span class="third v" style:left="33.333%"></span>
      <span class="third v" style:left="66.666%"></span>
      <span class="third h" style:top="33.333%"></span>
      <span class="third h" style:top="66.666%"></span>
    {/if}
    {#if !mini}
      {@const badge = colorBadge(store.byId.get(id))}
      {#if badge}<span
          class="badge {badge.level}"
          style:left={clipCss ? `calc(6px + ${(inset.x / w.win.w) * 100}%)` : undefined}
          style:bottom={clipCss ? `calc(6px + ${(inset.y / w.win.h) * 100}%)` : undefined}
          title={badge.title}>{badge.label}</span
        >{/if}
    {/if}
  </div>
  {#if border}<BorderOverlay {border} px={bpx} {mini} />{/if}
  <!-- У выделенной ячейки «!» dpi, ✥ и ползунок рисует CellOverlay (выше стека, не обрезаются) -->
  {#if !mini && (!selected || preview) && !missing}<DpiMark dpi={fv?.dpi ?? 0} minDPI={store.view.minDPI} at={dpiAt} />{/if}
</div>

<style>
  /* Положение и переезд при смене раскладки — у обёртки в Spread; ячейка заполняет её. */
  .cell {
    position: absolute;
    inset: 0;
    cursor: grab;
    transition:
      opacity 0.2s,
      box-shadow 0.15s;
  }
  :global(.spread.mini) .cell {
    cursor: inherit;
    transition: none;
  }
  /* Кадр обрезается ячейкой; рамка выделения и индикаторы вставки лежат снаружи. */
  .clip {
    position: absolute;
    inset: 0;
    overflow: hidden;
    background: #ddd;
  }
  /* Окно рамочки (079): позиция задана стилем, а не inset. */
  .clip.win {
    right: auto;
    bottom: auto;
  }
  /* Маска окна рамочки-картинки / мягкого края: пересекается с обрезкой по border-radius. Префикс — для WKWebView. */
  .clip.masked {
    -webkit-mask: var(--m) center / 100% 100% no-repeat;
    mask: var(--m) center / 100% 100% no-repeat;
  }
  /* Вписанное целиком: вокруг картинки виден фон страницы, а не серая подложка. */
  .clip.contain {
    background: transparent;
  }
  .ph.gone {
    visibility: hidden;
  }
  .ph {
    position: absolute;
    max-width: none;
    display: block;
    user-select: none;
  }
  /* Рамку выделения рисует SelFrame, обмен и вставку — CellOverlay (выше стека). */
  .cell.lifted {
    opacity: 0.35;
  }
  .third {
    position: absolute;
    pointer-events: none;
    background: rgba(255, 255, 255, 0.75);
    box-shadow: 0 0 0 0.5px rgba(0, 0, 0, 0.45);
  }
  /* Промежуточные линии сетки 9×9 — вдвое прозрачнее третей */
  .ninth {
    position: absolute;
    pointer-events: none;
    background: rgba(255, 255, 255, 0.38);
    box-shadow: 0 0 0 0.5px rgba(0, 0, 0, 0.22);
  }
  .third.v,
  .ninth.v {
    top: 0;
    bottom: 0;
    width: 1px;
  }
  .third.h,
  .ninth.h {
    left: 0;
    right: 0;
    height: 1px;
  }
  /* Значок цветового профиля на ячейке: внизу слева */
  .badge {
    position: absolute;
    left: 6px;
    bottom: 6px;
    min-width: 18px;
    height: 18px;
    padding: 0 5px;
    box-sizing: border-box;
    border-radius: 9px;
    font-size: 11px;
    font-weight: 700;
    line-height: 18px;
    text-align: center;
    background: rgba(0, 0, 0, 0.65);
    color: #fff;
    z-index: 3;
  }
  .badge.warn {
    background: #e0803a;
    color: #1e1e1e;
  }
</style>
