import { cutHeldNext, type CutEv } from './chips'

// Признак «зажат Ctrl (⌘)» для ленты (094 §1): пока он есть, проступают пунктиры разреза и черта клеит.
// Второй экземпляр — gapKey — для промежутков между листами на холсте (103 §3): свои слушатели, пока ряд ≥ 2 листов.
// Чистое правило — cutHeldNext; здесь только слушатели окна.
export class CutKey {
  held = $state(false)

  // Запись только при смене: pointermove частый, лишних обновлений $state не нужно.
  feed(ev: CutEv) {
    const next = cutHeldNext(ev)
    if (next !== this.held) this.held = next
  }

  // Слушатели на время, пока в ленте есть разрез и склейка; возвращает снятие.
  attach(): () => void {
    const on = (e: KeyboardEvent | PointerEvent) => this.feed(e as unknown as CutEv)
    const blur = () => this.feed({ type: 'blur' })
    const vis = () => document.visibilityState === 'hidden' && this.feed({ type: 'hidden' })
    const opts = { passive: true }
    const types = ['keydown', 'keyup', 'pointermove', 'pointerdown'] as const
    for (const t of types) window.addEventListener(t, on, opts)
    window.addEventListener('blur', blur)
    document.addEventListener('visibilitychange', vis)
    return () => {
      for (const t of types) window.removeEventListener(t, on)
      window.removeEventListener('blur', blur)
      document.removeEventListener('visibilitychange', vis)
      this.held = false
    }
  }
}

export const cutKey = new CutKey()
export const gapKey = new CutKey()
