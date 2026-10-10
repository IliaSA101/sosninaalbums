<script lang="ts">
  import { store } from '../../base/core/store.svelte'
  import { drag } from '../../base/drag/drag.svelte'
  import { canvas } from '../../editor/canvas/canvas.svelte'
  import { tip, tipped } from '../../base/ui/tip.svelte'
  import { thumbUrl } from '../../base/core/media'
  import { flipCss, photoCss, thumbBox } from '../../base/geom/frame'
  import { H, PAD, cellW, chipW, emptyPlaces, groupW, isBlank, mergeCount } from './timeline'
  import { row } from '../../editor/spread/spreadrow.svelte'
  import { rowScroll } from '../../editor/spread/spreadrow'
  import { shouldScroll } from './stripscroll'
  import { labelTitle, spreadLabel } from './spreadname'
  import FixedTip from '../../base/ui/FixedTip.svelte'
  import SpreadName from './SpreadName.svelte'
  import SpreadMenu from './SpreadMenu.svelte'
  import JoinLine from './JoinLine.svelte'
  import AddColumn from './AddColumn.svelte'
  import { prefs } from '../../base/prefs/prefs.svelte'
  import { spreadsel } from './spreadsel.svelte'
  import { isSelectClick } from './spreadsel'

  // Лента альбома (спека 003): миниатюры фото, сгруппированные по разворотам.
  // Расчёты размеров — timeline/timeline.ts, подсказка — FixedTip. Поля по краям ленты — полширины окна:
  // и первая, и последняя группа встают по центру, под разворотом.
  const spreads = $derived(store.view.spreads)
  const n = $derived(spreads.length)
  const max = $derived(store.view.maxSpreadPhotos || 7)

  let box: HTMLDivElement
  let bw = $state(0) // видимая ширина ленты
  // Разворот, у которого сейчас правится название (двойной щелчок по номеру).
  let renaming = $state<string | null>(null)

  // Лента едет вместе с рядом разворотов (спека 012 §7): прокрутка — от дробной позиции row.pos,
  // плавность даёт сам ряд, поэтому без smooth. Центры групп (последний — «+») кэшируются:
  // пересчёт при правке, смене числа групп, ширины ленты, названия в правке и в начале движения.
  let centers: number[] = []
  let centersKey = ''
  let wasMoving = false
  let lastPos = NaN
  let written: number | null = null // scrollLeft после нашей последней записи
  function measure() {
    centers = Array.from(box.querySelectorAll<HTMLElement>('[data-drop="mini"], .addcol'), (el) => el.offsetLeft + el.offsetWidth / 2)
  }
  $effect(() => {
    const key = `${store.view.rev}|${n}|${bw}|${renaming}`
    const moving = row.moving
    const pos = row.pos
    if (!box || n === 0 || bw === 0) return
    if (key !== centersKey || centers.length !== n + 1 || (moving && !wasMoving)) {
      measure()
      centersKey = key
    }
    const go = shouldScroll({ posChanged: pos !== lastPos, movingStarted: moving && !wasMoving, scrollLeft: box.scrollLeft, written })
    wasMoving = moving
    lastPos = pos
    if (!go) return // пользователь прокрутил ленту сам — не отбираем до следующего хода ряда
    box.scrollLeft = rowScroll(centers, pos, bw)
    written = box.scrollLeft
  })

  function wheel(e: WheelEvent) {
    if (Math.abs(e.deltaY) > Math.abs(e.deltaX)) {
      box.scrollLeft += e.deltaY
      e.preventDefault()
    }
  }

  // Разрез и склейка — функция «Разрез и склейка» (спека 011); без неё черта остаётся только целью переноса.
  const cuts = $derived(prefs.can('timeline'))

  // Склейка и разрез у сетки закрыты: сначала «Вернуть авто» (спека 007).
  const gridAt = (i: number) => !!(spreads[i]?.grid || spreads[i + 1]?.grid)

  function split(e: MouseEvent, id: string, at: number) {
    e.stopPropagation()
    if (!drag.clickAllowed()) return
    tip.hide()
    store.split(id, at)
  }

  function pickCell(e: MouseEvent, i: number, id: string, sid: string) {
    e.stopPropagation()
    if (!drag.clickAllowed()) return
    // Ctrl/Shift+щелчок по фото — то же, что по группе; простой снимает выделение групп.
    if (isSelectClick(e)) return pickGroup(e, i, sid)
    spreadsel.clear()
    store.go(i)
    store.selectedCell = id
  }

  const stop = (e: PointerEvent) => e.stopPropagation()

  // Меню правого щелчка по группе: удалить именно её, а по выделенной (две и больше) — все выделенные.
  let menu = $state<{ x: number; y: number; ids: string[] } | null>(null)

  // Щелчок по группе: Ctrl/Cmd — в выделение и обратно, Shift — диапазон; текущий разворот не меняется.
  // Без модификатора выделение снимается и лента переходит к группе.
  function pickGroup(e: MouseEvent, i: number, id: string) {
    if (!drag.clickAllowed()) return
    if (e.ctrlKey || e.metaKey) spreadsel.toggle(id)
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

  function deleteFromMenu() {
    const ids = menu?.ids ?? []
    menu = null
    if (ids.length > 1) store.deleteSpreads(ids)
    else if (ids.length === 1) store.deleteSpread(ids[0])
  }
</script>

<div class="tl">
  <div
    class="timeline"
    class:dragging={drag.active}
    class:empty={n === 0}
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
    {#if n > 0}<div class="edge" style:width="{bw / 2}px"></div>{/if}
    {#if n > 0}
      <!-- Узкая зона нового разворота в начало -->
      <div
        class="lead"
        class:over={drag.over === 'new:0'}
        data-drop="new"
        data-at="0"
        title="Перетащите фото сюда — новый разворот в начале"
      ></div>
    {/if}

    {#each spreads as sp, i (sp.id)}
      {@const cnt = mergeCount(spreads, i)}
      {@const gridJoin = gridAt(i)}
      <div
        class="group"
        class:current={i === row.near}
        class:over={drag.over.startsWith(`mini:${sp.id}:`) && drag.payload?.kind === 'photos'}
        class:ins-before={drag.over === `mini:${sp.id}:before` && drag.payload?.kind === 'spread'}
        class:ins-after={drag.over === `mini:${sp.id}:after` && drag.payload?.kind === 'spread'}
        class:lifted={drag.active && drag.payload?.kind === 'spread' && drag.payload.id === sp.id}
        class:renaming={renaming === sp.id}
        class:picked={spreadsel.has(sp.id)}
        style:contain-intrinsic-size="auto {groupW(sp)}px auto {H + 2 * PAD + 17}px"
        data-drop="mini"
        data-spread={sp.id}
        data-index={i}
        role="button"
        tabindex="-1"
        aria-label="Разворот {i + 1}"
        onpointerdown={(e) => drag.start(e, () => ({ kind: 'spread', id: sp.id, index: i }))}
        onclick={(e) => pickGroup(e, i, sp.id)}
        oncontextmenu={(e) => openMenu(e, sp.id)}
        onkeydown={() => {}}
      >
        <div class="chips">
          {#each sp.photos as id, j (id)}
            <div
              class="chip"
              class:selected={store.selectedCell === id}
              class:ins-before={drag.over === `chip:${id}:before`}
              class:ins-after={drag.over === `chip:${id}:after`}
              class:lifted={drag.active &&
                drag.payload?.kind === 'photos' &&
                drag.payload.fromSpread === sp.id &&
                drag.payload.ids[0] === id}
              style:width="{cellW(sp.rects[j])}px"
              data-drop="chip"
              data-spread={sp.id}
              data-index={j}
              data-photo={id}
              role="button"
              tabindex="-1"
              onpointerdown={(e) => {
                e.stopPropagation()
                drag.start(e, () => ({ kind: 'photos', ids: [id], fromSpread: sp.id }))
              }}
              onclick={(e) => pickCell(e, i, id, sp.id)}
              onkeydown={() => {}}
            >
              {#if sp.frames?.[j]?.r}
                <!-- Повёрнутое фото: целиком в своей рамке, лишнее обрезано (как на развороте) -->
                <span class="rot"
                  ><img
                    src={thumbUrl(id)}
                    alt=""
                    loading="lazy"
                    draggable="false"
                    style={thumbBox(sp.rects[j], sp.frames[j].img.w / sp.frames[j].img.h, sp.frames[j].r)}
                    style:transform={photoCss(sp.frames[j])}
                  /></span
                >
              {:else}
                <img src={thumbUrl(id)} alt="" loading="lazy" draggable="false" style:transform={flipCss(sp.frames?.[j])} />
              {/if}
              {#if j > 0 && !sp.grid && cuts}
                <!-- Разрез перед этим фото -->
                <button
                  class="cut"
                  aria-label="Разрезать разворот {i + 1} перед фото {j + 1}"
                  onpointerdown={stop}
                  use:tipped={{ text: 'Разрезать' }}
                  onclick={(e) => split(e, sp.id, j)}
                ></button>
              {/if}
            </div>
          {/each}
          {#if isBlank(sp)}
            <!-- Пустой разворот: квадратная заглушка, без пунктира (пунктир — признак «+») -->
            <div class="chip empty" style:width="{chipW(1)}px"></div>
          {/if}
          {#each emptyPlaces(sp) as pl (pl.id)}
            <!-- Пустое место сетки: светло-серый прямоугольник -->
            <div class="chip empty" style:width="{cellW(pl)}px"></div>
          {/each}
        </div>
        {#if renaming === sp.id}
          <SpreadName id={sp.id} name={sp.name} onclose={() => (renaming = null)} />
        {:else}
          <div
            class="num"
            title={labelTitle(i, sp.name, true)}
            ondblclick={() => prefs.can('names') && (renaming = sp.id)}
            role="presentation"
          >
            {spreadLabel(i, sp.name)}
          </div>
        {/if}
      </div>

      {#if i < n - 1}<JoinLine {i} id={sp.id} {cnt} {gridJoin} {cuts} {max} />{/if}
    {/each}

    <AddColumn />

    {#if n > 0}<div class="edge" style:width="{bw / 2}px"></div>{/if}
  </div>
  <!-- Уголок от разворота к текущей группе: центр ленты = центр разворота -->
  {#if n > 0 && !canvas.zoomed}<div class="pointer"></div>{/if}
</div>

<FixedTip />
{#if menu}<SpreadMenu x={menu.x} y={menu.y} count={menu.ids.length} onDelete={deleteFromMenu} onclose={() => (menu = null)} />{/if}

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
  .group {
    position: relative;
    flex: none;
    padding: var(--pad);
    border-radius: 4px;
    cursor: pointer;
    transition: background 0.15s;
    /* 100+ разворотов: группы вне вида не рисуются. Обрезает всё за краем группы,
       поэтому индикаторы рисуются внутри её отступа. */
    content-visibility: auto;
  }
  .group.current {
    background: rgba(255, 255, 255, 0.14);
  }
  .dragging .group.current {
    background: rgba(245, 217, 10, 0.35);
  }
  .group.over,
  .dragging .group.over {
    background: rgba(245, 217, 10, 0.6);
  }
  /* Выделенные Ctrl/Shift+щелчком группы (спека 019 F) */
  .group.picked {
    outline: 2px solid var(--accent);
    outline-offset: -2px;
  }
  .group.lifted {
    opacity: 0.4;
  }
  .group.ins-before::before,
  .group.ins-after::before {
    content: '';
    position: absolute;
    top: 0;
    bottom: 0;
    width: 3px;
    border-radius: 2px;
    background: var(--accent);
    z-index: 3;
  }
  .group.ins-before::before {
    left: 0;
  }
  .group.ins-after::before {
    right: 0;
  }
  .chips {
    display: flex;
    gap: 2px; /* = GAP */
    height: var(--h);
  }
  .chip {
    position: relative;
    flex: none;
    height: var(--h);
    border: 2px solid #fff; /* = B */
    background: #333;
    cursor: grab;
  }
  .chip img {
    display: block;
    width: 100%;
    height: 100%;
    object-fit: cover;
    pointer-events: none;
    user-select: none;
  }
  /* Повёрнутое фото: картинка лежит по своим размерам, рамка обрезает */
  .chip .rot {
    position: absolute;
    inset: 0;
    overflow: hidden;
  }
  .chip .rot img {
    position: absolute;
    width: auto;
    height: auto;
    max-width: none;
    object-fit: fill;
  }
  .chip.empty {
    background: #d4d4d8;
    cursor: inherit;
  }
  .chip.selected {
    border-color: var(--accent);
  }
  .chip.lifted {
    opacity: 0.35;
  }
  .chip.ins-before::before,
  .chip.ins-after::before {
    content: '';
    position: absolute;
    top: -4px;
    bottom: -4px;
    width: 3px;
    border-radius: 2px;
    background: var(--accent);
    z-index: 3;
  }
  .chip.ins-before::before {
    left: -6px;
  }
  .chip.ins-after::before {
    right: -6px;
  }
  /* Зона разреза: по центру зазора перед фото, захватывает края соседних миниатюр */
  .cut {
    all: unset;
    position: absolute;
    top: -4px;
    bottom: -4px;
    left: -8px; /* позиция от внутреннего края рамки: центр зазора на -3px */
    width: 10px;
    cursor: pointer;
    z-index: 2;
  }
  .cut::before {
    content: '';
    position: absolute;
    top: 0;
    bottom: 0;
    left: 4px;
    border-left: 2px dashed var(--accent);
    opacity: 0;
    transition: opacity 0.1s;
  }
  .cut:hover::before,
  .cut:focus-visible::before {
    opacity: 1;
  }
  .dragging .cut {
    display: none;
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
