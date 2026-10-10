<script lang="ts">
  import { store } from '../../base/core/store.svelte'
  import { placesel } from '../places/placesel.svelte'
  import { gallery } from '../../panels/layouts/gallery.svelte'
  import { prefs } from '../../base/prefs/prefs.svelte'
  import Icon from '../../base/ui/Icon.svelte'
  import { canvas } from '../canvas/canvas.svelte'
  import { row } from './spreadrow.svelte'

  const total = $derived(store.view.spreads.length)
  const sp = $derived(store.spread)
  // Номер — по ближайшему к центру слоту ряда (на слоте «новый» — последний разворот)
  const shown = $derived(Math.min(row.near, total - 1) + 1)
  // Блок раскладок держит место и когда листать нечего, чтобы стрелки разворотов не плавали.
  // Без функции «Варианты раскладки» (спека 011) блок скрыт, место за ним остаётся.
  const canStep = $derived(prefs.can('variants') && !!sp?.layout && (sp.variants > 1 || sp.variant < 0))
</script>

<!-- Три колонки 1fr / auto / 1fr: навигация всегда по центру, действия — справа
     в своей колонке и не двигают её, даже когда меняется набор кнопок.
     Узкая колонка (панель деталей, малое окно) — действия становятся значками. -->
<div class="bar">
  {#if sp}
    <div class="nav">
      <button class="icon" disabled={row.near === 0} onclick={() => row.step(-1)} title="Предыдущий (←)" aria-label="Предыдущий разворот"
        ><Icon name="chevron-left" /></button
      >
      <span class="label counter">Разворот <b>{shown}</b> из {total}</span>
      <button
        class="icon"
        disabled={row.atNew || (canvas.zoomed && store.current >= total - 1)}
        onclick={() => row.step(1)}
        title="Следующий (→)"
        aria-label="Следующий разворот"><Icon name="chevron-right" /></button
      >
      <!-- Ячейка фиксированной ширины: название не двигает стрелки при листании -->
      <span class="name" title={sp.name || undefined}>{sp.name ? `· ${sp.name}` : ''}</span>

      <div class="variants" class:hidden={!canStep} inert={!canStep}>
        <div class="sep"></div>
        <button
          class="icon"
          onclick={() => gallery.step('spread', -1)}
          title="Предыдущая раскладка (Shift+пробел)"
          aria-label="Предыдущая раскладка"><Icon name="caret-left" size={12} /></button
        >
        <span class="label counter" title="Другие варианты расположения тех же фото"
          >Раскладка {sp.variant >= 0 ? sp.variant + 1 : '—'} / {sp.variants}</span
        >
        <button class="icon" onclick={() => gallery.step('spread', 1)} title="Другая раскладка (пробел)" aria-label="Следующая раскладка"
          ><Icon name="caret-right" size={12} /></button
        >
        <!-- Место под пометку держится всегда: навигация не прыгает -->
        <span class="mark" class:hidden={!sp.rearranged} title="Фото стоят не в порядке ленты">· перестановка</span>
      </div>
    </div>

    <div class="actions">
      <div class="sep"></div>
      <button
        class="ghost act"
        disabled={!store.selectedCell}
        onclick={() => placesel.deleteSelected()}
        title="Убрать выбранное фото (Del)"
        aria-label="Убрать фото"><Icon name="photo-minus" size={18} stroke={1.8} /><span class="text">Убрать фото (Del)</span></button
      >
      <button class="ghost act" onclick={() => store.deleteSpread()} title="Удалить разворот" aria-label="Удалить разворот"
        ><Icon name="spread-delete" size={18} stroke={1.8} /><span class="text">Удалить разворот</span></button
      >
    </div>
  {/if}
</div>

<style>
  .bar {
    flex: none;
    height: 40px;
    display: grid;
    grid-template-columns: 1fr auto 1fr;
    align-items: center;
    font-size: 13px;
    color: var(--muted);
    white-space: nowrap;
    container-type: inline-size;
  }
  .nav {
    grid-column: 2;
    display: flex;
    align-items: center;
    gap: 6px;
  }
  .variants,
  .actions {
    display: flex;
    align-items: center;
    gap: 6px;
  }
  .variants.hidden,
  .mark.hidden {
    visibility: hidden;
  }
  .mark {
    color: var(--accent);
  }
  .actions {
    grid-column: 3;
    justify-self: start;
    min-width: 0;
  }
  b {
    color: var(--text);
  }
  .counter {
    font-variant-numeric: tabular-nums;
    text-align: center;
  }
  /* Ширина по самому длинному значению («Разворот 999 из 999») — цифры не двигают стрелки */
  .nav > .counter {
    min-width: 138px;
  }
  .name {
    width: 180px;
    overflow: hidden;
    text-overflow: ellipsis;
    color: var(--text);
  }
  .variants .counter {
    min-width: 118px;
  }
  .sep {
    width: 1px;
    height: 20px;
    background: var(--line);
    margin: 0 10px;
  }
  .icon {
    width: 28px;
    height: 28px;
    padding: 0;
    display: inline-grid;
    place-items: center;
  }
  /* Значок слева от подписи */
  .act {
    display: inline-flex;
    align-items: center;
    gap: 6px;
  }
  /* Ширина, при которой подписи действий уже не влезают в свою колонку 1fr
     (навигация ≈ 706 px с ячейкой названия + две колонки по ≈ 343 px — ширине действий со значками и подписями). */
  @container (max-width: 1402px) {
    .act {
      width: 32px;
      padding: 0;
      justify-content: center;
    }
    .act .text {
      display: none;
    }
  }
</style>
