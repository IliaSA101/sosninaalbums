<script lang="ts">
  // ⟳ на «усике» над серединой верхней стороны рамки (общая для мест и фона): стоит внутри повёрнутой
  // рамки и крутится вместе с ней. Shift — шаг 15°, Alt — без прилипания, двойной щелчок — 0°.
  // rotIn — ⟳ внутри рамки (над ней он вышел бы за разворот). Жест ведёт хозяин рамки.
  let { rotIn = false, onrotate, onzero }: { rotIn?: boolean; onrotate: (e: PointerEvent) => void; onzero: () => void } = $props()
</script>

<div
  class="rot"
  data-guide="rotate-handle"
  class:in={rotIn}
  role="presentation"
  title="Повернуть (Shift — шаг 15°, Alt — без привязки). Двойной щелчок — 0°"
  onpointerdown={(e) => {
    if (e.button !== 0) return
    e.stopPropagation()
    onrotate(e)
  }}
  onclick={(e) => e.stopPropagation()}
  ondblclick={(e) => {
    e.stopPropagation()
    onzero()
  }}
></div>

<style>
  /* Кружок на «усике» над серединой верхней стороны, как у текста. */
  .rot {
    position: absolute;
    left: 50%;
    top: -30px;
    width: 12px;
    height: 12px;
    margin-left: -6px;
    box-sizing: border-box;
    border-radius: 50%;
    border: 1.5px solid var(--accent);
    background: #fff;
    cursor: grab;
    touch-action: none;
    pointer-events: auto;
  }
  .rot::before {
    content: '';
    position: absolute;
    left: 50%;
    top: 100%;
    width: 1.5px;
    height: 14px;
    margin-left: -0.75px;
    background: var(--accent);
  }
  .rot.in {
    top: 18px;
  }
  .rot.in::before {
    top: auto;
    bottom: 100%;
    height: 18px;
  }
  /* Область захвата шире видимого кружка. */
  .rot::after {
    content: '';
    position: absolute;
    inset: -6px;
  }
  .rot:active {
    cursor: grabbing;
  }
</style>
