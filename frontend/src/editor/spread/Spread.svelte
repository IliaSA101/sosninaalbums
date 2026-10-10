<script lang="ts">
  import { cubicOut } from 'svelte/easing'
  import type { RotRect, Settings, SpreadView } from '../../base/core/types'
  import { drag } from '../../base/drag/drag.svelte'
  import { store } from '../../base/core/store.svelte'
  import { gallery, pageSwitchable } from '../../panels/layouts/gallery.svelte'
  import { modes } from '../../base/prefs/view.svelte'
  import { placesel } from '../places/placesel.svelte'
  import { fieldClick } from '../places/placegesture'
  import { bgmode } from '../backdrop/bgmode.svelte'
  import BackdropLayer from '../backdrop/BackdropLayer.svelte'
  import BackdropEditor from '../backdrop/BackdropEditor.svelte'
  import Cell from '../photo/Cell.svelte'
  import CellOverlay from '../photo/CellOverlay.svelte'
  import GroupFrame from '../places/GroupFrame.svelte'
  import PlaceLayer from '../places/PlaceLayer.svelte'
  import SelFrame from '../places/SelFrame.svelte'
  import TextLayer from '../text/TextLayer.svelte'
  import TextOverlay from '../text/TextOverlay.svelte'
  import { TextGestures } from '../text/textgesture.svelte'
  import { layerIndex } from '../places/layers'
  import { pickUnder } from '../canvas/pickunder.svelte'

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

  const sw = $derived(settings.pageW * 2)
  const height = $derived((width * settings.pageH) / sw)
  // Безопасная зона (мм из Go): только на большом развороте и в крупном просмотре.
  const safe = $derived(!mini && modes.safe ? (store.view.safe ?? []) : [])
  // Предпросмотр карточки галереи — только на большом развороте.
  const preview = $derived(!mini && gallery.preview?.spread === spread.id)
  const rects: RotRect[] = $derived(preview && gallery.preview ? gallery.preview.rects : spread.rects)
  // Цвета страниц (левая, правая) считает Go: страница → альбом → белый.
  const bg = $derived(spread.bg ?? ['#ffffff', '#ffffff'])
  // Страница, с которой работают (группа в полосе или галерея страницы): вторая притемняется.
  const focus = $derived.by(() => {
    if (mini || drag.active || store.spread?.id !== spread.id) return null
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
  const turned = $derived(preview ? rects.some((r) => r.rot) : !!spread.grid && (spread.places ?? []).some((p) => p.rot))

  // Режим фона на этом развороте: места и тексты приглушены, сверху — рамка фона (BackdropEditor).
  const bgEdit = $derived(!mini && bgmode.cur?.spread === spread.id && store.spread?.id === spread.id)

  // Рамка выделения: у текущего большого разворота, не в предпросмотре галереи и не во время переноса фото.
  // Контролы выделенной ячейки (✥, ползунок, «!») и подсветка цели броска — над стеком, вне обрезки (CellOverlay).
  const cellUi = $derived(!mini && !preview && !bgEdit)
  const framed = $derived(!mini && !preview && !drag.active && !bgEdit && store.spread?.id === spread.id && placesel.selected.length > 0)

  // Рамка группы — тот же набор условий, плюс выделено 2 и больше объектов (в том числе тексты).
  const grouped = $derived(!mini && !preview && !drag.active && !bgEdit && store.spread?.id === spread.id && placesel.isGroup)

  // Есть ли у листа сгиб (разворот из двух страниц). Сейчас он у всех; когда появятся листы без разворота,
  // выключить здесь: от этого зависит белый уголок под листом.
  const hasFold = $derived(true)

  // Страница под курсором: «+ Место» встаёт на неё.
  function track(e: PointerEvent) {
    if (mini || store.spread?.id !== spread.id) return
    const r = (e.currentTarget as HTMLElement).getBoundingClientRect()
    placesel.page = e.clientX - r.left < r.width / 2 ? 0 : 1
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
  data-drop={mini ? undefined : 'spread'}
  data-spread={spread.id}
  role="presentation"
  onclick={bgClick}
  onpointermove={track}
>
  <div class="page" style:background={bg[0]}></div>
  <div class="page right" style:background={bg[1]}></div>
  <BackdropLayer {spread} {settings} {mini} />
  {#if spread.grid}<div class="spine"></div>{/if}
  <!-- Белый уголок под листом по центру сгиба (как в SmartAlbums); только на большом развороте -->
  {#if !mini && hasFold}<div class="notch"></div>{/if}
  <!-- Один контекст наложения: слоты, пустые места, подписи и тексты с z-index = 10 + z (текст бывает под фото) -->
  <div class="content" class:turned use:pickUnder={{ spread: () => spread, root: () => root, tg, on: !mini }}>
    {#each spread.photos as id, i (id)}
      {@const r = rects[i]}
      {@const rot = preview ? (r?.rot ?? 0) : (spread.frames?.[i]?.cellRot ?? 0)}
      {#if r}
        <!-- Обёртка держит место и переезд при смене раскладки; Cell заполняет её -->
        <div
          class="slot"
          class:spine={!mini && spineOf.has(id)}
          title={!mini && spineOf.has(id) ? 'Место пересекает корешок, а альбом не раскрывается плоско' : undefined}
          style:z-index={layerIndex(spread.frames?.[i]?.z)}
          style:left="{(r.x / sw) * 100}%"
          style:top="{(r.y / settings.pageH) * 100}%"
          style:width="{(r.w / sw) * 100}%"
          style:height="{(r.h / settings.pageH) * 100}%"
          style:transform={rot ? `rotate(${rot}deg)` : undefined}
          in:appear={rot}
          out:vanish
        >
          <Cell {spread} {i} {r} {mini} {preview} />
        </div>
      {/if}
    {/each}
    {#if spread.grid && !preview}<PlaceLayer {spread} {settings} {mini} />{/if}
    <TextLayer {spread} {settings} {mini} {tg} />
    <!-- В «Авто» корешок виден над фото (как до 0.5.0), но под текстами: z верхнего фото, позже слотов в DOM -->
    {#if !spread.grid}<div class="spine front" style:z-index={photoTop}></div>{/if}
  </div>
  {#if !mini}<TextOverlay {spread} {settings} {width} {tg} />{/if}
  {#if cellUi}<CellOverlay {spread} {settings} {width} />{/if}
  {#if framed}<SelFrame {spread} {settings} {width} />{/if}
  {#if grouped}<GroupFrame {spread} {settings} {width} />{/if}
  {#if bgEdit}<BackdropEditor {spread} {settings} {width} />{/if}
  {#if !mini}
    <div class="dim" class:on={focus} style:left={side === 'left' ? '50%' : '0'}></div>
    <div class="focus" class:on={focus} style:left={side === 'right' ? '50%' : '0'}></div>
  {/if}
  {#each safe as r, i (i)}
    <div
      class="safe"
      style:left="{(r.x / sw) * 100}%"
      style:top="{(r.y / settings.pageH) * 100}%"
      style:width="{(r.w / sw) * 100}%"
      style:height="{(r.h / settings.pageH) * 100}%"
    ></div>
  {/each}
  <!-- На сетке с местами у каждого пустого места своя метка «+ фото» — общая подсказка её перекрывает -->
  {#if spread.photos.length === 0 && !mini && !spread.places?.length}
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
    left: 50%;
    top: 0;
    bottom: 0;
    width: 1px;
    background: rgba(0, 0, 0, 0.12);
    z-index: 2;
    pointer-events: none;
  }
  /* Уголок острием вниз — пара серому язычку ленты (Timeline .pointer, тот же размер), в px экрана:
     не растёт с зумом. top: 100% и left: 50% держат его на сгибе при любом зуме. */
  .notch {
    position: absolute;
    top: 100%;
    left: 50%;
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
    width: 50%;
    pointer-events: none;
  }
  .page.right {
    left: 50%;
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
    width: 50%;
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
