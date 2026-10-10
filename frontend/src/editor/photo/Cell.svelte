<script lang="ts">
  import type { Frame, Rect, SpreadView } from '../../base/core/types'
  import { drag } from '../../base/drag/drag.svelte'
  import { store } from '../../base/core/store.svelte'
  import { thumbUrl, previewUrl } from '../../base/core/media'
  import { colorBadge } from './color'
  import { DEFAULT_FRAME, photoCss, placeImage } from '../../base/geom/frame'
  import { framelive } from './framelive.svelte'
  import { aabbOf, uprightAt } from '../../base/geom/rotbox'
  import { placesel } from '../places/placesel.svelte'
  import { prefs } from '../../base/prefs/prefs.svelte'
  import { photoui } from './photoui.svelte'
  import { gestureClickAllowed } from '../places/placegesture'
  import { moveSelected } from '../places/groupgesture'
  import { additive, bodyAction } from '../places/gesture'
  import Icon from '../../base/ui/Icon.svelte'
  import DpiMark from './DpiMark.svelte'

  // Ячейка фото: рамка из раскладки Go, внутри кадр (спека 006). Геометрию картинки
  // присылает Go (frames[i].img); локально считаем только предпросмотр до отпускания.
  let {
    spread,
    i,
    r,
    mini = false,
    preview = false,
  }: {
    spread: SpreadView
    i: number
    r: Rect // ячейка, мм разворота (при предпросмотре галереи — не из View)
    mini?: boolean
    preview?: boolean // r взят из карточки галереи: картинку считаем сами
  } = $props()

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
  const dpiAt = $derived(rot ? uprightAt(r, rot, aabbOf({ ...r, rot }), 6) : undefined)
  // Пропорции файла: img лежит в осях фото, поэтому и при повороте на 90° это они, а не рамки.
  const aspect = $derived(fv && fv.img.h > 0 ? fv.img.w / fv.img.h : r.w / r.h)

  // Кадр, который тянут (✥, ползунки на ячейке и в карточке справа — всё через framelive), до отпускания.
  const pre = $derived(mini ? null : framelive.of(id))
  const cur = $derived(pre ?? frame)
  const img = $derived(pre || preview || !fv ? placeImage(r, aspect, cur) : fv.img)

  const selected = $derived(!mini && store.selectedCell === id)
  const missing = $derived(store.missing.includes(id))

  let el = $state<HTMLDivElement>()

  // Тело (спека 009, gestures.md): «Авто» — перенос фото (обмен, вставка, другой разворот, лента);
  // «Ручная» — перенос места: выделяет фото и двигает рамку, у члена группы — всю группу. Кадр двигает ✥.
  function startCell(e: PointerEvent) {
    const root = el?.closest<HTMLElement>('.spread')
    const here = !!root && spread.id === store.spread?.id
    const inGroup = placesel.isGroup && placesel.refs.includes(id)
    const act = bodyAction({ grid: !!spread.grid && prefs.can('manual'), selected, inGroup })
    if (act === 'drag' || !here || !root || e.button !== 0) {
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
  class="cell"
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
  <div class="clip" class:contain>
    {#if missing}
      <div class="miss" title="Файл не найден"><Icon name="photo-minus" size={mini ? 12 : 28} stroke={1.8} /></div>
    {:else}
      {@const pos = `left:${pct(img.x - r.x, r.w)};top:${pct(img.y - r.y, r.h)};width:${pct(img.w, r.w)};height:${pct(img.h, r.h)}`}
      <img
        class="ph"
        class:gone={contain && previewReady}
        style={pos}
        style:transform={photoCss(fv)}
        src={thumbUrl(id)}
        alt=""
        draggable="false"
      />
      {#if !mini}<img
          class="ph"
          style={pos}
          style:transform={photoCss(fv)}
          src={previewUrl(id)}
          alt=""
          draggable="false"
          loading="lazy"
          onload={() => (previewReady = true)}
        />{/if}
    {/if}
    {#if framelive.panning(id) || framelive.tilting(id)}
      <!-- Сетка третей: только пока идёт сдвиг или тянут наклон -->
      <span class="third v" style:left="33.333%"></span>
      <span class="third v" style:left="66.666%"></span>
      <span class="third h" style:top="33.333%"></span>
      <span class="third h" style:top="66.666%"></span>
    {/if}
    {#if !mini}
      {@const badge = colorBadge(store.byId.get(id))}
      {#if badge}<span class="badge {badge.level}" title={badge.title}>{badge.label}</span>{/if}
    {/if}
  </div>
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
  /* Фото нет на диске: серая ячейка со значком */
  .miss {
    position: absolute;
    inset: 0;
    display: grid;
    place-items: center;
    background: #6a6a6e;
    color: #cfcfd2;
  }
  .third {
    position: absolute;
    pointer-events: none;
    background: rgba(255, 255, 255, 0.75);
    box-shadow: 0 0 0 0.5px rgba(0, 0, 0, 0.45);
  }
  .third.v {
    top: 0;
    bottom: 0;
    width: 1px;
  }
  .third.h {
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
