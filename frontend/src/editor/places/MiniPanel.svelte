<script lang="ts">
  import type { Rect, SpreadView } from '../../base/core/types'
  import { store } from '../../base/core/store.svelte'
  import { placesel } from './placesel.svelte'
  import { edit } from '../text/edit.svelte'
  import { nameEdit } from '../captions/nameedit.svelte'
  import { captionBottom, captionBoxes } from '../captions/nametarget'
  import { prefs } from '../../base/prefs/prefs.svelte'
  import { isMac } from '../../base/core/platform'
  import { modes } from '../../base/prefs/view.svelte'
  import { drag } from '../../base/drag/drag.svelte'
  import { tip, tipped } from '../../base/ui/tip.svelte'
  import {
    arcRoom,
    miniContext,
    miniItems,
    swapTip,
    type LayerOp,
    sameView,
    zoomLift,
    MINI_H,
    NO_ROOM,
    type MiniId,
    type MiniView,
  } from './minipanel'
  import { arcWhereOf, colHeightOf, placeMini } from './minicol'
  import { measureMini, panelOutside, sameOrg, type Org } from './miniview'
  import { portal } from '../../base/ui/portal'
  import { MoreList, Through } from './ministate.svelte'
  import { miniSpot } from './minispot.svelte'
  import { canvas } from '../canvas/canvas.svelte'
  import { panels } from '../inspector/panelmode.svelte'
  import { row } from '../spread/spreadrow.svelte'
  import { tiltmode } from '../tilt/tiltmode.svelte'
  import { onMount } from 'svelte'
  import { isTextField } from '../../shell/menu/keys'
  import Icon from '../../base/ui/Icon.svelte'
  import MenuList from '../../base/ui/MenuList.svelte'
  import type { MenuNode } from '../../base/ui/menulist'

  // Мини-панель выделенного (спека 009, 108): столбец значков сбоку от фото (владелец 2026-10-23, owner-ux §1) — справа,
  // иначе слева, иначе внутри у правого края (minicol.ts); не помещается — прежняя полоса: под рамкой (и под её подписью,
  // 037), иначе над ней выше ⟳, иначе внутри у нижнего края, иначе внутри у верха (2026-10-17). Состав — minipanel.ts, «⋯» —
  // общий MenuList. Границы — видимая область холста без линеек, ленты «Добавить» и плавающей карточки. box — рамка всего
  // выделенного (у группы описанная). avoid — центр ⟳ (px): панель его не закрывает. Одиночное фото сообщает miniSpot'у,
  // где стоит панель и есть ли место дуге наклона (063).
  let {
    spread,
    box,
    width,
    height,
    ppm,
    avoid,
  }: { spread: SpreadView; box: Rect; width: number; height: number; ppm: number; avoid?: { x: number; y: number } } = $props()

  const photo = $derived(placesel.selected.find((i) => i.photo)?.photo ?? '')
  // Одиночное фото на месте: только ему дуга наклона и ползунок масштаба над панелью.
  const solo = $derived(placesel.selected.length === 1 && !!photo && placesel.texts.length + placesel.selectedStickers.length === 0)
  // Тексты и стикеры группы мини-панель считает вместе («Удалить выделенное (N)»).
  const items = $derived(
    miniItems(
      {
        ...miniContext(spread, placesel.selected, placesel.texts.length + placesel.selectedStickers.length),
        tilt: tiltmode.on,
        missing: store.missing.includes(photo),
      },
      prefs.can,
    ),
  )

  let rowW = $state(0) // ширина ряда-запасного, px (замер; у столбца не меряется: он 34 px, и ряд от него не зависит)
  let rawW = $state(0)
  $effect(() => {
    if (!isCol && rawW > 0) rowW = rawW
  })
  // Якорь на месте панели внутри листа: от его родителя — экранный прямоугольник листа. Сама панель уходит в body
  // (095, owner-ux §1.9): окно холста её обрезало бы, а у нижнего фото она встаёт поверх ленты разворотов.
  let anchor = $state<HTMLElement>()
  // Видимая область холста в осях разворота (px) без линеек, ленты и карточки. DOM не реактивен: перемеряем после
  // раскладки при зуме, сдвиге, смене размера окна, листании ряда и смене панелей.
  let view = $state.raw<MiniView>()
  // Угол листа на экране: панель стоит position: fixed от него.
  let org = $state.raw<Org>()
  $effect(() => {
    void [
      canvas.cur,
      canvas.vp,
      canvas.win,
      row.pos,
      width,
      height,
      panels.mode,
      modes.details,
      modes.rulers,
      placesel.selected.length,
      prefs.level,
      prefs.enabled,
    ]
    const m = measureMini(anchor?.parentElement, canvas.win, width)
    if (!sameView(m.v, view)) view = m.v
    if (!sameOrg(m.o, org)) org = m.o
  })
  const seen = $derived(view ?? { left: 0, top: 0, right: width, bottom: height })
  // В режиме наклона ⟳ спрятан (стоит на месте дуги): панель его не обходит.
  const avoidAt = $derived(solo && tiltmode.on ? undefined : avoid)
  const colH = $derived(colHeightOf(items))
  // Подписи и заглушки выделенного: ряд встаёт под ними, столбец внутри у низа — над ними (037).
  const capBottom = $derived(captionBottom(spread, placesel.selected))
  const caps = $derived(captionBoxes(spread, placesel.selected))
  const place = $derived(placeMini(box, ppm, width, height, rowW, colH, avoidAt, capBottom, seen, caps))
  const isCol = $derived(place.where === 'right' || place.where === 'left' || place.where === 'inright')
  const panelH = $derived(isCol ? colH : MINI_H)
  // Подсказки значков — сбоку от столбца, с противоположной от фото стороны (108-4); у ряда — сверху.
  const tipSide = $derived(place.where === 'right' ? 'right' : isCol ? 'left' : 'top')
  // Панель целиком вне окна холста (и ленты под ним) — не рисуем: в body её больше ничто не обрезает.
  const outside = $derived(panelOutside(place, panelH, isCol, seen))
  // Дуге наклона: повёрнутое место считаем по его осям (box тут — описанный прямоугольник). Где стоит полоса — берём как в режиме
  // наклона (⟳ спрятан), а не как сейчас: иначе при выключенном режиме «есть место» → включили → полоса переехала → «нет» → выход.
  const arc = $derived.by(() => {
    const sel = placesel.selected[0]
    if (!solo || !sel) return NO_ROOM
    return arcRoom(sel.rot ? sel : box, ppm, sel.rot ?? 0, seen, arcWhereOf(box, ppm, width, height, rowW, colH, capBottom, seen, caps))
  })
  // Рамка места для ползунка кадра: у повёрнутого — оси места (sel — до поворота), иначе рамка выделения.
  const zoomBox = $derived.by(() => {
    const sel = placesel.selected[0]
    const b = sel?.rot ? sel : box
    return { x: b.x * ppm, y: b.y * ppm, w: b.w * ppm, h: b.h * ppm, rot: sel?.rot ?? 0 }
  })

  $effect(() => {
    if (solo) miniSpot.set(photo, place.where, zoomLift(place, zoomBox, seen), arc)
    else miniSpot.clear()
  })
  onMount(() => () => miniSpot.clear())

  const stop = (e: Event) => e.stopPropagation()

  const through = new Through()
  $effect(() => {
    void [place.top, place.left, place.where, photo, placesel.selected.length]
    through.placed()
  })

  // Список «⋯»: сбоку от столбца, в сторону подсказок; у ряда-запасного — под кнопкой.
  const more = new MoreList()
  const moreNodes = $derived(items.find((i) => i.id === 'more')?.more ?? [])
  // Панель сдвинулась или «⋯» пропал (другое выделение, уровень) — список закрывается: он стоял бы на старом месте.
  $effect(() => {
    void [place.top, place.left, place.where]
    more.close()
  })
  $effect(() => {
    if (moreNodes.length === 0) more.close()
  })
  function pickMore(n: MenuNode) {
    more.close()
    if (n.id === 'caption') edit.editCaption(photo)
    else if (n.id === 'name') nameEdit.openFor(spread, photo)
    else if (n.id === 'layer') placesel.layer(n.arg as LayerOp)
  }

  function run(id: MiniId, el: HTMLElement) {
    switch (id) {
      case 'flipH':
        return placesel.flip('h')
      case 'flipV':
        return placesel.flip('v')
      case 'turnCw':
        return placesel.turn(1)
      case 'tilt':
        return tiltmode.toggle()
      case 'fit':
        return placesel.setFit(!placesel.allContain)
      case 'fitPhoto':
        return placesel.arrange('fitPhoto')
      case 'same':
        return placesel.arrange('same')
      case 'more':
        tip.hide()
        return more.toggle(el, isCol)
      case 'delete':
        return placesel.deleteSelected()
      case 'swap':
        // Щелчок без переноса — подсказка; перенос уже ушёл в drag.
        if (drag.clickAllowed()) store.notify(swapTip(isMac()))
        return
    }
  }

  // Esc на document в фазе всплытия: раньше хоткея «снять выделение» (он на window), но позже всех, кто держит Esc
  // в захвате на window (меню фото и список «⋯», палитра, ✥, дуга, окна): их stopPropagation сюда не пускает, и одно
  // нажатие — одно действие (059 п.10). Выход из режима наклона, выделение остаётся. Панель живёт, пока выделено
  // фото, поэтому слушает и при мелкой ячейке, где дуги на холсте нет.
  function esc(e: KeyboardEvent) {
    if (isTextField(e.target as Element | null) || !tiltmode.key(e)) return // в поле ввода Esc — его (отмена правки)
    e.stopPropagation()
    e.preventDefault()
  }

  // ⇄ — drag того же вида, что тело фото в «Авто»: все цели броска работают без изменений.
  function swapDown(e: PointerEvent) {
    tip.hide()
    drag.start(e, () => ({ kind: 'photos', ids: [photo], fromSpread: spread.id }))
  }
</script>

<svelte:document onkeydown={esc} onvisibilitychange={() => document.hidden && through.off('hidden')} />
<svelte:window
  onkeydown={(e) => through.event(e)}
  onkeyup={(e) => through.event(e)}
  onpointermove={(e) => through.event(e)}
  onpointerover={(e) => through.event(e)}
  onpointerdowncapture={(e) => through.event(e)}
  onblur={() => through.off('blur')}
/>

<!-- Полоса — ребёнок якоря: портал уносит её в body, и если бы она была соседом, Svelte при скрытии ветки шёл бы
     по nextSibling от якоря до «последнего узла» (он уже в body) и сносил соседей справа в .selframe — другие
     якоря и направляющие (спека 100 п. 5). Ветка из одного span такого обхода не допускает. -->
<span class="anchor" bind:this={anchor}>
  <!-- Клики полосы не должны снимать выделение: клавиатурных аналогов у самой полосы нет, кнопки внутри фокусируются сами.
     При любом переносе — сквозная, как при Alt (095 сц. 15): бросок на миниатюры под ней попадает в них. Ряд едет — гаснет. -->
  <!-- svelte-ignore a11y_click_events_have_key_events -->
  <div
    class="minipanel"
    use:portal
    class:through={through.on || drag.active}
    class:away={row.moving || !org || outside}
    class:col={isCol}
    bind:clientWidth={rawW}
    style:top="{(org?.y ?? 0) + place.top * (org?.k ?? 1)}px"
    style:left="{(org?.x ?? 0) + place.left * (org?.k ?? 1)}px"
    style:height="{panelH}px"
    role="toolbar"
    tabindex="-1"
    aria-label="Действия с выделенным"
    data-guide="mini-panel"
    data-chrome
    data-text-ui
    onpointerdown={stop}
    onclick={stop}
    ondblclick={stop}
  >
    {#each items as it (it.id)}
      {#if it.sep}<span class="sep"></span>{/if}
      <button
        class:on={it.on || (it.id === 'more' && more.open)}
        class:del={it.id === 'delete'}
        class:swap={it.id === 'swap'}
        aria-label={it.label}
        aria-pressed={it.on === undefined ? undefined : it.on}
        aria-haspopup={it.id === 'more' ? 'menu' : undefined}
        aria-expanded={it.id === 'more' ? more.open : undefined}
        {...it.id === 'tilt' ? { 'data-guide': 'tilt-mini' } : {}}
        use:tipped={{ text: it.id === 'more' && more.open ? '' : it.label, side: tipSide }}
        onpointerdown={it.id === 'swap' ? swapDown : undefined}
        onclick={(e) => run(it.id, e.currentTarget)}
      >
        <Icon name={it.icon} size={14} />{#if it.badge}<span class="badge">{it.badge}</span>{/if}
      </button>
    {/each}
  </div>
</span>

{#if more.open && moreNodes.length > 0}
  <MenuList
    items={moreNodes}
    at={more.at}
    side={place.where === 'right' || !isCol ? 'right' : 'left'}
    keep={more.btn}
    onpick={pickMore}
    onclose={() => more.close()}
  />
{/if}

<style>
  .anchor {
    display: none;
  }
  /* z 40: над лентой разворотов, под окнами (80), меню (90), подсказками (95) и призраком (100) */
  .minipanel {
    position: fixed;
    z-index: 40;
    transform: translateX(-50%);
    display: flex;
    align-items: center;
    gap: 2px;
    padding: 0 4px;
    box-sizing: border-box;
    white-space: nowrap;
    background: rgba(30, 30, 32, 0.94);
    border: 1px solid var(--line);
    border-radius: 6px;
    box-shadow: 0 2px 10px rgba(0, 0, 0, 0.4);
    pointer-events: auto;
    cursor: default;
  }
  /* Столбец (108): 34 px шириной, высота — расчётом colHeight (minicol.ts): кнопка 22, линия 7, зазор 2, рамка 10.
     Позиция — левый верхний угол, без центровки. */
  .minipanel.col {
    transform: none;
    flex-direction: column;
    justify-content: flex-start;
    padding: 4px 3px;
    width: 34px;
  }
  .minipanel.through {
    pointer-events: none;
    opacity: 0.35;
  }
  /* Как полоса раскладки над листом (Stage .frame.away) */
  .minipanel.away {
    opacity: 0;
    pointer-events: none;
  }
  button {
    position: relative;
    height: 22px;
    min-width: 26px;
    padding: 0 6px;
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 4px;
    font-size: 12px;
    background: transparent;
    color: #cfcfd4;
    border-radius: 4px;
  }
  .col button {
    width: 26px;
    padding: 0;
  }
  button:hover {
    background: #38383c;
    color: #fff;
  }
  button.on {
    background: #46464b;
    color: #fff;
  }
  button.del:hover {
    color: #ff8a8a;
  }
  button.swap {
    cursor: grab;
    touch-action: none;
  }
  .badge {
    font-variant-numeric: tabular-nums;
  }
  /* В столбце число уголком: кнопка и столбец не шире */
  .col .badge {
    position: absolute;
    top: -3px;
    right: -3px;
    min-width: 12px;
    padding: 0 3px;
    box-sizing: border-box;
    font-size: 9px;
    line-height: 12px;
    text-align: center;
    color: #1a1a1c;
    background: var(--accent);
    border-radius: 6px;
  }
  .sep {
    width: 1px;
    height: 14px;
    margin: 0 2px;
    background: var(--line);
  }
  .col .sep {
    width: 16px;
    height: 1px;
    margin: 3px 0;
  }
</style>
