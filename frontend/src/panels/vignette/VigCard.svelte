<script lang="ts">
  import type { VigChoice } from '../../base/core/types'
  import { toneOf } from './vignette'

  // Превью шаблона виньетки: места по долям листа, роли разным тоном. Общее у панели в карточке и галереи
  // раскладок; размер задаёт вызывающий, подпись и выбор — тоже его.
  let { c, w, h }: { c: VigChoice; w: number; h: number } = $props()
</script>

<span class="card" class:over={c.over} style:width="{w}px" style:height="{h}px">
  {#each c.boxes as b, i (i)}
    <span
      class="cell {toneOf(c.roles[i])}"
      style:left="{b.x * 100}%"
      style:top="{b.y * 100}%"
      style:width="{b.w * 100}%"
      style:height="{b.h * 100}%"
    ></span>
  {/each}
</span>

<style>
  .card {
    position: relative;
    display: block;
    flex: none;
    background: var(--panel2);
    border: 1px solid var(--line);
    border-radius: 3px;
  }
  .card.over {
    opacity: 0.35;
  }
  /* Роли разным тоном: учителя — светлые, ученики — серые, групповое — акцентное. */
  .cell {
    position: absolute;
    box-sizing: border-box;
    border-radius: 1px;
  }
  .cell.pupil {
    background: #8a8a92;
  }
  .cell.teacher {
    background: #e8e8ea;
  }
  .cell.group {
    background: var(--accent);
  }
</style>
