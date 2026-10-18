<script lang="ts">
  import type { Rect, SpreadView } from '../../base/core/types'
  import { store } from '../../base/core/store.svelte'
  import { placesel } from './placesel.svelte'
  import { edit } from '../text/edit.svelte'
  import { nameEdit } from '../captions/nameedit.svelte'
  import { captionBottom, captionBoxes } from '../captions/nametarget'
  import { prefs } from '../../base/prefs/prefs.svelte'
  import { modes } from '../../base/prefs/view.svelte'
  import { drag } from '../../base/drag/drag.svelte'
  import { tip, tipped } from '../../base/ui/tip.svelte'
  import {
    arcRoom,
    chromeInset,
    miniContext,
    miniItems,
    miniPlace,
    passThrough,
    visibleArea,
    sameView,
    zoomLift,
    arcWhere,
    MINI_H,
    NO_ROOM,
    type MiniId,
    type MiniView,
  } from './minipanel'
  import { miniSpot } from './minispot.svelte'
  import { canvas } from '../canvas/canvas.svelte'
  import { panels } from '../inspector/panelmode.svelte'
  import { row } from '../spread/spreadrow.svelte'
  import { tiltmode } from '../tilt/tiltmode.svelte'
  import { onMount } from 'svelte'
  import { isTextField } from '../../shell/menu/keys'
  import Icon from '../../base/ui/Icon.svelte'
  import LayerMenu from './LayerMenu.svelte'

  // Мини-панель выделенного (спека 009, gestures.md «Мини-панель»): полоса значков с подсказками,
  // вместо текстовой полоски 0.3.0. Состав — places/minipanel.ts. Границы — видимая область холста (серое поле тоже годится): под рамкой
  // (и под её подписью, 037), иначе над ней выше ⟳, иначе внутри у нижнего края (владелец 2026-10-17), иначе внутри у верха.
  // Границы — без линеек, ленты «Добавить» и плавающей карточки. box — рамка всего выделенного (у группы описанная).
  // avoid — центр ⟳ (px): полоса его не закрывает. Одиночное фото сообщает miniSpot'у, где стоит панель и есть ли место
  // дуге наклона (063).
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
  let w = $state(0) // ширина полосы, px (замер)
  let el = $state<HTMLElement>()
  // Видимая область холста в осях разворота (px) без линеек, ленты и карточки. DOM не реактивен: перемеряем после
  // раскладки при зуме, сдвиге, смене размера окна, листании ряда и смене панелей.
  let view = $state.raw<MiniView>()
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
    const sheet = el?.parentElement?.getBoundingClientRect()
    const win = canvas.win?.getBoundingClientRect()
    let v: MiniView | undefined
    if (sheet && win) {
      const parts = [...(canvas.win?.parentElement?.querySelectorAll<HTMLElement>('[data-mini-edge]') ?? [])].map((n) => ({
        edge: n.dataset.miniEdge as 'top' | 'left' | 'right',
        box: n.getBoundingClientRect(),
      }))
      v = visibleArea(sheet, win, width, chromeInset(win, parts))
    }
    if (!sameView(v, view)) view = v
  })
  const seen = $derived(view ?? { left: 0, top: 0, right: width, bottom: height })
  // В режиме наклона ⟳ спрятан (стоит на месте дуги): полоса его не обходит.
  const avoidAt = $derived(solo && tiltmode.on ? undefined : avoid)
  const placeFor = (av?: { x: number; y: number }) =>
    miniPlace(box, ppm, width, height, w, av, captionBottom(spread, placesel.selected), seen, captionBoxes(spread, placesel.selected))
  const place = $derived(placeFor(avoidAt))
  // Дуге наклона: повёрнутое место считаем по его осям (box тут — описанный прямоугольник). Где стоит полоса — берём как в режиме
  // наклона (⟳ спрятан), а не как сейчас: иначе при выключенном режиме «есть место» → включили → полоса переехала → «нет» → выход.
  const arc = $derived.by(() => {
    const sel = placesel.selected[0]
    if (!solo || !sel) return NO_ROOM
    return arcRoom(
      sel.rot ? sel : box,
      ppm,
      sel.rot ?? 0,
      seen,
      arcWhere(box, ppm, width, height, w, captionBottom(spread, placesel.selected), seen, captionBoxes(spread, placesel.selected)),
    )
  })
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

  $effect(() => {
    if (solo) {
      miniSpot.photo = photo
      miniSpot.where = place.where
      miniSpot.lift = zoomLift(place, (box.y + box.h) * ppm, placesel.selected[0]?.rot ?? 0)
      const a = miniSpot.arc
      if (arc.mode !== a.mode || arc.vx !== a.vx || arc.vy !== a.vy || arc.vw !== a.vw) miniSpot.arc = arc // тот же объект — читатели не гоняются
    } else clear()
  })
  const clear = () => {
    miniSpot.photo = ''
    miniSpot.where = 'below'
    miniSpot.lift = 0
    miniSpot.arc = NO_ROOM
  }
  onMount(() => clear)

  const stop = (e: Event) => e.stopPropagation()

  // Модификатор зажат — панель полупрозрачна и не ловит мышь. pointermove видит и Shift, зажатый вне окна.
  let through = $state(false)
  const mods = (e: KeyboardEvent | PointerEvent) => (through = passThrough(e))

  function run(id: MiniId) {
    switch (id) {
      case 'flipH':
        return placesel.flip('h')
      case 'flipV':
        return placesel.flip('v')
      case 'turn':
        return placesel.turn(1)
      case 'tilt':
        return tiltmode.toggle()
      case 'fit':
        return placesel.setFit(!placesel.allContain)
      case 'fitPhoto':
        return placesel.arrange('fitPhoto')
      case 'same':
        return placesel.arrange('same')
      case 'caption':
        return edit.editCaption(photo)
      case 'name':
        return nameEdit.openFor(spread, photo)
      case 'delete':
        return placesel.deleteSelected()
      case 'swap':
        // Щелчок без переноса — подсказка; перенос уже ушёл в drag.
        if (drag.clickAllowed()) store.notify('Перетащите на другое место — фото поменяются местами')
        return
    }
  }

  // Esc на document в фазе всплытия: раньше хоткея «снять выделение» (он на window), но позже всех, кто держит Esc
  // в захвате на window (меню фото, «Слой ▾», палитра, ✥, дуга, окна): их stopPropagation сюда не пускает, и одно
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

<svelte:document onkeydown={esc} />
<svelte:window onkeydown={mods} onkeyup={mods} onpointermove={mods} onblur={() => (through = false)} />

<!-- Клики полосы не должны снимать выделение: клавиатурных аналогов у самой полосы нет, кнопки внутри фокусируются сами -->
<!-- svelte-ignore a11y_click_events_have_key_events -->
<div
  class="minipanel"
  bind:this={el}
  class:through
  bind:clientWidth={w}
  style:top="{place.top}px"
  style:left="{place.left}px"
  style:height="{MINI_H}px"
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
    {#if it.id === 'layer'}
      <LayerMenu onpick={(op) => placesel.layer(op)} label={it.label} />
    {:else}
      <button
        class:on={it.on}
        class:del={it.id === 'delete'}
        class:swap={it.id === 'swap'}
        aria-label={it.label}
        aria-pressed={it.on === undefined ? undefined : it.on}
        {...it.id === 'tilt' ? { 'data-guide': 'tilt-mini' } : {}}
        use:tipped={{ text: it.label }}
        onpointerdown={it.id === 'swap' ? swapDown : undefined}
        onclick={() => run(it.id)}
      >
        <Icon name={it.icon} size={14} />{#if it.badge}<span class="badge">{it.badge}</span>{/if}
      </button>
    {/if}
  {/each}
</div>

<style>
  .minipanel {
    position: absolute;
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
  .minipanel.through {
    pointer-events: none;
    opacity: 0.35;
  }
  button {
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
  .sep {
    width: 1px;
    height: 14px;
    margin: 0 2px;
    background: var(--line);
  }
</style>
