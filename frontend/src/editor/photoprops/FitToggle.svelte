<script lang="ts">
  import { placesel } from '../places/placesel.svelte'

  // «Целиком / С обрезкой» выделенного (спека 008): подсвечено, только если так у всех выделенных.
  // У «Авто» «Целиком» закрепляет разворот (gridAct.placeEdit).
  const n = $derived(placesel.selected.length)
</script>

<div class="seg" role="group" aria-label="Фото в месте" data-guide="fit-toggle">
  <button
    class:on={placesel.allContain}
    disabled={n === 0}
    aria-pressed={placesel.allContain}
    onclick={() => placesel.setFit(true)}
    title="Вписать фото в место целиком, без обрезки">Целиком</button
  >
  <button
    class:on={placesel.allCover}
    disabled={n === 0}
    aria-pressed={placesel.allCover}
    onclick={() => placesel.setFit(false)}
    title="Заполнять место с обрезкой">С обрезкой</button
  >
</div>

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
