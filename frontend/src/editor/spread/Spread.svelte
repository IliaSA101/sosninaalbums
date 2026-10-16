<script lang="ts">
  import { cubicOut } from 'svelte/easing'
  import type { RotRect, Settings, SpreadView } from '../../base/core/types'
  import { drag } from '../../base/drag/drag.svelte'
  import { store } from '../../base/core/store.svelte'
  import { gallery, pageSwitchable } from '../../panels/gallery/gallery.svelte'
  import { modes } from '../../base/prefs/view.svelte'
  import { placesel } from '../places/placesel.svelte'
  import { fieldClick } from '../places/placegesture'
  import { bgmode } from '../backdrop/bgmode.svelte'
  import BackdropLayer from '../backdrop/BackdropLayer.svelte'
  import PatternLayer from '../backdrop/PatternLayer.svelte'
  import BackdropEditor from '../backdrop/BackdropEditor.svelte'
  import Cell from '../photo/Cell.svelte'
  import CellOverlay from '../photo/CellOverlay.svelte'
  import StraightenLayer from '../photo/StraightenLayer.svelte'
  import { straighten } from '../photo/straighten.svelte'
  import GroupFrame from '../places/GroupFrame.svelte'
  import PlaceLayer from '../places/PlaceLayer.svelte'
  import { shapeCss } from '../../base/geom/shape'
  import SelFrame from '../places/SelFrame.svelte'
  import TextLayer from '../text/TextLayer.svelte'
  import StickerLayer from '../sticker/StickerLayer.svelte'
  import { sticker } from '../sticker/sticker.svelte'
  import { stickerOverflow } from '../sticker/sticker'
  import StickerOverlay from '../sticker/StickerOverlay.svelte'
  import NameEditor from '../captions/NameEditor.svelte'
  import { nameEdit } from '../captions/nameedit.svelte'
  import { nameTargetAt, takesNameTarget, type NameTarget } from '../captions/nametarget'
  import { edit } from '../text/edit.svelte'
  import { emptyField } from '../canvas/field'
  import { prefs } from '../../base/prefs/prefs.svelte'
  import { tip } from '../../base/ui/tip.svelte'
  import TextOverlay from '../text/TextOverlay.svelte'
  import { TextGestures } from '../text/textgesture.svelte'
  import { layerIndex } from '../places/layers'
  import { hasSeam, isCover, pageAt } from '../../base/geom/sheet'
  import { pickUnder } from '../canvas/pickunder.svelte'
  import CoverDrop from '../cover/CoverDrop.svelte'
  import CoverGuides from '../cover/CoverGuides.svelte'
  import CoverPlate from '../cover/CoverPlate.svelte'
  import CoverUnderlay from '../cover/CoverUnderlay.svelte'

  // peer — сосед в ряду разворотов (спека 012): рисуется как mini (лёгкий, без жестов), но с тенью и обводкой
  // листа — тот же экземпляр становится текущим сменой mini, без перемонтирования.
  let {
    spread,
    settings,
    width,
    mini = false,
    peer = false,
  }: { spread: SpreadView; settings: Settings; width: number; mini?: boolean; peer?: boolean } = $props()

  // Рамка разворота: от неё жесты текста считают экранные координаты.
  let root = $state<HTMLElement>()
  const tg = new TextGestures(() => ({ spread, settings, width, root }))

  // Размер и шов листа приходят из Go (sheet.ts): у листа виньетки шва нет.
  const sw = $derived(spread.w)
  const sh = $derived(spread.h)
  const seam = $derived(hasSeam(spread))
  const midPct = $derived(seam ? (spread.mid / sw) * 100 : 0)
  const height = $derived((width * sh) / sw)
  // Обложка (032): плашка корешка, зоны броска и направляющие; шов у неё — центр корешка, сгиба и «Авто» нет.
  const cover = $derived(isCover(spread) ? store.cover : null)
  // Безопасная зона (мм из Go): только на большом развороте и в крупном просмотре; у обложки её рисуют направляющие.
  const safe = $derived(!mini && !cover && modes.safe ? (store.view.safe ?? []) : [])
  // Предпросмотр карточки галереи — только на большом развороте.
  const preview = $derived(!mini && gallery.preview?.spread === spread.id)
  const rects: RotRect[] = $derived(preview && gallery.preview ? gallery.preview.rects : spread.rects)
  // Цвета страниц (левая, правая) считает Go: страница → альбом → белый.
  const bg = $derived(spread.bg ?? ['#ffffff', '#ffffff'])
  // Страница, с которой работают (группа в полосе или галерея страницы): вторая притемняется.
  const focus = $derived.by(() => {
    if (!seam || cover || mini || drag.active || store.spread?.id !== spread.id) return null
    const g = gallery.cur
    const f = store.hoverPage ?? (g?.spread === spread.id && g.scope !== 'spread' ? g.scope : null)
    if (!f || !pageSwitchable(spread.pages?.[f === 'left' ? 0 : 1])) return null
    return f
  })
  // Сторона держится и во время угасания, чтобы слой не перескакивал.
  let side = $state<'left' | 'right'>('left')
  $effect(() => {
    if (focus) side = focus
  })

  const spineOf = $derived(new Set((spread.grid ? (spread.places ?? []) : []).filter((p) => p.spine).map((p) => p.photo)))

  // Повёрнутые места вылетают за разворот (hold внутри): `.content` обрезает вылет, а ручки, SelFrame
  // и рамка текста лежат вне него. Без поворота обрезки нет — не режем выделение и подсветку у края.
  // Стикер может вылезать за лист (центр на листе, 048): печатается только лист, поэтому и на холсте обрезаем —
  // но только когда какой-то стикер реально вылез (с живой рамкой жеста), иначе режется подсветка у края.
  const turned = $derived(
    preview
      ? rects.some((r) => r.rot)
      : (!!spread.grid && (spread.places ?? []).some((p) => p.rot)) ||
          stickerOverflow(
            (spread.stickers ?? []).map((st) => sticker.boxOf(st)),
            sw,
            sh,
          ),
  )

  // Режим фона на этом развороте: места и тексты приглушены, сверху — рамка фона (BackdropEditor).
  const bgEdit = $derived(!mini && bgmode.cur?.spread === spread.id && store.spread?.id === spread.id)

  // Рамка выделения: у текущего большого разворота, не в предпросмотре галереи и не во время переноса фото.
  // Контролы выделенной ячейки (✥, ползунок, «!») и подсветка цели броска — над стеком, вне обрезки (CellOverlay).
  const cellUi = $derived(!mini && !preview && !bgEdit)
  const framed = $derived(!mini && !preview && !drag.active && !bgEdit && store.spread?.id === spread.id && placesel.selected.length > 0)

  // Рамка выделенных стикеров (048): те же условия, что у рамки места. Один — с ручками, в группе — обводки членов.
  const stickerFramed = $derived(
    !mini && !preview && !drag.active && !bgEdit && store.spread?.id === spread.id && placesel.selectedStickers.length > 0,
  )

  // Рамка группы — тот же набор условий, плюс выделено 2 и больше объектов (в том числе тексты).
  const grouped = $derived(!mini && !preview && !drag.active && !bgEdit && store.spread?.id === spread.id && placesel.isGroup)

  // Сгиб есть только у листа со швом; от него зависит белый уголок под листом.
  const hasFold = $derived(seam && !cover)

  // Подпись или заглушка с полями имени под курсором (037): курсор text и подсказка. Заглушки мышь не ловят,
  // поэтому цель ищем по точке (мм), а не по элементу. hint — невидимая рамка цели, от неё встаёт подсказка.
  let named = $state<NameTarget | null>(null)
  let hint = $state<HTMLElement>()
  $effect(() => {
    const el = hint
    if (!el) return
    tip.showAt(el, 'Двойной щелчок — ввести имя')
    return () => tip.hideFor(el)
  })

  // Цель по точке не знает, что выше (свободный текст, фото) и открыт ли редактор: решают признаки элемента.
  function takes(target: EventTarget | null): boolean {
    const el = target as Element | null
    return takesNameTarget({
      can: prefs.can('captions'),
      bgEdit,
      editing: edit.editing,
      empty: emptyField(el),
      onNamed: !!el?.closest?.('.hit.caption.named'),
    })
  }

  // Страница под курсором: «+ Место» встаёт на неё; заодно — цель ввода имени.
  function track(e: PointerEvent) {
    if (mini || store.spread?.id !== spread.id) return
    const r = (e.currentTarget as HTMLElement).getBoundingClientRect()
    const x = ((e.clientX - r.left) / r.width) * sw
    placesel.page = pageAt(spread, x)
    const t = drag.active || nameEdit.cur || !takes(e.target) ? null : nameTargetAt(spread, x, ((e.clientY - r.top) / r.height) * sh)
    const next = t?.who.length ? t : null
    if (next?.photo !== named?.photo) named = next
  }

  // Двойной щелчок по подписи по шаблону или заглушке: ввод имени вместо «вписать» (Stage.dbl его не увидит).
  function dbl(e: MouseEvent) {
    if (mini || store.spread?.id !== spread.id || !root || !takes(e.target)) return
    if (nameEdit.dbl(spread, e, root)) {
      e.stopPropagation()
      named = null
    }
  }

  // Щелчок по полю (и сквозь фото на фоне) только снимает выделение; с Shift/Ctrl — ничего (gestures.md);
  // после жеста над рамкой он не считается.
  const bgClick = fieldClick

  // Верх слоя фото: корешок «Авто» ложится над ними и под текстами (у фото и текстов общий z из Go).
  const photoTop = $derived(Math.max(0, ...(spread.frames ?? []).map((f) => layerIndex(f.z))))

  // Новое фото «вырастает» на своё место; в мини-развороте без анимации. Анимация transform затирает
  // inline `rotate(...)` повёрнутого слота на все 320 мс, поэтому поворот входит в каждый кадр.
  function appear(_node: Element, rot: number = 0) {
    if (mini) return { duration: 0 }
    const turn = rot ? `rotate(${rot}deg) ` : ''
    return {
      duration: 320,
      easing: cubicOut,
      css: (t: number) => `opacity:${t};transform:${turn}scale(${0.85 + 0.15 * t})`,
    }
  }
  function vanish(_node: Element) {
    return mini ? { duration: 0 } : { duration: 140, css: (t: number) => `opacity:${t}` }
  }
</script>

<div
  bind:this={root}
  class="spread"
  class:mini
  class:peer
  class:bgedit={bgEdit}
  class:over={!mini && drag.over === `spread:${spread.id}`}
  style:width="{width}px"
  style:height="{height}px"
  style:--mid="{midPct}%"
  data-drop={mini || cover ? undefined : 'spread'}
  data-spread={spread.id}
  role="presentation"
  onclick={bgClick}
  onpointermove={track}
  onpointerleave={() => (named = null)}
  ondblclick={dbl}
  style:cursor={named ? 'text' : undefined}
>
  <div class="page" class:whole={!seam} style:background={bg[0]}></div>
  {#if seam}<div class="page right" style:background={bg[1]}></div>{/if}
  <PatternLayer {spread} {width} />
  <BackdropLayer {spread} {mini} />
  {#if cover}
    <CoverPlate {cover} />
    {#if !mini}<CoverDrop {cover} />{/if}
  {/if}
  {#if spread.grid && seam && !cover}<div class="spine"></div>{/if}
  <!-- Белый уголок под листом по центру сгиба (как в SmartAlbums); только на большом развороте -->
  {#if !mini && hasFold}<div class="notch"></div>{/if}
  <!-- Один контекст наложения: слоты, пустые места, подписи и тексты с z-index = 10 + z (текст бывает под фото) -->
  <div class="content" class:turned use:pickUnder={{ spread: () => spread, root: () => root, tg, on: !mini }}>
    {#each spread.photos as id, i (id)}
      {@const r = rects[i]}
      {@const rot = preview ? (r?.rot ?? 0) : (spread.frames?.[i]?.cellRot ?? 0)}
      {#if r}
        <!-- Обёртка держит место и переезд при смене раскладки; Cell заполняет её. border-radius и у неё:
             иначе угол габарита круга ловил бы щелчок, а не отдавал его тому, что лежит ниже. -->
        <div
          class="slot"
          class:spine={!mini && spineOf.has(id)}
          title={!mini && spineOf.has(id) ? 'Место пересекает корешок, а альбом не раскрывается плоско' : undefined}
          style:z-index={layerIndex(spread.frames?.[i]?.z)}
          style:left="{(r.x / sw) * 100}%"
          style:top="{(r.y / sh) * 100}%"
          style:width="{(r.w / sw) * 100}%"
          style:height="{(r.h / sh) * 100}%"
          style:transform={rot ? `rotate(${rot}deg)` : undefined}
          style:border-radius={preview ? undefined : shapeCss(spread.frames?.[i]?.shape, spread.frames?.[i]?.radius, r.w, r.h)}
          in:appear={rot}
          out:vanish
        >
          <Cell {spread} {i} {r} {mini} {preview} />
        </div>
      {/if}
    {/each}
    {#if spread.grid && !preview}<PlaceLayer {spread} {mini} />{/if}
    <TextLayer {spread} {mini} {tg} />
    <StickerLayer {spread} {mini} {width} />
    <!-- В «Авто» корешок виден над фото (как до 0.5.0), но под текстами: z верхнего фото, позже слотов в DOM -->
    {#if !spread.grid && seam && !cover}<div class="spine front" style:z-index={photoTop}></div>{/if}
  </div>
  {#if !mini}<TextOverlay {spread} {width} {tg} />{/if}
  {#if named}
    <!-- key: при переходе на соседнюю подпись без зазора подсказка переставляется заново -->
    {#key named.photo}
      <div
        bind:this={hint}
        class="namehint"
        style:left="{(named.box.x / sw) * 100}%"
        style:top="{(named.box.y / sh) * 100}%"
        style:width="{(named.box.w / sw) * 100}%"
        style:height="{(named.box.h / sh) * 100}%"
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
  {#if !mini && !preview && straighten.cur?.spread === spread.id && store.spread?.id === spread.id}<StraightenLayer {spread} />{/if}
  {#if !mini && seam}
    <div
      class="dim"
      class:on={focus}
      style:left={side === 'left' ? 'var(--mid)' : '0'}
      style:width={side === 'left' ? 'calc(100% - var(--mid))' : 'var(--mid)'}
    ></div>
    <div
      class="focus"
      class:on={focus}
      style:left={side === 'right' ? 'var(--mid)' : '0'}
      style:width={side === 'right' ? 'calc(100% - var(--mid))' : 'var(--mid)'}
    ></div>
  {/if}
  {#each safe as r, i (i)}
    <div
      class="safe"
      style:left="{(r.x / sw) * 100}%"
      style:top="{(r.y / sh) * 100}%"
      style:width="{(r.w / sw) * 100}%"
      style:height="{(r.h / sh) * 100}%"
    ></div>
  {/each}
  {#if cover && !mini}
    <CoverUnderlay {cover} />
    <CoverGuides {cover} />
  {/if}
  <!-- На сетке с местами у каждого пустого места своя метка «+ фото» — общая подсказка её перекрывает -->
  {#if spread.photos.length === 0 && !mini && !spread.places?.length && !cover}
    <div class="empty">Перетащите сюда фото из ленты</div>
  {/if}
</div>

<style>
  /* Режим фона: места, фото, подписи и тексты (весь .content) приглушены и мышь не ловят (её берёт BackdropEditor). */
  .spread.bgedit > :is(.content, .empty) {
    opacity: 0.3;
  }
  .spread.bgedit > .content,
  .spread.bgedit > .content :global(*),
  .spread.bgedit > :global(.tover),
  .spread.bgedit > :global(.tover *) {
    pointer-events: none !important;
  }
  .spread {
    position: relative;
    background: #fff;
    box-shadow: 0 2px 18px rgba(0, 0, 0, 0.35);
    flex: none;
    transition: outline-color 0.15s;
    outline: 3px solid transparent;
    outline-offset: 4px;
  }
  .spread.over {
    outline-color: var(--accent);
  }
  .spread.mini:not(.peer) {
    box-shadow: none;
    outline: none;
  }
  .spine {
    position: absolute;
    left: var(--mid);
    top: 0;
    bottom: 0;
    width: 1px;
    background: rgba(0, 0, 0, 0.12);
    z-index: 2;
    pointer-events: none;
  }
  /* Уголок острием вниз — пара серому язычку ленты (Timeline .pointer, тот же размер), в px экрана:
     не растёт с зумом. top: 100% и left: var(--mid) держат его на сгибе при любом зуме. */
  .notch {
    position: absolute;
    top: 100%;
    left: var(--mid);
    margin-left: -9px;
    width: 0;
    height: 0;
    border-left: 9px solid transparent;
    border-right: 9px solid transparent;
    border-top: 8px solid #fff;
    pointer-events: none;
    z-index: 2;
  }
  /* В «Авто» корешок лежит в `.content` (z задан в разметке); в сетке — под слоями */
  /* Подложки страниц: цвет страницы, а не разворота */
  .page {
    position: absolute;
    left: 0;
    top: 0;
    bottom: 0;
    width: var(--mid);
    pointer-events: none;
  }
  .page.whole {
    width: 100%;
  }
  .page.right {
    left: var(--mid);
    width: auto;
    right: 0;
  }
  /* Один контекст наложения: слоты, пустые места, подписи и тексты лежат в нём по z из Go (10 + z). Выше него
     и вне обрезки — рамки, ручки, мини-панель, направляющие, затемнение, безопасная зона. */
  .content {
    position: absolute;
    inset: 0;
    z-index: 3;
    isolation: isolate;
    pointer-events: none;
  }
  .content > .slot,
  .content > :global(.place) {
    pointer-events: auto;
  }
  /* Без поворота обрезки нет — не режем подсветку и индикаторы вставки у края */
  .content.turned {
    overflow: clip;
  }
  .slot.spine::after {
    content: '';
    position: absolute;
    inset: 0;
    border: 2px solid #e0803a;
    pointer-events: none;
    z-index: 3;
  }
  .slot {
    position: absolute;
    /* Главный эффект: при смене раскладки фото плавно переезжают. */
    transition:
      left var(--move),
      top var(--move),
      width var(--move),
      height var(--move);
  }
  .mini .slot {
    transition: none;
  }
  /* Безопасная зона: двойной пунктир (белый + тёмный), виден и на фото, и на белом фоне */
  .safe {
    position: absolute;
    z-index: 6;
    pointer-events: none;
    box-sizing: border-box;
    border: 1.5px dashed var(--accent);
    box-shadow:
      0 0 0 1px rgba(0, 0, 0, 0.45),
      inset 0 0 0 1px rgba(0, 0, 0, 0.45);
  }
  .dim,
  .focus {
    position: absolute;
    top: 0;
    bottom: 0;
    z-index: 5;
    pointer-events: none;
    opacity: 0;
    transition: opacity 0.15s;
  }
  .dim {
    background: rgba(0, 0, 0, 0.45);
  }
  .focus {
    box-shadow: inset 0 0 0 2px var(--accent);
  }
  .dim.on,
  .focus.on {
    opacity: 1;
  }
  .namehint {
    position: absolute;
    pointer-events: none;
  }
  .empty {
    position: absolute;
    inset: 0;
    display: grid;
    place-items: center;
    color: #999;
    font-size: 15px;
    pointer-events: none;
  }
</style>
