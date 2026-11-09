// Состояние мини-панели, вынесенное из MiniPanel.svelte: «насквозь» при модификаторе и список «⋯» (спека 108).
import { throughNext, type Mods, type ThroughEv } from './minipanel'
import type { Box } from '../../base/popup/popover'

// Модификаторы последнего события указателя — на уровне модуля, общие для всех экземпляров Through: панель
// пересоздаётся (вероятная причина — мигание рамки выделения при жесте и загрузка миниатюр; гипотеза H1 прогоном не
// подтверждена), а мышь под ней неподвижна — нового события указателя нет, и пустой `last` у свежего экземпляра
// гасил «насквозь» при зажатом Shift (CI, e2e R4). Слушатель постоянный: пока панели нет, указатель тоже идёт,
// иначе после её возврата остались бы устаревшие модификаторы.
const lastPointer: Mods = { shiftKey: false, ctrlKey: false, altKey: false, metaKey: false }
let pointerSeen = false

function setMods(e: KeyboardEvent | PointerEvent) {
  lastPointer.shiftKey = e.shiftKey
  lastPointer.ctrlKey = e.ctrlKey
  lastPointer.altKey = e.altKey
  lastPointer.metaKey = e.metaKey
}
function onPointer(e: Event) {
  setMods(e as PointerEvent)
  pointerSeen = true
}
// Отпускание клавиши несёт настоящие модификаторы: без него Shift, отпущенный без движения мыши, остался бы в
// признаке, и новая панель ушла бы «насквозь» зря. Включает признак только указатель — keydown не слушаем.
function onKeyup(e: Event) {
  setMods(e as KeyboardEvent)
}
// Окно потеряло фокус или скрыто: что с модификаторами, неизвестно — как Through.off().
function forget() {
  setMods({ shiftKey: false, ctrlKey: false, altKey: false, metaKey: false } as PointerEvent)
  pointerSeen = false
}
function onHidden() {
  if (document.hidden) forget()
}
if (typeof window !== 'undefined') {
  const opt = { capture: true, passive: true }
  const types = ['pointermove', 'pointerover', 'pointerdown']
  for (const t of types) window.addEventListener(t, onPointer, opt)
  window.addEventListener('keyup', onKeyup, opt)
  window.addEventListener('blur', forget, opt)
  document.addEventListener('visibilitychange', onHidden)
  // Горячая замена модуля: старые слушатели снять, иначе после каждой правки их копится по комплекту.
  import.meta.hot?.dispose(() => {
    for (const t of types) window.removeEventListener(t, onPointer, opt)
    window.removeEventListener('keyup', onKeyup, opt)
    window.removeEventListener('blur', forget, opt)
    document.removeEventListener('visibilitychange', onHidden)
  })
}

// Модификатор зажат — панель полупрозрачна и не ловит мышь (QA 0.14.1 №1). Признак не копим по keydown/keyup: каждое
// событие указателя несёт настоящее состояние модификаторов (throughNext), а новое положение панели его пересчитывает.
export class Through {
  on = $state(false)

  // Событие клавиши или указателя окна.
  event(e: KeyboardEvent | PointerEvent) {
    const m: Mods = { shiftKey: e.shiftKey, ctrlKey: e.ctrlKey, altKey: e.altKey, metaKey: e.metaKey }
    this.on = throughNext({ type: e.type, ...m } as ThroughEv)
  }

  // Новое положение панели или другое выделенное (или сама панель новая): сбросить на состояние последнего указателя.
  // Развилка: потерянный keyup (Alt+щелчок открыл системное меню) — новая панель верит последнему указателю, как при
  // переезде старой (070), и «насквозь» держится до первого движения мыши; решение — 0.38/decisions.md.
  placed() {
    this.on = throughNext({ type: 'place', lastPointer: pointerSeen ? lastPointer : undefined })
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
