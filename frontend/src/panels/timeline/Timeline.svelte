<script lang="ts">
  import { untrack } from 'svelte'
  import { toggleKey } from '../../base/core/platform'
  import { store } from '../../base/core/store.svelte'
  import { drag } from '../../base/drag/drag.svelte'
  import { canvas } from '../../editor/canvas/canvas.svelte'
  import { tip } from '../../base/ui/tip.svelte'
  import { H, PAD, mergeCount } from './timeline'
  import { afterNav, isHidden, widthsKey } from './chips'
  import { row } from '../../editor/spread/spreadrow.svelte'
  import { rowScroll } from '../../editor/spread/spreadrow'
  import { CENTER_SEL, StripFollow } from './stripscroll'
  import FixedTip from '../../base/ui/FixedTip.svelte'
  import SpreadMenu from './SpreadMenu.svelte'
  import JoinLine from './JoinLine.svelte'
  import TimelineGroup from './TimelineGroup.svelte'
  import AddColumn from './AddColumn.svelte'
  import CoverTile from '../cover/CoverTile.svelte'
  import { prefs } from '../../base/prefs/prefs.svelte'
  import { spreadsel } from './spreadsel.svelte'
  import { COVER_ID, sheetIn, unitOf } from '../../base/geom/sheet'
  import { isSelectClick } from './spreadsel'
  import { anyVaries } from '../gallery/variants'
  import { clearVignette, makeVignette } from '../vignette/vigact'
  import { vigDialog } from '../vignette/vigsession.svelte'
  import { modes } from '../../base/prefs/view.svelte'
  import { Nearby } from '../../base/ui/nearby.svelte'

  // Лента альбома (спека 003): миниатюры фото, сгруппированные по разворотам.
  // Расчёты размеров — timeline/timeline.ts, подсказка — FixedTip. Поля по краям ленты — полширины окна:
  // и первая, и последняя группа встают по центру, под разворотом.
  const spreads = $derived(store.view.spreads)
  const n = $derived(spreads.length)
  const unit = $derived(unitOf(store.view.settings))
  // Обложка (032) — первый слот ряда, плитка стоит перед разворотами; слотов в ряду столько же, сколько центров в ленте.
  const lead = $derived(store.lead)
  const slots = $derived(n + lead)
  // Плитка есть всегда, когда есть что ей предшествовать: обложка или развороты (пустая «перетащите фото»).
  const coverTile = $derived(slots > 0 && !store.view.settings.sheet)
  const max = $derived(store.view.maxSpreadPhotos || 64)
  // Пропавшие фото — для метки в плитке разворота.
  const missSet = $derived(new Set(store.missing))

  let box: HTMLDivElement
  // 073: миниатюры группы монтируются только рядом с видимой частью ленты; саму группу (цели броска, номер) рисуем у всех.
  const near = new Nearby(() => box ?? null, '0px 1500px')
  let bw = $state(0) // видимая ширина ленты
  // Разворот, у которого сейчас правится название (двойной щелчок по номеру).
  let renaming = $state<string | null>(null)

  // Лента едет вместе с рядом разворотов (спека 012 §7): прокрутка — от дробной позиции row.pos,
  // плавность даёт сам ряд, поэтому без smooth. Центры групп (последний — «+») кэшируются: перемер — при смене ширин групп
  // (widthsKey, а не каждой правки: чтение offsetLeft после правки DOM форсирует раскладку), числа слотов, ширины ленты,
  // названия в правке, обложки и в начале движения.
  let centers: number[] = []
  let centersKey = ''
  const follow = new StripFollow()
  function measure() {
    centers = Array.from(box.querySelectorAll<HTMLElement>(CENTER_SEL), (el) => el.offsetLeft + el.offsetWidth / 2)
  }
  $effect(() => {
    const cv = store.cover?.sheet
    const key = `${widthsKey(spreads, expanded)}|${slots}|${bw}|${renaming}|${cv?.w}x${cv?.h}`
    const moving = row.moving
    const pos = row.pos
    if (!box || slots === 0 || bw === 0) {
      // Пустая лента: прокрутку обнулил браузер, своя запись устарела (первый лист после пустой — 0 → 1).
      follow.reset()
      centersKey = ''
      return
    }
    const same = key === centersKey && centers.length === slots + 1
    if (same && follow.idle(pos, moving)) return // правка не сдвинула группы: ни перемера, ни чтения scrollLeft
    if (!same || follow.starting(moving)) {
      measure()
      centersKey = key
    }
    // false — пользователь прокрутил ленту сам: не отбираем до следующего хода ряда
    if (!follow.next(pos, moving, box.scrollLeft)) return
    box.scrollLeft = rowScroll(centers, pos, bw)
    follow.wrote(box.scrollLeft)
  })

  function wheel(e: WheelEvent) {
    if (Math.abs(e.deltaY) > Math.abs(e.deltaX)) {
      box.scrollLeft += e.deltaY
      e.preventDefault()
    }
  }

  // Разрез и склейка — функция «Разрез и склейка» (спека 011); без неё черта остаётся только целью переноса.
  const cuts = $derived(prefs.can('timeline'))

  // Раскрытая длинная группа (027): одна на ленту. Сворачивается при смене текущего разворота;
  // выбор на холсте фото, спрятанного под «+N», раскрывает группу текущего разворота.
  let expanded = $state<string | null>(null)
  // Переход на обложку (032) currentId не меняет — ключ учитывает onCover.
  const navKey = () => (store.onCover ? COVER_ID : store.currentId)
  let prevCur = navKey()
  $effect(() => {
    const cur = navKey()
    expanded = afterNav(
      untrack(() => expanded),
      prevCur,
      cur,
    )
    prevCur = cur
  })
  // Реагирует только на смену выделения и разворота: правка проекта не должна раскрывать свёрнутое заново.
  $effect(() => {
    const id = store.selectedCell
    const cur = store.currentId
    const onCover = store.onCover
    untrack(() => {
      if (onCover) return
      const sp = spreads.find((s) => s.id === cur)
      if (id && sp && isHidden(sp, id, expanded)) expanded = sp.id
    })
  })

  // Склейка и разрез у сетки закрыты: сначала «Вернуть авто» (спека 007).
  const gridAt = (i: number) => !!(spreads[i]?.grid || spreads[i + 1]?.grid)

  function pickCell(e: MouseEvent, i: number, id: string, sid: string) {
    e.stopPropagation()
    if (!drag.clickAllowed()) return
    // Ctrl/Shift+щелчок по фото — то же, что по группе; простой снимает выделение групп.
    if (isSelectClick(e)) return pickGroup(e, i, sid)
    spreadsel.clear()
    store.go(i)
    store.selectedCell = id
  }

  // Меню правого щелчка по группе: удалить именно её, а по выделенной (две и больше) — все выделенные.
  let menu = $state<{ x: number; y: number; ids: string[] } | null>(null)

  // Щелчок по группе: Ctrl/Cmd — в выделение и обратно, Shift — диапазон; текущий разворот не меняется.
  // Без модификатора выделение снимается и лента переходит к группе.
  function pickGroup(e: MouseEvent, i: number, id: string) {
    if (!drag.clickAllowed()) return
    if (toggleKey(e)) spreadsel.toggle(id)
    else if (e.shiftKey)
      spreadsel.extend(
        spreads.map((s) => s.id),
        store.currentId,
        id,
      )
    else {
      spreadsel.clear()
      store.go(i)
    }
  }

  function openMenu(e: MouseEvent, id: string) {
    e.preventDefault()
    e.stopPropagation()
    if (drag.active) return
    tip.hide()
    if (!(spreadsel.has(id) && spreadsel.size >= 2)) spreadsel.clear()
    const ids = spreadsel.size >= 2 ? spreadsel.ordered(spreads.map((s) => s.id)) : [id]
    menu = { x: e.clientX, y: e.clientY, ids }
  }

  // «Сделать виньеткой» / «Обычный разворот» для группы под курсором (031).
  function vignetteFromMenu(act: 'make' | 'clear' | 'setup') {
    const sp = menu && sheetIn(store.view, menu.ids[0])
    menu = null
    if (!sp) return
    if (act === 'setup') vigDialog.open(sp.id)
    else if (act === 'make') void makeVignette(sp)
    else void clearVignette(sp.id)
  }

  // «Разнообразить раскладки» (041) — для выделенных групп или той, по которой щёлкнули.
  function varyFromMenu() {
    const ids = menu?.ids ?? []
    menu = null
    void store.varyLayouts(ids, modes.varyLevel, modes.autoFamilies)
  }

  function deleteFromMenu() {
    const ids = menu?.ids ?? []
    menu = null
    if (ids.length > 1) store.deleteSpreads(ids)
    else if (ids.length === 1) store.deleteSpread(ids[0])
  }
</script>

<div class="tl" data-guide="timeline">
  <div
    class="timeline"
    class:dragging={drag.active}
    class:empty={slots === 0}
    bind:this={box}
    bind:clientWidth={bw}
    onwheel={wheel}
    onscroll={() => {
      tip.hide()
      menu = null
    }}
    style:--h="{H}px"
    style:--pad="{PAD}px"
  >
    {#if slots > 0}<div class="edge" style:width="{bw / 2}px"></div>{/if}
    {#if coverTile}<CoverTile />{/if}
    {#if n > 0}
      <!-- Узкая зона нового разворота в начало -->
      <div
        class="lead"
        class:over={drag.over === 'new:0'}
        data-drop="new"
        data-at="0"
        title="Перетащите фото сюда — новый {unit.one} в начале"
      ></div>
    {/if}

    {#each spreads as sp, i (sp.id)}
      {@const cnt = mergeCount(spreads, i)}
      {@const gridJoin = gridAt(i)}
      <TimelineGroup
        {sp}
        {i}
        {lead}
        {near}
        {expanded}
        {renaming}
        {cuts}
        {missSet}
        onpick={pickCell}
        onpickgroup={pickGroup}
        onmenu={openMenu}
        onexpand={() => (expanded = sp.id)}
        onfold={() => (expanded = null)}
        onrename={(id) => (renaming = id)}
      />

      {#if i < n - 1}<JoinLine {i} id={sp.id} {cnt} {gridJoin} {cuts} {max} />{/if}
    {/each}

    <AddColumn />

    {#if slots > 0}<div class="edge" style:width="{bw / 2}px"></div>{/if}
  </div>
  <!-- Уголок от разворота к текущей группе: центр ленты = центр разворота -->
  {#if slots > 0 && !canvas.zoomed}<div class="pointer"></div>{/if}
</div>

<FixedTip />
{#if menu}
  <SpreadMenu
    x={menu.x}
    y={menu.y}
    count={menu.ids.length}
    kind={sheetIn(store.view, menu.ids[0])?.kind}
    onVignette={vignetteFromMenu}
    onVary={varyFromMenu}
    varies={anyVaries(store.view.spreads, menu.ids)}
    onDelete={deleteFromMenu}
    onclose={() => (menu = null)}
  />
{/if}

<style>
  .tl {
    position: relative;
    flex: none;
    display: flex;
    flex-direction: column;
  }
  /* «Язычок» от текущей группы к развороту: треугольник вверх цвета её подсветки сидит вплотную над верхним
     краем группы и читается её продолжением. #3f3f41 = rgba(255,255,255,.14) (.group.current) поверх --panel2,
     непрозрачный, чтобы шов не светился. Не прокручивается вместе с группами. */
  .pointer {
    position: absolute;
    top: 1px; /* под верхней чертой ленты; основание — на верхнем крае группы (1 + padding-top 8 = 9) */
    left: 50%;
    margin-left: -9px;
    width: 0;
    height: 0;
    border-left: 9px solid transparent;
    border-right: 9px solid transparent;
    border-bottom: 8px solid #3f3f41;
    pointer-events: none;
    z-index: 2;
  }
  .edge {
    flex: none;
    height: 1px;
  }
  .timeline {
    position: relative; /* offsetLeft групп — от начала ленты */
    flex: none;
    display: flex;
    align-items: flex-start;
    padding: 8px 16px 6px;
    /* Полоса всегда занимает место: с первым разворотом лента не прыгает по высоте. */
    overflow-x: scroll;
    overflow-y: hidden;
    background: var(--panel2);
    border-top: 1px solid var(--line);
    scrollbar-width: thin;
  }
  /* Пустой альбом: одна ячейка «+» по центру, прокручивать нечего — полосу не рисуем, место держим. */
  .timeline.empty {
    justify-content: center;
    scrollbar-color: transparent transparent;
  }
  /* Пустой альбом: под «+» — прозрачная полоса прокрутки; отступы сдвинуты, чтобы «+» с подписью
     стоял по центру по высоте (сумма прежняя — высота ленты не меняется). */
  .timeline.empty {
    padding-top: 12px;
    padding-bottom: 2px;
  }
  .lead {
    all: unset;
    position: relative;
    flex: none;
    width: 14px;
    height: calc(var(--h) + 2 * var(--pad));
  }
  .lead.over::before {
    content: '';
    position: absolute;
    top: 0;
    bottom: 0;
    left: 5px;
    width: 4px;
    border-radius: 2px;
    background: var(--accent);
  }
</style>
