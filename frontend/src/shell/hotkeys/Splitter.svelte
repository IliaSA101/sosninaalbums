<script lang="ts">
  import { primaryPress } from '../../base/core/platform'

  // Граница между блоками окна клавиш (074-30): тянется мышью, двойной щелчок — «авто». Родитель — сетка тела окна;
  // положение отдаётся ему в пикселях относительно тела (пределы и доли считает keyslayout.ts).
  let {
    area,
    dir,
    onmove,
    onend,
    onreset,
  }: {
    area: 'hs' | 'vs'
    dir: 'row' | 'col' // row — граница горизонтальная (меняет высоты), col — вертикальная
    onmove: (p: { x: number; y: number }, body: { w: number; h: number }) => void
    onend: () => void
    onreset: () => void
  } = $props()

  let dragging = $state(false)

  function down(e: PointerEvent) {
    if (!primaryPress(e)) return
    ;(e.currentTarget as Element).setPointerCapture(e.pointerId)
    dragging = true
    e.preventDefault()
  }
  function move(e: PointerEvent) {
    if (!dragging) return
    const body = (e.currentTarget as HTMLElement).parentElement
    if (!body) return
    const r = body.getBoundingClientRect()
    onmove({ x: e.clientX - r.left, y: e.clientY - r.top }, { w: r.width, h: r.height })
  }
  function up() {
    if (!dragging) return
    dragging = false
    onend()
  }
</script>

<div
  class="split {area}"
  class:drag={dragging}
  style:grid-area={area}
  role="separator"
  aria-orientation={dir === 'row' ? 'horizontal' : 'vertical'}
  title="Потяните, чтобы изменить размер; двойной щелчок — по умолчанию"
  onpointerdown={down}
  onpointermove={move}
  onpointerup={up}
  onpointercancel={up}
  ondblclick={onreset}
></div>

<style>
  .split {
    position: relative;
    z-index: 2;
    touch-action: none;
    user-select: none;
  }
  .split::before,
  .split::after {
    content: '';
    position: absolute;
    border-radius: 2px;
    transition: background 0.15s;
  }
  .split::before {
    background: var(--line);
  }
  .split::after {
    background: #5a5a60;
    border-radius: 3px;
  }
  .hs {
    cursor: row-resize;
  }
  .vs {
    cursor: col-resize;
  }
  .hs::before {
    left: 0;
    right: 0;
    top: 50%;
    height: 2px;
    margin-top: -1px;
  }
  .hs::after {
    left: 50%;
    top: 50%;
    width: 44px;
    height: 4px;
    margin: -2px 0 0 -22px;
  }
  .vs::before {
    top: 0;
    bottom: 0;
    left: 50%;
    width: 2px;
    margin-left: -1px;
  }
  .vs::after {
    top: 50%;
    left: 50%;
    height: 44px;
    width: 4px;
    margin: -22px 0 0 -2px;
  }
  .split:hover::before,
  .split:hover::after,
  .split.drag::before,
  .split.drag::after {
    background: var(--accent);
  }
</style>
