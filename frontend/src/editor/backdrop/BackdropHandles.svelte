<script lang="ts">
  import { primaryPress } from '../../base/core/platform'
  import type { Side } from '../../base/geom/bdbox'
  import type { Corner } from '../../base/geom/textbox'
  import RotateHandle from '../../base/ui/RotateHandle.svelte'

  // Ручки рамки фона (внутри повёрнутой рамки, крутятся вместе с ней): 4 середины сторон — размер по
  // одной оси, 4 угла — пропорционально, ⟳ на «усике» — поворот; двойной щелчок по ⟳ — 0°.
  // rotIn — ⟳ внутри рамки (над ней не видно). Жесты ведёт BackdropEditor.
  let {
    rotIn,
    onside,
    oncorner,
    onrotate,
    onzero,
  }: {
    rotIn: boolean
    onside: (e: PointerEvent, s: Side) => void
    oncorner: (e: PointerEvent, c: Corner) => void
    onrotate: (e: PointerEvent) => void
    onzero: () => void
  } = $props()

  const CORNERS: Corner[] = ['tl', 'tr', 'bl', 'br']
  const SIDES: Side[] = ['t', 'r', 'b', 'l']
</script>

{#each SIDES as s (s)}
  <div class="side {s}" role="presentation" title="Потяните, чтобы изменить размер" onpointerdown={(e) => onside(e, s)}></div>
{/each}
{#each CORNERS as c (c)}
  <div
    class="corner {c}"
    role="presentation"
    title="Потяните, чтобы изменить размер с сохранением пропорций"
    onpointerdown={(e) => oncorner(e, c)}
  ></div>
{/each}
<RotateHandle {rotIn} accept={primaryPress} {onrotate} {onzero} />

<style>
  .side,
  .corner {
    position: absolute;
    box-sizing: border-box;
    border: 1.5px solid var(--accent);
    background: #fff;
    touch-action: none;
    z-index: 2; /* выше ✥ и ползунка кадра в узкой рамке */
  }
  /* Область захвата шире видимой ручки. */
  .side::after,
  .corner::after {
    content: '';
    position: absolute;
    inset: -5px;
  }
  .side {
    border-radius: 4px;
  }
  .side.t,
  .side.b {
    left: 50%;
    width: 24px;
    height: 8px;
    margin-left: -12px;
    cursor: ns-resize;
  }
  .side.t {
    top: -5px;
  }
  .side.b {
    bottom: -5px;
  }
  .side.l,
  .side.r {
    top: 50%;
    width: 8px;
    height: 24px;
    margin-top: -12px;
    cursor: ew-resize;
  }
  .side.l {
    left: -5px;
  }
  .side.r {
    right: -5px;
  }
  .corner {
    width: 10px;
    height: 10px;
  }
  .corner.tl {
    left: -6px;
    top: -6px;
    cursor: nwse-resize;
  }
  .corner.br {
    right: -6px;
    bottom: -6px;
    cursor: nwse-resize;
  }
  .corner.tr {
    right: -6px;
    top: -6px;
    cursor: nesw-resize;
  }
  .corner.bl {
    left: -6px;
    bottom: -6px;
    cursor: nesw-resize;
  }
</style>
