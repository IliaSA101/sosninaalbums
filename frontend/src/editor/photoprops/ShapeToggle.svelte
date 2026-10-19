<script lang="ts">
  import { placesel } from '../places/placesel.svelte'
  import TiltRow from './TiltRow.svelte'
  import { SHAPE_BTNS, SHAPE_ROW, pressedOf } from './shapetoggle'

  // Форма выделенного (044, 076): «Форма ▭ ▢ ◯» — как «Целиком / С обрезкой» (подсвечено, только если так у всех
  // выделенных; у «Авто» ▢ и ◯ закрепляют разворот). Под строкой у некруглых — «Скругление, %» (строка «Наклон, °»
  // с другими пределами): предпросмотр на ходу, SetPlaceRound — на отпускании. У круга строки нет.
  const n = $derived(placesel.selected.length)
</script>

<div class="row" data-guide="shape-toggle">
  <span class="cap">{SHAPE_ROW}</span>
  <div class="seg" role="group" aria-label="Форма места">
    {#each SHAPE_BTNS as b (b.key)}
      {@const on = pressedOf(b.key, placesel)}
      <button class:on disabled={n === 0} aria-pressed={on} aria-label={b.title} title={b.title} onclick={() => placesel.setShape(b.key)}
        >{b.glyph}</button
      >
    {/each}
  </div>
</div>
{#if placesel.hasRound}
  <TiltRow
    tilt={placesel.round}
    label="Скругление, %"
    numLabel="Скругление углов, процент"
    rangeLabel="Скругление"
    rangeTitle="Скругление углов, % меньшей стороны; двойной щелчок — без скругления"
    min={0}
    max={50}
    step={1}
    guide="round"
    id="round"
    onlive={(v) => (placesel.liveRound = v)}
    oncommit={(v) => placesel.setRound(v)}
    onend={() => (placesel.liveRound = null)}
  />
{/if}

<style>
  .row {
    display: flex;
    align-items: center;
    gap: 6px;
    margin-top: 10px;
    font-size: 12px;
  }
  .cap {
    flex: none;
    color: #cfcfd4;
  }
  .seg {
    flex: 1;
    display: flex;
    border-radius: 5px;
    overflow: hidden;
  }
  .seg button {
    flex: 1;
    height: 26px;
    padding: 0 6px;
    font-size: 14px;
    line-height: 1;
    border-radius: 0;
    background: #38383c;
    color: #cfcfd4;
  }
  .seg button.on {
    background: var(--accent);
    color: #1e1e1e;
  }
  .seg button.on:hover:not(:disabled) {
    background: #ffe53a;
  }
</style>
