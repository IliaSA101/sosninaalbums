<script lang="ts">
  import { placesel } from '../places/placesel.svelte'
  import TiltRow from './TiltRow.svelte'

  // Форма выделенного (спека 044): «Прямоугольник · Круг» — как «Целиком / С обрезкой» (подсвечено, только если
  // так у всех выделенных; у «Авто» «Круг» закрепляет разворот). Под переключателем у прямоугольных — «Скругление, %»
  // (строка «Наклон, °» с другими пределами): предпросмотр на ходу, SetPlaceRound — на отпускании. У круга строки нет.
  const n = $derived(placesel.selected.length)
</script>

<div class="seg" role="group" aria-label="Форма места" data-guide="shape-toggle">
  <button
    class:on={placesel.allRect}
    disabled={n === 0}
    aria-pressed={placesel.allRect}
    onclick={() => placesel.setShape(false)}
    title="Прямоугольное место">Прямоугольник</button
  >
  <button
    class:on={placesel.allOval}
    disabled={n === 0}
    aria-pressed={placesel.allOval}
    onclick={() => placesel.setShape(true)}
    title="Круглое место: круг вписан в рамку, при разных Ш и В — овал">Круг</button
  >
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
  .seg {
    display: flex;
    margin-top: 10px;
    border-radius: 5px;
    overflow: hidden;
  }
  .seg button {
    flex: 1;
    height: 26px;
    padding: 0 6px;
    font-size: 12px;
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
