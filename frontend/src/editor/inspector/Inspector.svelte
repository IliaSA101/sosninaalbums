<script lang="ts">
  import type { Component } from 'svelte'
  import { panels } from './panelmode.svelte'
  import { canvas } from '../canvas/canvas.svelte'
  import { drag } from '../../base/drag/drag.svelte'
  import { prefs } from '../../base/prefs/prefs.svelte'
  import { BAR, INSP_W, M, cardTitle, inspectorMaxH, type PanelMode } from './panels'
  import { modes } from '../../base/prefs/view.svelte'
  import { edit } from '../text/edit.svelte'
  import { placesel } from '../places/placesel.svelte'
  import { store } from '../../base/core/store.svelte'
  import { unitOf } from '../../base/geom/sheet'
  import InspectorPlace from './InspectorPlace.svelte'
  import InspectorFrame from './InspectorFrame.svelte'
  import InspectorText from '../text/InspectorText.svelte'
  import InspectorSticker from '../sticker/InspectorSticker.svelte'
  import InspectorSpread from './InspectorSpread.svelte'

  // Карточка свойств выделенного (спека 008, panels.md): справа на холсте или, при открытой
  // панели деталей (docked), первым блоком в ней. Тело по режиму — строка в BODIES; ничего не выделено —
  // «Разворот N» (InspectorSpread). В панели деталей карточка сворачивается (modes.sections.card, 028 Б):
  // заголовок — кнопка, тело не размонтируется. reveal — «показать элемент» от прокручиваемой панели.
  let { docked = false, reveal }: { docked?: boolean; reveal?: (el: HTMLElement) => void } = $props()

  const BODIES: Partial<Record<NonNullable<PanelMode>, Component<{ head?: boolean }>>> = {
    idle: InspectorSpread,
    places: InspectorPlace,
    text: InspectorText,
    sticker: InspectorSticker,
    photo: InspectorFrame as Component<{ head?: boolean }>, // своего заголовка нет: имя файла остаётся в теле
  }
  const Body = $derived(panels.mode ? BODIES[panels.mode] : undefined)
  // При переносе фото карточка остаётся: в ней зона «Фото фона» (цель броска). Остальное тело карточки — не цель,
  // и всё под ней тоже (pickDrop по data-chrome). При переносе разворота (не фото) карточка скрыта.
  const photos = $derived(drag.active && drag.payload?.kind === 'photos')
  // Плавающая карточка — только «Полный»; в панели деталей (docked) она на любом уровне (вопрос 11).
  const shown = $derived(!!Body && (docked || (prefs.can('panels') && (!drag.active || photos))))
  const through = $derived(!docked && photos)

  // Зона фона ниже края невысокой карточки недоступна (колесом при переносе не покрутить) — подвести её в поле зрения.
  let el = $state<HTMLElement>()
  $effect(() => {
    if (through && el) el.querySelector('[data-slot="backdrop"]')?.scrollIntoView({ block: 'nearest' })
  })

  // В панели деталей свёрнутая карточка на время переноса фото раскрывается (без записи в modes):
  // в ней единственная зона броска «Фото фона».
  const open = $derived(!docked || modes.sections.card || photos)
  const title = $derived(
    cardTitle(panels.mode, {
      spreadNo: store.current + 1,
      cover: store.onCover,
      unit: unitOf(store.view.settings).cap,
      caption: !!edit.caption,
      group: placesel.isGroup,
      places: placesel.selected.length,
      texts: placesel.texts.length,
      stickers: placesel.selectedStickers.length,
    }),
  )
  // Пока карточка раскрывается (аккордеон ~0,45 с), зона едет вниз: подводим её в поле зрения каждый кадр.
  // Не scrollIntoView: он сдвинул бы и aside с overflow: hidden.
  $effect(() => {
    if (!docked || !photos || !el || !reveal) return
    const t0 = performance.now()
    let raf = 0
    const step = () => {
      const zone = el?.querySelector<HTMLElement>('[data-slot="backdrop"]')
      if (zone) reveal(zone)
      if (performance.now() - t0 < 700) raf = requestAnimationFrame(step)
    }
    step()
    return () => cancelAnimationFrame(raf)
  })
</script>

{#if shown && Body}
  <!-- Якорь справочника (033) — только у плавающей карточки: «Показать карточку» не должна мигать панелью деталей.
       Спред с литералом, а не выражение в атрибуте: страж справочника запрещает динамический якорь. -->
  <!-- data-text-ui: щелчок по карточке не снимает выделение текста; data-chrome: холст не зумит и не панорамирует -->
  <section
    class="insp"
    data-mini-edge={docked ? null : 'right'}
    {...docked ? {} : { 'data-guide': 'inspector' }}
    class:docked
    bind:this={el}
    style:right={docked ? null : `${M}px`}
    style:top={docked ? null : `${BAR + M}px`}
    style:width={docked ? null : `${INSP_W}px`}
    style:max-height={docked ? null : `${inspectorMaxH(canvas.vp.h)}px`}
    aria-label="Свойства"
    data-chrome
    data-text-ui
    data-card={docked ? '' : null}
  >
    {#if docked}
      <button class="head" aria-expanded={open} aria-controls="card-body" onclick={() => modes.toggleSection('card')}>
        <svg class="chev" class:open viewBox="0 0 12 12" aria-hidden="true"><path d="M4 2.5 7.5 6 4 9.5" /></svg>
        {title}
      </button>
      <!-- Аккордеон, как у Section: тело не размонтируется, поля не теряют набранное -->
      <div class="body" class:open id="card-body" inert={!open}>
        <div class="inner"><div class="pad"><Body head={false} /></div></div>
      </div>
    {:else}
      <Body />
    {/if}
  </section>
{/if}

<style>
  .insp {
    position: absolute;
    z-index: 20;
    box-sizing: border-box;
    padding: 10px 12px 12px;
    background: var(--panel);
    border: 1px solid var(--line);
    border-radius: 8px;
    box-shadow: 0 6px 18px rgba(0, 0, 0, 0.45);
    overflow-x: hidden;
    overflow-y: auto;
    scrollbar-width: thin;
    cursor: default;
  }
  .insp.docked {
    position: static;
    flex: none;
    padding: 0;
    border: none;
    border-bottom: 1px solid var(--line);
    border-radius: 0;
    box-shadow: none;
  }
  /* Шапка и аккордеон — как у Section (panels/details) */
  .head {
    all: unset;
    box-sizing: border-box;
    width: 100%;
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 12px 16px;
    font-weight: 600;
    cursor: pointer;
  }
  .head:hover {
    background: rgba(255, 255, 255, 0.04);
  }
  .head:focus-visible {
    outline: 2px solid var(--accent);
    outline-offset: -2px;
  }
  .chev {
    width: 12px;
    height: 12px;
    flex: none;
    fill: none;
    stroke: var(--muted);
    stroke-width: 1.6;
    stroke-linecap: round;
    stroke-linejoin: round;
    transition: transform var(--move);
  }
  .chev.open {
    transform: rotate(90deg);
  }
  .body {
    display: grid;
    grid-template-rows: 0fr;
    transition: grid-template-rows var(--move);
  }
  .body.open {
    grid-template-rows: 1fr;
  }
  .inner {
    min-height: 0;
    overflow: hidden;
  }
  .pad {
    padding: 0 16px 14px;
  }
</style>
