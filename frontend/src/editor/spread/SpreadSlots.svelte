<script lang="ts">
  import Spread from './Spread.svelte'
  import Icon from '../../base/ui/Icon.svelte'
  import { store } from '../../base/core/store.svelte'
  import { drag } from '../../base/drag/drag.svelte'
  import { modes } from '../../base/prefs/view.svelte'
  import { prefs } from '../../base/prefs/prefs.svelte'
  import { addVignette } from '../../panels/vignette/vigact'
  import { row } from './spreadrow.svelte'
  import { AUTO_LABEL, emptyCanvas } from './emptycanvas'
  import { centerAt, pitch, renderRange, slotBox, type SlotBox } from './spreadrow'
  import { labelTitle } from '../../panels/timeline/spreadname'
  import { COVER_ID, unitOf, type SheetSize } from '../../base/geom/sheet'
  import type { Settings } from '../../base/core/types'

  type Box = SlotBox

  // Содержимое окна холста: слоты ряда (развороты и «новый»), пустой альбом. fit — вписанный разворот
  // (px окна, текущий лист), place — действующий (в зуме отличается), k — px на мм, общий для всего ряда,
  // base — размер обычного разворота (слот «новый»), centers — центры слотов (зазор GAP между краями любых листов), spreadW — ширина рисунка текущего.
  let {
    settings,
    k,
    base,
    centers,
    spreadW,
    w,
    fit,
    place,
    zoomed,
    onhover,
  }: {
    settings: Settings
    k: number
    base: SheetSize
    centers: number[]
    spreadW: number
    w: number
    fit: Box
    place: Box
    zoomed: boolean
    onhover: (on: boolean) => void
  } = $props()

  // Листы ряда (032): [обложка?] + развороты. «Новый» разворот встаёт после последнего — его индекс в альбоме
  // (data-at) считается без обложки.
  const sheets = $derived(store.sheets)
  const n = $derived(sheets.length)
  const lead = $derived(store.lead)
  const spreadsN = $derived(store.view.spreads.length)
  const unit = $derived(unitOf(settings))
  const cur = $derived(store.sheetIndex)
  const rowW = $derived(k * base.w)
  const P = $derived(pitch(rowW))
  const range = $derived(renderRange(row.pos, n, row.max, w, rowW, P, zoomed, cur))
  // Ряд едет transform'ом (композитор); в покое transform нет — у position: fixed внутри нет лишнего контейнера.
  const shift = $derived(
    !zoomed && row.pos !== cur ? `translate3d(${centerAt(centers, cur) - centerAt(centers, row.pos)}px, 0, 0)` : undefined,
  )
  // Слот i: от текущего по центрам слотов, размер листа в общем масштабе; в зуме текущий стоит по placement.
  const box = (i: number, size: { w: number; h: number }): Box => (zoomed ? place : slotBox(fit, i, cur, centers, k, size))
  // Подсказка и главная кнопка пустого альбома (U2).
  const empty = $derived(emptyCanvas(modes.big, store.photos.length))
  // Слот «новый» соседом справа уходит под плавающую карточку (запас ряда = её зона): виден лишь левый край,
  // текст по центру обрезался бы — там только «+» у ближнего края, полный текст — когда ряд стоит на слоте.
  const peek = $derived(!row.atNew && !modes.details && prefs.can('panels'))
</script>

{#if n === 0}
  <div
    class="first"
    class:over={drag.over === 'new:0'}
    style:left="{fit.left}px"
    style:top="{fit.top}px"
    style:width="{rowW}px"
    style:height="{k * base.h}px"
    data-drop="new"
    data-at="0"
  >
    {#if store.loaded}
      <div>
        <div class="big">Альбом пуст</div>
        {#each empty.hint as line, i (i)}
          {#if i > 0}<br />{/if}{line}
        {/each}
        <!-- С фото главное — автосборка, пустой разворот и виньетка ссылками; без фото первый разворот одним
             щелчком, как слот «новый» в ряду (бросок фото на рамку тоже создаёт его) -->
        <div class="mk">
          {#if empty.primary === 'auto'}
            <button type="button" class="first-add" title="Собрать альбом из фото ленты" onclick={() => store.openAuto()}
              >{AUTO_LABEL}</button
            >
            <button type="button" class="first-vig" title="Новый пустой {unit.one}" onclick={(e) => e.detail < 2 && store.addEmptySpread(0)}
              >или пустой {unit.one}</button
            >
          {:else}
            <button type="button" class="first-add" title="Новый пустой {unit.one}" onclick={(e) => e.detail < 2 && store.addEmptySpread(0)}
              >+ {unit.new}</button
            >
          {/if}
          {#if prefs.can('vignette')}
            <button type="button" class="first-vig" title="Лист с учителями и учениками" onclick={() => addVignette(0)}
              >или новая виньетка</button
            >
          {/if}
        </div>
      </div>
    {/if}
  </div>
{:else}
  <div class="row" class:moving={row.moving} style:transform={shift}>
    <!-- Ключ — id разворота: один и тот же экземпляр Spread у слота при смене текущего (mini ↔ полный), без
         перемонтирования и мерцания. Не заменять на {#if i === cur} с двумя компонентами или ключ по cur. -->
    {#each range as i (sheets[i]?.id ?? 'new')}
      {@const sp = sheets[i]}
      {@const b = box(i, sp ?? base)}
      {#if sp}
        {@const isCur = i === cur}
        <div
          class="slot"
          style:left="{b.left}px"
          style:top="{b.top}px"
          style:width="{b.width}px"
          style:height="{b.height}px"
          data-current={isCur ? '' : undefined}
          role="presentation"
          onmouseenter={isCur ? () => onhover(true) : undefined}
          onmouseleave={isCur ? () => onhover(false) : undefined}
        >
          <Spread spread={sp} {settings} width={isCur ? spreadW : k * sp.w} mini={!isCur} peer={!isCur} />
          {#if !isCur}
            <!-- Слой соседа: щелчок, перетаскивание ряда и бросок фото «на весь разворот» -->
            <div
              class="peer"
              class:over={drag.over === `spread:${sp.id}`}
              data-peer
              data-drop={sp.id === COVER_ID ? undefined : 'spread'}
              data-spread={sp.id}
              data-index={i - lead}
              role="presentation"
              title={sp.id === COVER_ID ? 'Обложка' : (labelTitle(i - lead, sp.name) ?? `${unit.cap} ${i - lead + 1}`)}
              onclick={() => drag.clickAllowed() && row.to(i)}
            ></div>
          {/if}
        </div>
      {:else}
        <div class="slot" style:left="{b.left}px" style:top="{b.top}px" style:width="{b.width}px" style:height="{b.height}px">
          <button
            type="button"
            class="new"
            class:peek
            class:over={drag.over === `new:${spreadsN}`}
            data-drop="new"
            data-at={spreadsN}
            title="Щёлкните — новый пустой {unit.one}; или перетащите сюда фото"
            onclick={(e) => e.detail < 2 && drag.clickAllowed() && store.addEmptySpread(spreadsN)}
          >
            {#if modes.big || peek}
              <Icon name="plus" size={24} />
            {:else}
              <span>+ {unit.new}<br />перетащите фото сюда</span>
            {/if}
          </button>
        </div>
      {/if}
    {/each}
  </div>
{/if}

<style>
  .row {
    position: absolute;
    inset: 0;
  }
  .row.moving {
    will-change: transform;
  }
  .slot {
    position: absolute;
    z-index: 0; /* свой контекст: слой соседа (z 20) не перекрывает панели текущего */
  }
  /* Рамка выделения, ручки, мини-панель и уголок текущего выходят за лист и не должны уходить под соседей */
  .slot[data-current] {
    z-index: 1;
  }
  .peer {
    position: absolute;
    inset: 0;
    z-index: 20;
    cursor: pointer;
    outline: 3px solid transparent;
    outline-offset: 4px;
    transition: outline-color 0.15s;
  }
  .peer.over {
    outline-color: var(--accent);
  }
  .new {
    width: 100%;
    height: 100%;
    background: none;
    font: inherit;
    cursor: pointer;
    box-sizing: border-box;
    border: 2px dashed #6a6a70;
    border-radius: 4px;
    display: flex;
    align-items: center;
    justify-content: center;
    padding: 4px;
    color: #8a8a90;
    font-size: 13px;
    text-align: center;
    transition:
      border-color 0.15s,
      background 0.15s;
  }
  .new.peek {
    justify-content: flex-start;
    padding-left: 14px;
  }
  .new.over,
  .first.over {
    border-color: var(--accent);
    background: rgba(245, 217, 10, 0.08);
    color: var(--accent);
  }
  .first {
    position: absolute;
    border: 2px dashed #6a6a70;
    display: grid;
    place-items: center;
    text-align: center;
    color: #9a9aa0;
    line-height: 1.6;
  }
  .mk {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 6px;
    margin-top: 14px;
  }
  .first-add {
    height: 32px;
    padding: 0 18px;
    font-size: 14px;
  }
  .first-vig {
    all: unset;
    font-size: 12px;
    color: var(--muted);
    text-decoration: underline;
    cursor: pointer;
  }
  .first-vig:hover {
    color: var(--text);
  }
  .first-vig:focus-visible {
    outline: 2px solid var(--accent);
    outline-offset: 2px;
  }
  .big {
    font-size: 22px;
    color: #ddd;
    margin-bottom: 8px;
  }
</style>
