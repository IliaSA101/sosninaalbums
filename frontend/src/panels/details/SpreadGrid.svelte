<script lang="ts">
  import { untrack } from 'svelte'
  import Spread from '../../editor/spread/Spread.svelte'
  import LowDpiBadge from '../../base/ui/LowDpiBadge.svelte'
  import VigMark from '../vignette/VigMark.svelte'
  import { store } from '../../base/core/store.svelte'
  import { drag } from '../../base/drag/drag.svelte'
  import { Reorder, gridSource } from '../../base/drag/reorder.svelte'
  import { newSheetSize, sheetSize, unitOf } from '../../base/geom/sheet'
  import { modes } from '../../base/prefs/view.svelte'
  import { labelTitle, spreadLabel } from '../timeline/spreadname'
  import { SpreadMenuCtl } from '../spreadmenu/menuact.svelte'
  import SpreadMenuHost from '../spreadmenu/SpreadMenuHost.svelte'
  import SpreadName from '../timeline/SpreadName.svelte'
  import { naming } from '../timeline/naming.svelte'
  import { spreadsel } from '../timeline/spreadsel.svelte'
  import { Nearby } from '../../base/ui/nearby.svelte'

  // Ширина панели 400, поля раздела 16, между плитками 8: три колонки.
  const COLS = 3
  const GAP = 8
  const TILE = Math.floor((400 - 2 * 16 - (COLS - 1) * GAP) / COLS)
  // Внутренний отступ плитки: content-visibility обрезает всё за её краем, рамка должна влезть.
  const PAD = 4
  const MINI = TILE - 2 * PAD
  const s = $derived(store.view.settings)
  // Все плитки одного размера: берём размер листа у первого (Go: sv.w × sv.h); альбом пуст — по настройкам.
  const sheet = $derived(store.view.spreads[0] ? sheetSize(store.view.spreads[0]) : newSheetSize(s))
  const unit = $derived(unitOf(s))
  const tileH = $derived((MINI * sheet.h) / sheet.w)

  let box = $state<HTMLDivElement>()
  // 073: разворот в плитке монтируется только рядом с видимой частью; плитки обрезает скроллер панели (.scroll),
  // а не окно, поэтому он и есть корень — иначе запас rootMargin не работает. untrack: не подписываться на box.
  const near = new Nearby(() => untrack(() => box?.closest('.scroll') ?? null), '400px 0px')

  // Порядок разворотов — перетаскиванием плитки (096), тот же жест, что в ленте; щелчок без сдвига — переход.
  const reorder = new Reorder()
  const scroller = () => box?.closest<HTMLElement>('.scroll') ?? null
  function press(e: PointerEvent, i: number, id: string) {
    if (!box) return
    const grid = box
    drag.start(
      e,
      () => ({ kind: 'spread', id, index: i }),
      undefined,
      (sx, sy) => reorder.session(gridSource(grid, scroller), i, sx, sy),
    )
  }

  // Меню правого щелчка — то же, что у групп ленты (103 §1); работает на одну плитку, выделение групп ленты снимается:
  // в сетке его не видно, и удалить невидимое было бы ловушкой (103-2).
  const ctl = new SpreadMenuCtl('grid')
  function openMenu(e: MouseEvent, id: string) {
    e.preventDefault()
    e.stopPropagation()
    if (drag.active) return
    spreadsel.clear()
    ctl.open(e, [id], -1)
  }
  // Меню — position: fixed и за прокруткой панели не едет: прокрутка его закрывает.
  $effect(() => {
    const sc = ctl.menu ? scroller() : null
    if (!sc) return
    const close = () => ctl.close()
    sc.addEventListener('scroll', close, { passive: true })
    return () => sc.removeEventListener('scroll', close)
  })
  // Пропал разворот с открытым полем (удалён, отменён) — поле не должно ожить, если он вернётся.
  $effect(() => {
    const id = naming.id
    if (naming.where === 'grid' && id && !store.view.spreads.some((s) => s.id === id)) naming.stop()
  })

  // Текущая плитка остаётся видимой в панели. Прокручиваем только вертикально
  // и только свой контейнер: scrollIntoView сдвинул бы и панель с overflow: hidden.
  $effect(() => {
    const i = store.current
    if (!modes.details || !modes.sections.grid) return
    const el = box?.querySelector<HTMLElement>(`[data-tile="${i}"]`)
    const sc = box?.closest<HTMLElement>('.scroll')
    if (!el || !sc) return
    const a = sc.getBoundingClientRect()
    const b = el.getBoundingClientRect()
    if (b.top < a.top) sc.scrollBy({ top: b.top - a.top - 8, behavior: 'smooth' })
    else if (b.bottom > a.bottom) sc.scrollBy({ top: b.bottom - a.bottom + 8, behavior: 'smooth' })
  })
</script>

{#if store.view.spreads.length}
  <div
    class="grid"
    class:reordering={reorder.busy}
    class:landing={reorder.landing}
    data-guide="spread-grid"
    bind:this={box}
    style:--tile="{TILE}px"
    style:--gap="{GAP}px"
    style:--pad="{PAD}px"
    style:--tile-h="{tileH}px"
  >
    {#each store.view.spreads as sp, i (sp.id)}
      <div
        class="tile"
        class:current={i === store.current}
        class:src={reorder.lifted(i)}
        style:transform={reorder.css(i)}
        data-tile={i}
        role="button"
        tabindex="-1"
        title={labelTitle(i, sp.name) ?? `${unit.cap} ${i + 1}`}
        onpointerdown={(e) => press(e, i, sp.id)}
        onclick={() => drag.clickAllowed() && store.go(i)}
        oncontextmenu={(e) => openMenu(e, sp.id)}
        onkeydown={() => {}}
      >
        <div
          class="frame"
          style:height="{(MINI * sp.h) / sp.w}px"
          {@attach (n) =>
            near.watch(
              n,
              untrack(() => sp.id),
            )}
        >
          {#if modes.details && near.has(sp.id)}<Spread spread={sp} settings={s} width={MINI} mini />{/if}
        </div>
        <LowDpiBadge count={sp.lowDPI} min={store.view.minDPI} />
        <VigMark kind={sp.kind} />
        {#if naming.where === 'grid' && naming.id === sp.id}
          <!-- Название правится на месте подписи плитки (103-3) -->
          <div class="num"><SpreadName id={sp.id} name={sp.name} onclose={() => naming.stop(sp.id)} /></div>
        {:else}
          <div class="num">{spreadLabel(i, sp.name)}</div>
        {/if}
      </div>
    {/each}
  </div>
{:else}
  <div class="empty">{unit.none}</div>
{/if}

<SpreadMenuHost {ctl} />

<style>
  .grid {
    position: relative; /* offsetLeft/offsetTop плиток — от сетки (замер перестановки) */
    display: grid;
    grid-template-columns: repeat(3, var(--tile));
    gap: 4px var(--gap);
  }
  .tile {
    position: relative;
    padding: var(--pad);
    cursor: pointer;
    /* Большие альбомы: плитки вне вида не рисуются */
    content-visibility: auto;
    contain-intrinsic-size: auto var(--tile) auto calc(var(--tile-h) + 2 * var(--pad) + 18px);
    border-radius: 4px;
    transition: transform 0.15s ease;
  }
  /* Перестановка (096): как в ленте — все плитки рисуются, бросок без переходов, поднятая поверх соседей. */
  .reordering .tile {
    content-visibility: visible;
  }
  .landing .tile {
    transition: none;
  }
  .tile.src {
    z-index: 5;
    transition: none;
    background: #3f3f41;
    box-shadow: 0 6px 18px rgba(0, 0, 0, 0.55);
  }
  .frame {
    pointer-events: none;
    outline: 2px solid transparent;
    outline-offset: 1px;
    transition: outline-color 0.15s;
  }
  .tile:hover .frame {
    outline-color: #5a5a60;
  }
  .tile.current .frame {
    outline-color: var(--accent);
  }
  .num {
    text-align: center;
    font-size: 11px;
    color: var(--muted);
    margin-top: 3px;
    /* Длинное название — многоточие, плитку не растягивает */
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .tile.current .num {
    color: var(--accent);
    font-weight: 600;
  }
  .empty {
    color: var(--muted);
    font-size: 13px;
  }
</style>
