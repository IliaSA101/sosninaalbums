<script lang="ts">
  import { onDestroy } from 'svelte'
  import { store } from '../../base/core/store.svelte'
  import { drag } from '../../base/drag/drag.svelte'
  import { tip, tipped } from '../../base/ui/tip.svelte'
  import { thumbUrl } from '../../base/core/media'
  import { cancelOnUnmount } from '../../base/ui/imgcancel'
  import { adjlive } from '../../editor/photo/adjlive.svelte'
  import { flipCss, photoCss, thumbBox } from '../../base/geom/frame'
  import { outlineOf } from '../../base/geom/shape'
  import type { SpreadView } from '../../base/core/types'
  import { B, cellW, chipW, cutState, cutTip, emptyPlaces, isBlank } from './timeline'
  import { HOLD_MS, asSheet, sheetThumbW, visibleChips } from './chips'
  import { cutClick, cutKey as cutPress } from './stripact'
  import { cutKey } from './cutkey.svelte'
  import { isMac } from '../../base/core/platform'
  import Spread from '../../editor/spread/Spread.svelte'
  import { unitOf } from '../../base/geom/sheet'

  // Миниатюры одной группы ленты (спека 027 §7): длинная группа свёрнута до 11 плиток и «+N».
  // «+N» и «‹» без data-drop: бросок на них попадает в цель группы — фото встаёт в конец разворота.
  let {
    sp,
    i,
    expanded,
    cuts,
    onpick,
    onexpand,
    onfold,
  }: {
    sp: SpreadView
    i: number
    expanded: string | null
    cuts: boolean
    onpick: (e: MouseEvent, i: number, photo: string, spread: string) => void
    onexpand: () => void
    onfold: () => void
  } = $props()

  const unit = $derived(unitOf(store.view.settings))
  const c = $derived(visibleChips(sp, expanded))
  const places = $derived(emptyPlaces(sp).slice(0, c.places))

  const stop = (e: PointerEvent) => e.stopPropagation()

  function press(e: MouseEvent, act: () => void) {
    e.stopPropagation()
    if (!drag.clickAllowed()) return
    tip.hide()
    act()
  }

  // Фото, зависшее над «+N», раскрывает группу: можно целиться в нужное место среди скрытых.
  let hold: ReturnType<typeof setTimeout> | undefined
  function enter() {
    if (!drag.active || drag.payload?.kind !== 'photos') return
    clearTimeout(hold)
    hold = setTimeout(() => drag.active && onexpand(), HOLD_MS)
  }
  const leave = () => clearTimeout(hold)
  onDestroy(leave)
</script>

{#if asSheet(sp)}
  <!-- Лист виньетки: одна миниатюра листа в его пропорциях (как плитка обложки); щелчок и перенос — у группы -->
  <div class="sheet" style:width="{sheetThumbW(sp) + 2 * B}px">
    <Spread spread={sp} settings={store.view.settings} width={sheetThumbW(sp)} mini />
  </div>
{:else}
  <div class="chips">
    {#each sp.photos.slice(0, c.photos) as id, j (id)}
      <div
        class="chip"
        class:selected={store.selectedCell === id}
        class:ins-before={drag.over === `chip:${sp.id}:${j}`}
        class:ins-after={j === c.photos - 1 && drag.over === `chip:${sp.id}:${j + 1}`}
        class:lifted={drag.active && drag.payload?.kind === 'photos' && drag.payload.fromSpread === sp.id && drag.payload.ids[0] === id}
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
        onclick={(e) => onpick(e, i, id, sp.id)}
        onkeydown={() => {}}
      >
        {#if sp.frames?.[j]?.r}
          <!-- Повёрнутое фото: целиком в своей рамке, лишнее обрезано (как на развороте) -->
          <span class="rot"
            ><img
              src={thumbUrl(id)}
              {@attach cancelOnUnmount}
              alt=""
              loading="lazy"
              draggable="false"
              style={thumbBox(
                sp.rects[j],
                sp.frames[j].img.w / sp.frames[j].img.h,
                sp.frames[j].r,
                outlineOf(sp.frames[j].shape, sp.frames[j].radius),
              )}
              style:transform={photoCss(sp.frames[j])}
              style:filter={adjlive.filterOf(id, sp.frames[j])}
            /></span
          >
        {:else}
          <img
            src={thumbUrl(id)}
            {@attach cancelOnUnmount}
            alt=""
            loading="lazy"
            draggable="false"
            style:transform={flipCss(sp.frames?.[j])}
            style:filter={adjlive.filterOf(id, sp.frames?.[j])}
          />
        {/if}
        {#if j > 0 && sp.kind !== 'vignette' && cuts}
          <!-- Разрез перед этим фото: Ctrl+щелчок или Enter (094 §1); без Ctrl зона мышь не ловит. У ручного — через вопрос (103) -->
          <button
            class="cut"
            aria-label="Разрезать {unit.one} {i + 1} перед фото {j + 1}"
            onpointerdown={stop}
            use:tipped={{ text: cutKey.held ? cutTip(isMac(), cutState(sp, j, unit).ask) : '' }}
            onclick={(e) => cutClick(e, sp.id, j)}
            onkeydown={(e) => cutPress(e, sp.id, j)}
          ></button>
        {/if}
      </div>
    {/each}
    {#if isBlank(sp)}
      <!-- Пустой разворот: квадратная заглушка, без пунктира (пунктир — признак «+») -->
      <div class="chip empty" style:width="{chipW(1)}px"></div>
    {/if}
    {#each places as pl (pl.id)}
      <!-- Пустое место сетки: светло-серый прямоугольник -->
      <div class="chip empty" style:width="{cellW(pl)}px"></div>
    {/each}
    {#if c.more > 0}
      <button
        type="button"
        class="chip tile"
        style:width="{chipW(1)}px"
        aria-label="Показать ещё {c.more}"
        onpointerdown={stop}
        onpointerenter={enter}
        onpointerleave={leave}
        use:tipped={{ text: `Показать ещё ${c.more}` }}
        onclick={(e) => press(e, onexpand)}>+{c.more}</button
      >
    {:else if c.fold}
      <button
        type="button"
        class="chip tile"
        style:width="{chipW(1)}px"
        aria-label="Свернуть"
        onpointerdown={stop}
        use:tipped={{ text: 'Свернуть' }}
        onclick={(e) => press(e, onfold)}>‹</button
      >
    {/if}
  </div>
{/if}

<style>
  .sheet {
    position: relative;
    height: var(--h);
    overflow: hidden;
    pointer-events: none;
    border: 2px solid #fff; /* = B */
  }
  /* Лист в плитке рисуется на ширину миниатюры; рамка — вне его */
  .sheet :global(.spread) {
    box-shadow: none;
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
  /* «+N» и «‹»: плитка-кнопка того же размера, что миниатюра, но тёмная и с цифрой */
  .chip.tile {
    box-sizing: border-box;
    padding: 0;
    display: grid;
    place-items: center;
    background: var(--panel);
    border-color: var(--line);
    color: var(--text);
    font-size: 13px;
    font-weight: 600;
    font-family: inherit;
    cursor: pointer;
  }
  .chip.tile:hover,
  .chip.tile:focus-visible {
    border-color: var(--accent);
    color: var(--accent);
  }
  .chip.selected {
    border-color: var(--accent);
  }
  .chip.lifted {
    opacity: 0.35;
  }
  /* Место вставки — одна черта по центру зазора (095 сц. 6): щель «после j» = «до j+1», черта у левого края j+1;
     у последней видимой — ещё справа. Центр — на −3 px от внутреннего края рамки, как пунктир разреза. */
  .chip.ins-before::before,
  .chip.ins-after::after {
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
    left: -4.5px;
  }
  .chip.ins-after::after {
    right: -4.5px;
  }
  /* Зона разреза (094 §1): белые рамки двух соседних миниатюр и зазор между ними — [-6, 0) от внутреннего края
     рамки, на сами фото не заходит. Мышь ловит только при зажатом Ctrl (.timeline.cutting), иначе щелчок по краю
     выделяет фото, протяжка тянет его. Курсор — ножницы 24×24 (CSP: img-src data:). */
  .cut {
    all: unset;
    position: absolute;
    top: -4px;
    bottom: -4px;
    left: -6px;
    width: 6px;
    pointer-events: none;
    cursor:
      url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='24' height='24' viewBox='0 0 24 24' fill='none' stroke-linecap='round'%3E%3Cg stroke='%23111' stroke-width='4'%3E%3Ccircle cx='6' cy='7' r='3'/%3E%3Ccircle cx='6' cy='17' r='3'/%3E%3Cpath d='M8.5 8.5L20 18M8.5 15.5L20 6'/%3E%3C/g%3E%3Cg stroke='%23fff' stroke-width='2'%3E%3Ccircle cx='6' cy='7' r='3'/%3E%3Ccircle cx='6' cy='17' r='3'/%3E%3Cpath d='M8.5 8.5L20 18M8.5 15.5L20 6'/%3E%3C/g%3E%3C/svg%3E")
        12 12,
      pointer;
    z-index: 2;
  }
  /* Пунктир — по середине зазора [-4, -2) */
  .cut::before {
    content: '';
    position: absolute;
    top: 0;
    bottom: 0;
    left: 2px;
    border-left: 2px dashed var(--accent);
    opacity: 0;
    transition: opacity 0.1s;
  }
  :global(.timeline.cutting) .cut {
    pointer-events: auto;
  }
  :global(.timeline.cutting) .cut::before {
    opacity: 0.45;
  }
  :global(.timeline.cutting) .cut:hover::before,
  .cut:focus-visible::before {
    opacity: 1;
  }
  :global(.dragging) .cut {
    display: none;
  }
</style>
