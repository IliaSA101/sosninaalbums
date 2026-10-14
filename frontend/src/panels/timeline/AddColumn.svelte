<script lang="ts">
  import { store } from '../../base/core/store.svelte'
  import { drag } from '../../base/drag/drag.svelte'
  import { prefs } from '../../base/prefs/prefs.svelte'
  import { row } from '../../editor/spread/spreadrow.svelte'
  import { newSheetSize, unitOf } from '../../base/geom/sheet'
  import { addVignette } from '../vignette/vigact'
  import SpreadMenu from './SpreadMenu.svelte'

  // Колонка «+ Новый разворот» в конце ленты. Как группа: рамка, под ней подпись в строке номера —
  // высота ленты одна и та же. Вся рамка — одна цель «новый пустой разворот» (щелчок и бросок фото);
  // новая виньетка (031) — в правом щелчке по рамке: отдельные кнопки внутри перехватывали щелчки (решение 80).
  const n = $derived(store.view.spreads.length)
  // Пропорции будущего листа — по настройкам альбома (sheet.ts).
  const newSheet = $derived(newSheetSize(store.view.settings))
  const unit = $derived(unitOf(store.view.settings))
  const canVig = $derived(prefs.can('vignette'))

  let menu = $state<{ x: number; y: number } | null>(null)
  function openMenu(e: MouseEvent) {
    if (!canVig) return
    e.preventDefault()
    e.stopPropagation()
    if (!drag.active) menu = { x: e.clientX, y: e.clientY }
  }
  function newVignette() {
    menu = null
    void addVignette(n)
  }
</script>

<div class="addcol" class:current={row.atNew} data-guide="add-vignette">
  <div class="box">
    <button
      type="button"
      class="add"
      data-guide="add-spread"
      class:over={drag.over === `new:${n}`}
      data-drop="new"
      data-at={n}
      style:aspect-ratio={newSheet.w / newSheet.h}
      title="Щёлкните — новый пустой {unit.one}; или перетащите сюда фото{canVig ? '. Правая кнопка — новая виньетка' : ''}"
      oncontextmenu={openMenu}
      onclick={(e) => e.detail < 2 && drag.clickAllowed() && store.addEmptySpread(n)}
    >
      +
    </button>
  </div>
  <div class="num">{unit.new}</div>
</div>

{#if menu}
  <SpreadMenu x={menu.x} y={menu.y} onNewVignette={newVignette} onclose={() => (menu = null)} />
{/if}

<style>
  .addcol {
    flex: none;
    margin-left: 6px;
    display: flex;
    flex-direction: column;
  }
  /* Пустой альбом: «+» один по центру ленты, отступ слева не нужен. */
  :global(.timeline.empty) .addcol {
    margin-left: 0;
  }
  .box {
    position: relative;
    display: flex;
  }
  /* Пропорции разворота — как пунктир «Альбом пуст» на холсте: цель — новый разворот. */
  .add {
    flex: none;
    box-sizing: content-box;
    padding: 0;
    background: none;
    font-family: inherit;
    cursor: pointer;
    height: calc(var(--h) + 2 * var(--pad));
    border: 2px dashed #5a5a60;
    border-radius: 4px;
    display: grid;
    align-content: center;
    justify-items: center;
    min-width: 66px; /* лист виньетки узкий: плитка остаётся мишенью */
    color: #7a7a80;
    font-size: 26px;
    line-height: 26px;
  }
  /* Слот «новый» по центру ряда — подсвечен, как текущая группа. */
  .addcol.current .add {
    background: rgba(255, 255, 255, 0.14);
    border-color: #7a7a80;
    color: var(--text);
  }
  .add:hover {
    border-color: #7a7a80;
    color: var(--text);
  }
  .add.over {
    border-color: var(--accent);
    color: var(--accent);
  }
  /* Подпись — как номер группы (общая разметка с Timeline) */
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
  .addcol.current .num {
    color: var(--text);
    font-weight: 600;
  }
</style>
