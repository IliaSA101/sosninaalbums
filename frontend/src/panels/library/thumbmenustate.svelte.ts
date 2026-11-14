// Состояние меню миниатюры ленты (спека 106): где открыто, на каких фото, и ответ «где стоит» от Go.
import { store } from '../../base/core/store.svelte'
import { drag } from '../../base/drag/drag.svelte'
import { photoui } from '../../editor/photo/photoui.svelte'
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

  // Выделенное остаётся, если щёлкнули по нему; иначе выделяется только плитка (как на холсте).
  private pick(id: string) {
    if (store.selected.has(id)) return
    store.selected.clear()
    store.selected.add(id)
    store.anchor = id
  }

  // Ctrl+правый щелчок (128): крупный просмотр вместо меню. Выделение — по тому же правилу, что у меню; на любом уровне.
  peek(e: MouseEvent, id: string) {
    e.preventDefault()
    e.stopPropagation()
    if (drag.active) return
    this.close()
    this.pick(id)
    photoui.openLibrary(id)
  }

  // Правый щелчок по плитке. «Простой» уровень — меню нет, плитка не трогается.
  open(e: MouseEvent, id: string) {
    e.preventDefault()
    e.stopPropagation()
    if (drag.active || !prefs.can('photoMenu')) return
    this.pick(id)
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
