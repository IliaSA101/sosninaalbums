// Состояние меню миниатюры ленты (спека 106): где открыто, на каких фото, и ответ «где стоит» от Go.
import { store } from '../../base/core/store.svelte'
import { drag } from '../../base/drag/drag.svelte'
import { prefs } from '../../base/prefs/prefs.svelte'
import type { UseSrc } from '../../base/words/whereuse'
import { UseList } from '../../editor/photoprops/uselist.svelte'

export interface ThumbMenuAt {
  x: number
  y: number
  ids: string[] // выделенные на момент открытия (меню держит их, пока не закроется)
}

class ThumbMenuState {
  menu = $state<ThumbMenuAt | null>(null)
  // Вхождения одного фото (общий загрузчик с меню холста); поздний ответ закрытого меню отбрасывает он.
  private readonly list = new UseList()

  // Вхождения одного фото; null — запрос идёт или не нужен.
  get uses(): UseSrc[] | null {
    return this.list.lines
  }

  // Правый щелчок по плитке: выделенное остаётся, если щёлкнули по нему; иначе выделяется только она (как на холсте).
  // «Простой» уровень — меню нет, плитка не трогается.
  open(e: MouseEvent, id: string) {
    e.preventDefault()
    e.stopPropagation()
    if (drag.active || !prefs.can('photoMenu')) return
    if (!store.selected.has(id)) {
      store.selected.clear()
      store.selected.add(id)
      store.anchor = id
    }
    const ids = store.selectedIds
    this.menu = { x: e.clientX, y: e.clientY, ids }
    // Вхождения нужны только одному фото, которое где-то стоит (число на плитке > 0).
    if (ids.length === 1 && store.useCount(id) > 0) this.list.load(id, () => this.close())
    else this.list.reset()
  }

  close() {
    this.list.reset()
    this.menu = null
  }
}

export const thumbmenu = new ThumbMenuState()
