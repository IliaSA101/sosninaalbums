// Состояние мини-панели, вынесенное из MiniPanel.svelte: «насквозь» при модификаторе и список «⋯» (спека 108).
import { throughNext, type Mods, type ThroughEv } from './minipanel'
import type { Box } from '../../base/ui/popover'

// Модификатор зажат — панель полупрозрачна и не ловит мышь (QA 0.14.1 №1). Признак не копим по keydown/keyup: каждое
// событие указателя несёт настоящее состояние модификаторов (throughNext), а новое положение панели его пересчитывает.
export class Through {
  on = $state(false)
  private last: Mods | undefined // модификаторы последнего события указателя

  // Событие клавиши или указателя окна.
  event(e: KeyboardEvent | PointerEvent) {
    const m: Mods = { shiftKey: e.shiftKey, ctrlKey: e.ctrlKey, altKey: e.altKey, metaKey: e.metaKey }
    if (e instanceof PointerEvent) this.last = m
    this.on = throughNext({ type: e.type, ...m } as ThroughEv)
  }

  // Новое положение панели или другое выделенное: сбросить на состояние последнего указателя.
  placed() {
    this.on = throughNext({ type: 'place', lastPointer: this.last })
  }

  off(type: 'blur' | 'hidden') {
    this.on = throughNext({ type })
  }
}

// Список «⋯»: у столбца открывается сбоку от кнопки, у ряда-запасного — под ней.
export class MoreList {
  open = $state(false)
  btn = $state.raw<HTMLElement>()
  at = $state.raw<Box>({ left: 0, top: 0, width: 0, height: 0 })

  toggle(el: HTMLElement, column: boolean) {
    if (this.open) {
      this.open = false
      return
    }
    this.btn = el
    const r = el.getBoundingClientRect()
    this.at = column
      ? { left: r.left, top: r.top, width: r.width, height: r.height }
      : { left: r.left, top: r.bottom + 4, width: 0, height: 0 }
    this.open = true
  }

  close() {
    this.open = false
  }
}
