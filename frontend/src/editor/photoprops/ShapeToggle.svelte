<script lang="ts">
  import Seg, { type SegItem } from '../../base/tpl/Seg.svelte'
  import type { ShapeKey } from '../../base/geom/shape'
  import { placesel } from '../places/placesel.svelte'
  import TiltRow from './TiltRow.svelte'
  import { SHAPE_BTNS, SHAPE_ROW, pressedOf } from './shapetoggle'

  // Форма выделенного (044, 076): «Форма ▭ ▢ ◯» — как «Целиком / С обрезкой» (подсвечено, только если так у всех
  // выделенных; у «Авто» ▢ и ◯ закрепляют разворот). Под строкой у некруглых — «Скругление, %» (строка «Наклон, °»
  // с другими пределами): предпросмотр на ходу, SetPlaceRound — на отпускании. У круга строки нет.
  const n = $derived(placesel.selected.length)
  const ITEMS: SegItem<ShapeKey>[] = SHAPE_BTNS.map((b) => ({ id: b.key, label: b.glyph, title: b.title, aria: b.title }))
  const pressed = $derived(SHAPE_BTNS.filter((b) => pressedOf(b.key, placesel)).map((b) => b.key))
</script>

<div class="row" data-guide="shape-toggle">
  <span class="cap">{SHAPE_ROW}</span>
  <Seg look="card" grow big label="Форма места" items={ITEMS} values={pressed} off={n === 0} onpick={(k) => placesel.setShape(k)} />
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
    reset
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
</style>
