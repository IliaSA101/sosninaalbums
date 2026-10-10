<script lang="ts">
  import type { Component } from 'svelte'
  import { panels } from './panelmode.svelte'
  import { canvas } from '../canvas/canvas.svelte'
  import { drag } from '../../base/drag/drag.svelte'
  import { prefs } from '../../base/prefs/prefs.svelte'
  import { BAR, INSP_W, M, inspectorMaxH, type PanelMode } from './panels'
  import InspectorPlace from './InspectorPlace.svelte'
  import InspectorFrame from './InspectorFrame.svelte'
  import InspectorText from '../text/InspectorText.svelte'
  import InspectorSpread from './InspectorSpread.svelte'

  // Карточка свойств выделенного (спека 008, panels.md): справа на холсте или, при открытой
  // панели деталей (docked), первым блоком в ней. Тело по режиму — строка в BODIES; ничего не выделено —
  // «Разворот N» (InspectorSpread).
  let { docked = false }: { docked?: boolean } = $props()

  const BODIES: Partial<Record<NonNullable<PanelMode>, Component>> = {
    idle: InspectorSpread,
    places: InspectorPlace,
    text: InspectorText,
    photo: InspectorFrame,
  }
  const Body = $derived(panels.mode ? BODIES[panels.mode] : undefined)
  // При переносе фото карточка остаётся: в ней зона «Фото фона» (цель броска). Но под карточкой может
  // оказаться «+ новый разворот», поэтому в переносе она пропускает указатель сквозь себя, кроме самой зоны
  // (стиль .through). При переносе разворота (не фото) карточка скрыта.
  const photos = $derived(drag.active && drag.payload?.kind === 'photos')
  // Плавающая карточка — только «Полный»; в панели деталей (docked) она на любом уровне (вопрос 11).
  const shown = $derived(!!Body && (docked || (prefs.can('panels') && (!drag.active || photos))))
  const through = $derived(!docked && photos)

  // Зона фона ниже края невысокой карточки недоступна (колесом при переносе не покрутить) — подвести её в поле зрения.
  let el = $state<HTMLElement>()
  $effect(() => {
    if (through && el) el.querySelector('[data-slot="backdrop"]')?.scrollIntoView({ block: 'nearest' })
  })
</script>

{#if shown && Body}
  <!-- data-text-ui: щелчок по карточке не снимает выделение текста; data-chrome: холст не зумит и не панорамирует -->
  <section
    class="insp"
    class:docked
    class:through
    bind:this={el}
    style:right={docked ? null : `${M}px`}
    style:top={docked ? null : `${BAR + M}px`}
    style:width={docked ? null : `${INSP_W}px`}
    style:max-height={docked ? null : `${inspectorMaxH(canvas.vp.h)}px`}
    aria-label="Свойства"
    data-chrome
    data-text-ui
  >
    <Body />
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
  .insp.through {
    pointer-events: none;
  }
  .insp.through :global([data-drop='backdrop']) {
    pointer-events: auto;
  }
  .insp.docked {
    position: static;
    flex: none;
    padding: 14px 16px;
    border: none;
    border-bottom: 1px solid var(--line);
    border-radius: 0;
    box-shadow: none;
  }
</style>
