<script lang="ts">
  import { untrack } from 'svelte'
  import { store } from '../../base/core/store.svelte'
  import { drag } from '../../base/drag/drag.svelte'
  import { row } from '../../editor/spread/spreadrow.svelte'
  import { H, PAD } from './timeline'
  import { shownW } from './chips'
  import { labelTitle, spreadLabel } from './spreadname'
  import { spreadsel } from './spreadsel.svelte'
  import { isSheet, unitOf } from '../../base/geom/sheet'
  import { missingIn } from '../library/missing'
  import type { Nearby } from '../../base/ui/nearby.svelte'
  import type { SpreadView } from '../../base/core/types'
  import SpreadName from './SpreadName.svelte'
  import SpreadChips from './SpreadChips.svelte'
  import LowDpiBadge from '../../base/ui/LowDpiBadge.svelte'
  import MissingMark from '../../base/ui/MissingMark.svelte'
  import VigMark from '../vignette/VigMark.svelte'

  // Группа ленты = один разворот: плитки, значки, номер или поле названия (вынесено из Timeline, 075-11).
  // Состояние ленты (раскрытая группа, правка названия, меню) живёт в Timeline и приходит пропсами.
  let {
    sp,
    i,
    lead,
    near,
    expanded,
    renaming,
    cuts,
    missSet,
    shift,
    lifted,
    onpress,
    onpick,
    onpickgroup,
    onmenu,
    onexpand,
    onfold,
    onnamed,
  }: {
    sp: SpreadView
    i: number
    lead: number
    near: Nearby
    expanded: string | null
    renaming: string | null
    cuts: boolean
    missSet: Set<string>
    shift: string | undefined // смещение на время перестановки (096)
    lifted: boolean // эту группу тянут: едет за курсором поверх соседей
    onpress: (e: PointerEvent, i: number, id: string) => void
    onpick: (e: MouseEvent, i: number, photo: string, spread: string) => void
    onpickgroup: (e: MouseEvent, i: number, id: string) => void
    onmenu: (e: MouseEvent, id: string) => void
    onexpand: () => void
    onfold: () => void
    onnamed: () => void // поле названия закрыто
  } = $props()

  const unit = $derived(unitOf(store.view.settings))
</script>

<div
  class="group"
  class:dragging={drag.active && drag.payload?.kind === 'photos'}
  class:current={i + lead === row.near}
  class:over={drag.over.startsWith(`mini:${sp.id}:`) && drag.payload?.kind === 'photos'}
  class:src={lifted}
  class:renaming={renaming === sp.id}
  class:picked={spreadsel.has(sp.id)}
  style:transform={shift}
  style:contain-intrinsic-size="auto {shownW(sp, expanded)}px auto {H + 2 * PAD + 17}px"
  data-drop="mini"
  {@attach (n) =>
    near.watch(
      n,
      untrack(() => sp.id),
    )}
  data-spread={sp.id}
  data-index={i}
  role="button"
  tabindex="-1"
  aria-label="{unit.cap} {i + 1}"
  onpointerdown={(e) => onpress(e, i, sp.id)}
  onclick={(e) => onpickgroup(e, i, sp.id)}
  oncontextmenu={(e) => onmenu(e, sp.id)}
  onkeydown={() => {}}
>
  {#if near.has(sp.id)}
    <SpreadChips {sp} {i} {expanded} {cuts} {onpick} {onexpand} {onfold} />
  {:else}
    <div style:width="{shownW(sp, expanded) - 2 * PAD}px" style:height="{H}px"></div>
  {/if}
  <LowDpiBadge count={sp.lowDPI} min={store.view.minDPI} />
  <MissingMark count={missingIn(sp, missSet)} />
  <VigMark kind={sp.kind} />
  {#if renaming === sp.id}
    <SpreadName id={sp.id} name={sp.name} onclose={onnamed} />
  {:else}
    <!-- Якорь справочника — только у номера текущего разворота: иначе подсветится самый широкий из всех.
         Двойной щелчок ловит лента целиком (naming.capture, 094 §3.1): второй щелчок мог попасть в другую группу. -->
    <div
      class="num"
      {...i + lead === row.near ? { 'data-guide': 'spread-num' } : {}}
      title={labelTitle(i, sp.name, true, isSheet(store.view.settings))}
      role="presentation"
    >
      {spreadLabel(i, sp.name)}
    </div>
  {/if}
</div>

<style>
  .group {
    position: relative;
    flex: none;
    padding: var(--pad);
    border-radius: 4px;
    cursor: pointer;
    transition:
      background 0.15s,
      transform 0.15s ease;
    /* 100+ разворотов: группы вне вида не рисуются. Обрезает всё за краем группы,
       поэтому индикаторы рисуются внутри её отступа. */
    content-visibility: auto;
  }
  .group.current {
    background: rgba(255, 255, 255, 0.14);
  }
  .group.dragging.current {
    background: rgba(245, 217, 10, 0.35);
  }
  .group.over,
  .group.dragging.over {
    background: rgba(245, 217, 10, 0.6);
  }
  /* Выделенные Ctrl/Shift+щелчком группы (спека 019 F) */
  .group.picked {
    outline: 2px solid var(--accent);
    outline-offset: -2px;
  }
  /* Перестановка (096): все группы рисуются — сдвинутая в вид не мелькает пустой; миниатюры по-прежнему только
     рядом с видом (Nearby). Бросок — без переходов: группы уже стоят там, где их поставит новый порядок. */
  :global(.timeline.reordering) .group {
    content-visibility: visible;
  }
  :global(.timeline.landing) .group {
    transition: none;
  }
  /* Поднятая группа едет за курсором без перехода, поверх соседей. */
  .group.src {
    z-index: 5;
    transition: background 0.15s;
    background: #3f3f41;
    box-shadow: 0 6px 18px rgba(0, 0, 0, 0.55);
  }
  .num {
    text-align: center;
    font-size: 11px;
    line-height: 14px;
    color: var(--muted);
    margin-top: 3px;
    /* Длинное название обрезается по ширине группы и саму группу не растягивает */
    width: 0;
    min-width: 100%;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    user-select: none;
  }
  /* Поле названия в узкой группе: группа на время правки раздвигается */
  .group.renaming {
    min-width: 88px;
  }
  .group.current .num {
    color: var(--text);
    font-weight: 600;
  }
</style>
