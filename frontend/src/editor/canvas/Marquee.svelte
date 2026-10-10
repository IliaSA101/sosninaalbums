<script lang="ts">
  import { store } from '../../base/core/store.svelte'
  import { placesel } from '../places/placesel.svelte'
  import { canvas } from './canvas.svelte'
  import { markDone } from '../places/placegesture'
  import { additive } from '../places/gesture'
  import { emptyField } from './field'
  import { MARQUEE_MIN, marqueeHits, rectOf, toSpreadMM, type Pt } from './marquee'
  import type { Rect } from '../../base/core/types'

  // Выделение рамкой (спека 009, gestures.md): протяжка по пустому полю разворота и по серому полю вокруг
  // него. Объекты, которых коснулась рамка, выделяются (Shift/Ctrl — добавляются). Начало жеста —
  // Stage.down через start(); рисуется пунктиром в координатах Stage, без вызова Go.
  let { stage }: { stage: HTMLElement | undefined } = $props()

  // Рамка в px Stage; null — жеста нет или порог ещё не пройден.
  let box = $state<Rect | null>(null)

  // Начать жест на нажатии e (левая кнопка, не панорама). Вернёт true, если жест взят.
  export function start(e: PointerEvent): boolean {
    const el = stage
    if (!el || !store.spread || e.button !== 0 || canvas.space || canvas.panning) return false
    if (!emptyField(e.target as Element | null)) return false
    const origin: Pt = { x: e.clientX, y: e.clientY }
    let moved = false
    canvas.busy = true

    const off = () => {
      window.removeEventListener('pointermove', move)
      window.removeEventListener('pointerup', up)
      window.removeEventListener('pointercancel', off)
      window.removeEventListener('keydown', esc, true)
      box = null
      canvas.busy = false
    }
    const move = (ev: PointerEvent) => {
      if (!moved && Math.hypot(ev.clientX - origin.x, ev.clientY - origin.y) < MARQUEE_MIN) return
      moved = true
      const r = el.getBoundingClientRect()
      box = rectOf({ x: origin.x - r.left, y: origin.y - r.top }, { x: ev.clientX - r.left, y: ev.clientY - r.top })
    }
    const up = (ev: PointerEvent) => {
      off()
      if (!moved) return // щелчок: снимет выделение click по полю (Spread.bgClick / Stage.fieldTap)
      markDone() // click после протяжки выделение не трогает
      const sp = el.querySelector<HTMLElement>('[data-current] > .spread')
      if (!sp) return
      const r = sp.getBoundingClientRect()
      const area = rectOf(origin, { x: ev.clientX, y: ev.clientY })
      const mm = toSpreadMM(area, r, store.view.settings.pageW * 2)
      placesel.pickHit(marqueeHits(store.spread, mm), additive(ev))
    }
    const esc = (ev: KeyboardEvent) => {
      if (ev.key !== 'Escape') return
      ev.stopPropagation() // Esc отменяет только жест, выделение остаётся
      off()
      window.addEventListener('pointerup', () => markDone(), { once: true })
    }
    window.addEventListener('pointermove', move)
    window.addEventListener('pointerup', up)
    window.addEventListener('pointercancel', off) // указатель отобран системой: жест отменён
    window.addEventListener('keydown', esc, true)
    return true
  }
</script>

{#if box}
  <div class="marquee" style:left="{box.x}px" style:top="{box.y}px" style:width="{box.w}px" style:height="{box.h}px"></div>
{/if}

<style>
  .marquee {
    position: absolute;
    z-index: 10; /* над разворотом, под полосой (30) и панелями */
    box-sizing: border-box;
    border: 1px dashed var(--accent);
    background: rgba(245, 217, 10, 0.08);
    pointer-events: none;
  }
</style>
