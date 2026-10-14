<script lang="ts">
  // Ручки выделенного блока (внутри рамки, крутятся вместе с ней): поворот над серединой верхней стороны,
  // две боковые — ширина, четыре угловые — кегль вместе с рамкой. Двойной щелчок по боковой возвращает
  // «по тексту». Жесты ведёт TextGestures.
  import type { Corner } from '../../base/geom/textbox'

  let {
    auto,
    onrotate,
    onresize,
    onauto,
    onscale,
    rotate = true,
    scale = true,
  }: {
    auto: boolean
    rotate?: boolean // ручка поворота (уровень «Полный»)
    scale?: boolean // угловые ручки кегля (уровень «Полный»)
    onrotate: (e: PointerEvent) => void
    onresize: (e: PointerEvent, side: 'l' | 'r') => void
    onauto: () => void
    onscale: (e: PointerEvent, corner: Corner) => void
  } = $props()

  const CORNERS: Corner[] = ['tl', 'tr', 'bl', 'br']

  const side = (e: PointerEvent, s: 'l' | 'r') => {
    if (e.button !== 0) return
    e.stopPropagation()
    onresize(e, s)
  }
  const dbl = (e: MouseEvent) => {
    e.stopPropagation() // не открывать редактор
    if (!auto) onauto()
  }
</script>

{#each scale ? CORNERS : [] as c (c)}
  <div
    class="corner {c}"
    title="Потяните, чтобы изменить размер текста"
    data-text-ui
    role="presentation"
    onpointerdown={(e) => {
      if (e.button !== 0) return
      e.stopPropagation()
      onscale(e, c)
    }}
    ondblclick={(e) => e.stopPropagation()}
  ></div>
{/each}
{#if rotate}
  <div
    class="rot"
    data-guide="text-rotate"
    title="Повернуть (Shift — шаг 15°)"
    data-text-ui
    role="presentation"
    onpointerdown={(e) => {
      if (e.button !== 0) return
      e.stopPropagation()
      onrotate(e)
    }}
    ondblclick={(e) => e.stopPropagation()}
  ></div>
{/if}
<div
  class="handle l"
  title={auto ? 'Потяните, чтобы задать ширину' : 'Двойной щелчок — ширина по тексту'}
  data-text-ui
  role="presentation"
  onpointerdown={(e) => side(e, 'l')}
  ondblclick={dbl}
></div>
<div
  class="handle r"
  title={auto ? 'Потяните, чтобы задать ширину' : 'Двойной щелчок — ширина по тексту'}
  data-text-ui
  role="presentation"
  onpointerdown={(e) => side(e, 'r')}
  ondblclick={dbl}
></div>

<style>
  /* Рамка-носитель мышь не ловит, ручки — да */
  .handle,
  .corner,
  .rot {
    pointer-events: auto;
  }
  .handle {
    position: absolute;
    top: 50%;
    width: 8px;
    height: 24px;
    margin-top: -12px;
    border-radius: 4px;
    border: 1.5px solid var(--accent);
    background: #fff;
    cursor: ew-resize;
    touch-action: none;
  }
  .handle.l {
    left: -10px;
  }
  .handle.r {
    right: -10px;
  }
  /* Угловые: квадрат 9 px по углам рамки; область захвата шире видимого квадрата. */
  .corner {
    position: absolute;
    width: 9px;
    height: 9px;
    box-sizing: border-box;
    border: 1.5px solid var(--accent);
    background: #fff;
    touch-action: none;
  }
  .corner::after {
    content: '';
    position: absolute;
    inset: -5px;
  }
  .corner.tl {
    left: -5px;
    top: -5px;
    cursor: nwse-resize;
  }
  .corner.br {
    right: -5px;
    bottom: -5px;
    cursor: nwse-resize;
  }
  .corner.tr {
    right: -5px;
    top: -5px;
    cursor: nesw-resize;
  }
  .corner.bl {
    left: -5px;
    bottom: -5px;
    cursor: nesw-resize;
  }
  /* Кружок 10 px, центр на 22 px над верхней стороной; ножка — до рамки. */
  .rot {
    position: absolute;
    left: 50%;
    top: -27px;
    width: 10px;
    height: 10px;
    margin-left: -5px;
    box-sizing: border-box;
    border-radius: 50%;
    border: 1.5px solid var(--accent);
    background: #fff;
    cursor: grab;
    touch-action: none;
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
