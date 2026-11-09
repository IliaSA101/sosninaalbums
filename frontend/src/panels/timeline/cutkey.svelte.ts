import { untrack } from 'svelte'
import { cutHeldNext, type CutEv } from './chips'

// Признак «зажат Ctrl (⌘)» для ленты (094 §1): пока он есть, проступают пунктиры разреза и черта клеит.
// Второй экземпляр — gapKey — для промежутков между листами на холсте (103 §3): свои слушатели, пока ряд ≥ 2 листов.
// Третий экземпляр — peekKey — для ленты фото (128): пока зажат Ctrl (⌘), над плитками курсор-лупа — подсказка жеста «Ctrl+правый щелчок».
// Чистое правило — cutHeldNext; здесь только слушатели окна.

// Ctrl/⌘ по последнему событию клавиши или указателя — на уровне модуля, общий для трёх экземпляров: attach при
// повторном подключении (гэп снова ≥ 2 листов, лента вернулась) стартует не с «не зажат», а с этого признака, иначе
// при зажатом Ctrl и неподвижной мыши пунктиры и склейка ждали бы первого движения. Слушатель постоянный: пока
// attach снят, клавиша тоже идёт. В отличие от мини-панели (ministate) keydown здесь включает признак — как и в attach.
const last = { ctrlKey: false, metaKey: false }
function remember(e: Event) {
  const k = e as KeyboardEvent
  last.ctrlKey = k.ctrlKey
  last.metaKey = k.metaKey
}
function forget() {
  last.ctrlKey = false
  last.metaKey = false
}
function onHidden() {
  if (document.hidden) forget()
}
if (typeof window !== 'undefined') {
  const opt = { capture: true, passive: true }
  const types = ['keydown', 'keyup', 'pointermove', 'pointerdown']
  for (const t of types) window.addEventListener(t, remember, opt)
  window.addEventListener('blur', forget, opt)
  document.addEventListener('visibilitychange', onHidden)
  // Горячая замена модуля: старые слушатели снять, иначе копятся по комплекту на правку.
  import.meta.hot?.dispose(() => {
    for (const t of types) window.removeEventListener(t, remember, opt)
    window.removeEventListener('blur', forget, opt)
    document.removeEventListener('visibilitychange', onHidden)
  })
}

export class CutKey {
  held = $state(false)

  // Запись только при смене: pointermove частый, лишних обновлений $state не нужно.
  feed(ev: CutEv) {
    const next = cutHeldNext(ev)
    if (next !== this.held) this.held = next
  }

  // Слушатели на время, пока в ленте есть разрез и склейка; возвращает снятие.
  attach(): () => void {
    // attach зовут из $effect: чтение held внутри feed подписало бы эффект на собственную запись.
    untrack(() => this.feed({ type: 'keydown', ...last }))
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
export const peekKey = new CutKey()
