<script lang="ts">
  import { tipped } from '../../base/ui/tip.svelte'

  // Красный «!» в углу ячейки: фото напечатается с малым dpi (числа — в карточке кадра справа).
  // at — у повёрнутого места: положение в осях ячейки и обратный поворот, чтобы значок стоял прямо у угла
  // описанного прямоугольника (geom/rotbox.ts, uprightAt).
  let { dpi, minDPI, at }: { dpi: number; minDPI: number; at?: { left: string; top: string; rot: number } } = $props()
</script>

{#if dpi > 0 && dpi < minDPI}
  <span
    class="dpi"
    data-guide="dpi"
    class:upright={!!at}
    style:left={at?.left}
    style:top={at?.top}
    style:transform={at ? `rotate(${at.rot}deg)` : undefined}
    role="img"
    aria-label="Мало точек"
    use:tipped={{ text: `≈ ${dpi} dpi при печати, нужно от ${minDPI}. Уменьшите фото на развороте или замените снимок` }}>!</span
  >
{/if}

<style>
  .dpi {
    position: absolute;
    left: 6px;
    top: 6px;
    width: 20px;
    height: 20px;
    border-radius: 50%;
    background: #d9382c;
    color: #fff;
    font-size: 13px;
    font-weight: 800;
    line-height: 20px;
    text-align: center;
    z-index: 4;
    pointer-events: auto; /* подсказка; в CellOverlay обёртка мышь не ловит */
  }
  .dpi.upright {
    transform-origin: 0 0;
  }
</style>
