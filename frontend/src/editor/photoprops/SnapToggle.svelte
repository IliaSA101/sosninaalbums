<script lang="ts">
  import { placesel } from '../places/placesel.svelte'

  // Общая «Привязка»: один флаг на места сетки и текст (хранится в placesel, настройка окна).
  // Alt при переносе — без привязки на этот жест.
  // Не забираем фокус у редактора текста: курсор остаётся в нём.
  const keepFocus = (e: MouseEvent) => e.preventDefault()
</script>

<!-- mousedown лишь держит фокус в редакторе текста, действие — на флажке -->
<!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
<label
  class="snap"
  data-guide="snap"
  title="Прилипание к краям, центрам и промежутку (Alt при переносе — без привязки)"
  onmousedown={keepFocus}
>
  <input type="checkbox" checked={placesel.snap} onchange={() => placesel.toggleSnap()} />Привязка
</label>

<style>
  .snap {
    display: flex;
    align-items: center;
    gap: 6px;
    margin-top: 10px;
    font-size: 12px;
    color: #cfcfd4;
    cursor: pointer;
  }
</style>
