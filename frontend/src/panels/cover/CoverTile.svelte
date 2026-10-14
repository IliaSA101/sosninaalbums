<script lang="ts">
  import { store } from '../../base/core/store.svelte'
  import { drag } from '../../base/drag/drag.svelte'
  import { tip } from '../../base/ui/tip.svelte'
  import LowDpiBadge from '../../base/ui/LowDpiBadge.svelte'
  import MissingMark from '../../base/ui/MissingMark.svelte'
  import { missingIn } from '../library/missing'
  import { row } from '../../editor/spread/spreadrow.svelte'
  import Spread from '../../editor/spread/Spread.svelte'
  import { spineNote } from '../../editor/cover/spinenote.svelte'
  import { spreadsel } from '../timeline/spreadsel.svelte'
  import SpreadMenu from '../timeline/SpreadMenu.svelte'
  import { B, H } from '../timeline/timeline'

  // Плитка обложки — первая в ленте (спека 032, п. 1–4). Нет обложки, а развороты есть: пустая плитка «перетащите фото»
  // (бросок создаёт обложку с фото на лицевой, кнопка «+ Обложка» — пустую). Есть: уменьшенный лист, «!» малого
  // разрешения и отметка «Корешок X → Y мм», пока обложку не открыли. Размеры плитки — как у группы разворота.
  const cover = $derived(store.cover)
  const current = $derived(!!cover && row.near === 0)
  // Миниатюра по пропорциям листа из Go (w/h): высота H вместе с рамкой, как у миниатюр групп.
  const thumbW = $derived(cover ? Math.round(((H - 2 * B) * cover.sheet.w) / cover.sheet.h) : 0)
  const over = $derived(drag.over === 'cover' && drag.payload?.kind === 'photos')

  let menu = $state<{ x: number; y: number } | null>(null)

  function open() {
    if (!drag.clickAllowed()) return
    spreadsel.clear()
    store.goCover()
  }
  function openMenu(e: MouseEvent) {
    e.preventDefault()
    e.stopPropagation()
    if (drag.active) return
    tip.hide()
    menu = { x: e.clientX, y: e.clientY }
  }
  function deleteFromMenu() {
    menu = null
    void store.deleteCover()
  }
  function dismiss(e: MouseEvent) {
    e.stopPropagation()
    spineNote.dismiss()
  }
</script>

{#if cover}
  <div
    class="tile"
    class:current
    data-guide="cover-tile"
    data-cover-slot
    role="button"
    tabindex="-1"
    aria-label="Обложка"
    onclick={open}
    oncontextmenu={openMenu}
    onkeydown={() => {}}
  >
    <div class="thumb" style:width="{thumbW + 2 * B}px">
      <Spread spread={cover.sheet} settings={store.view.settings} width={thumbW} mini />
    </div>
    <LowDpiBadge count={cover.sheet.lowDPI} min={store.view.minDPI} />
    <MissingMark count={missingIn(cover.sheet, new Set(store.missing))} />
    {#if spineNote.note}
      <button
        type="button"
        class="note"
        title="Корешок пересчитан по числу разворотов. Щёлкните, чтобы убрать отметку"
        onpointerdown={(e) => e.stopPropagation()}
        onclick={dismiss}>{spineNote.text}</button
      >
    {/if}
    <div class="num">Обложка</div>
  </div>
{:else}
  <div class="empty">
    <div
      class="box"
      class:over
      data-guide="cover-tile"
      data-drop="cover"
      role="presentation"
      title="Перетащите фото сюда — появится обложка с этим фото на лицевой"
    >
      <span class="hint">Перетащите фото</span>
      <button type="button" class="add" onclick={() => drag.clickAllowed() && store.createCover('')}>+ Обложка</button>
    </div>
    <div class="num">Обложка</div>
  </div>
{/if}
<div class="sep" aria-hidden="true"></div>

{#if menu}
  <SpreadMenu x={menu.x} y={menu.y} label="Удалить обложку" onDelete={deleteFromMenu} onclose={() => (menu = null)} />
{/if}

<style>
  .tile {
    position: relative;
    flex: none;
    padding: var(--pad);
    border-radius: 4px;
    cursor: pointer;
    transition: background 0.15s;
  }
  .tile.current {
    background: rgba(255, 255, 255, 0.14);
  }
  .thumb {
    position: relative;
    height: var(--h);
    overflow: hidden;
    pointer-events: none;
    border: 2px solid #fff; /* = B */
  }
  /* Лист в плитке рисуется на ширину миниатюры; рамка (как у фото в группах) — вне её */
  .thumb :global(.spread) {
    box-shadow: none;
  }
  .note {
    position: absolute;
    left: 50%;
    bottom: 20px;
    transform: translateX(-50%);
    z-index: 5;
    height: auto;
    padding: 1px 5px;
    font-size: 10px;
    line-height: 14px;
    border-radius: 7px;
    background: var(--accent);
    color: #1e1e1e;
    font-weight: 600;
  }
  .note:hover:not(:disabled) {
    background: #ffe53a;
  }
  .empty {
    flex: none;
    display: flex;
    flex-direction: column;
    padding: var(--pad);
  }
  .box {
    box-sizing: border-box;
    width: 114px;
    height: var(--h);
    border: 2px dashed #5a5a60;
    border-radius: 4px;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 4px;
    color: #7a7a80;
    font-size: 10px;
    text-align: center;
    transition:
      border-color 0.15s,
      color 0.15s;
  }
  .box.over {
    border-color: var(--accent);
    color: var(--accent);
  }
  .add {
    height: 20px;
    padding: 0 8px;
    font-size: 11px;
  }
  .num {
    text-align: center;
    font-size: 11px;
    line-height: 14px;
    color: var(--muted);
    margin-top: 3px;
    width: 0;
    min-width: 100%;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    user-select: none;
  }
  .tile.current .num {
    color: var(--text);
    font-weight: 600;
  }
  /* Черта отделяет обложку от разворотов (сценарий 1) */
  .sep {
    flex: none;
    align-self: stretch;
    width: 1px;
    margin: 0 8px;
    background: var(--line);
  }
</style>
