<script lang="ts">
  import { store } from '../../base/core/store.svelte'
  import { drag } from '../../base/drag/drag.svelte'
  import { row } from '../../editor/spread/spreadrow.svelte'

  // Колонка «+ Новый разворот» в конце ленты. Как группа: рамка, под ней подпись в строке номера —
  // высота ленты одна и та же.
  const n = $derived(store.view.spreads.length)
</script>

<div class="addcol" class:current={row.atNew}>
  <button
    type="button"
    class="add"
    class:over={drag.over === `new:${n}`}
    data-drop="new"
    data-at={n}
    style:aspect-ratio={(2 * store.view.settings.pageW) / store.view.settings.pageH}
    title="Щёлкните — новый пустой разворот; или перетащите сюда фото"
    onclick={(e) => e.detail < 2 && drag.clickAllowed() && store.addEmptySpread(n)}
  >
    +
  </button>
  <div class="num">Новый разворот</div>
</div>

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
    place-items: center;
    color: #7a7a80;
    font-size: 22px;
  }
  /* Слот «новый» по центру ряда — подсвечен, как текущая группа. */
  .addcol.current .add {
    background: rgba(255, 255, 255, 0.14);
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
