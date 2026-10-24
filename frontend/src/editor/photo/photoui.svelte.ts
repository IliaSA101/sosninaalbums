// Состояние меню фото и крупного просмотра (спека 008). Меню открывает правый щелчок по фото,
// месту, стикеру, надписи (099) или пустому листу (099); ref — ID фото или места (как в placesel).
import { drag } from '../../base/drag/drag.svelte'
import { store } from '../../base/core/store.svelte'
import { prefs } from '../../base/prefs/prefs.svelte'
import { modes } from '../../base/prefs/view.svelte'
import { gallery } from '../../panels/gallery/gallery.svelte'
import { bgmode } from '../backdrop/bgmode.svelte'
import { clip } from '../clip/clip.svelte'
import type { Pt } from '../clip/clip'
import { placesel } from '../places/placesel.svelte'
import { markDone } from '../places/placegesture'
import { edit } from '../text/edit.svelte'

// Чьё меню: фото, место и стикер; надпись (по образцу меню стикера); пустой лист («Вставить»).
export type MenuKind = 'photo' | 'text' | 'sheet'

export interface MenuState {
  x: number
  y: number
  kind: MenuKind
  // Точка щелчка на текущем листе, мм: «Вставить» ставит туда центр вставленного; нет точки — решает Go.
  at?: Pt
}

// Точка щелчка на текущем листе, мм (лист — ближайший предок с data-spread). Размер листа в мм приходит из Go,
// здесь только доля рамки; другой лист или нулевой размер — точки нет.
export function pointOnSheet(e: MouseEvent): Pt | undefined {
  const s = store.spread
  const root = (e.target as Element | null)?.closest?.('[data-spread]')
  if (!s || !root || root.getAttribute('data-spread') !== s.id) return undefined
  const r = root.getBoundingClientRect()
  if (r.width <= 0 || r.height <= 0) return undefined
  return { x: ((e.clientX - r.left) / r.width) * s.w, y: ((e.clientY - r.top) / r.height) * s.h }
}

class PhotoUi {
  menu = $state<MenuState | null>(null)
  // ID фото в крупном просмотре.
  preview = $state<string | null>(null)

  // Правый щелчок: выделенное остаётся, если щёлкнули по нему; иначе выделяется только щёлкнутое.
  open(e: MouseEvent, ref: string) {
    e.preventDefault()
    e.stopPropagation()
    if (drag.active) return
    if (!placesel.selected.some((b) => b.ref === ref || b.id === ref || b.photo === ref)) placesel.select([ref])
    this.menu = { x: e.clientX, y: e.clientY, kind: 'photo', at: pointOnSheet(e) }
  }

  // Правый щелчок по стикеру (048): выделенное остаётся, если щёлкнули по нему; иначе выделяется только он.
  openSticker(e: MouseEvent, id: string) {
    e.preventDefault()
    e.stopPropagation()
    if (drag.active) return
    if (!placesel.stickers.includes(id)) placesel.pickSticker(id, false)
    this.menu = { x: e.clientX, y: e.clientY, kind: 'photo', at: pointOnSheet(e) }
  }

  // Правый щелчок по надписи (099): выделенное остаётся, если щёлкнули по нему; иначе выделяется только она.
  // Без функции «Копировать и вставить» меню нет, и надпись не выделяется (как левым щелчком на «Простом»).
  openText(e: MouseEvent, id: string) {
    e.preventDefault()
    e.stopPropagation()
    const sid = store.spread?.id
    if (drag.active || !sid || !prefs.can('clipboard')) return
    // Текст отключён в «Своём» уровне: как левый щелчок — только тост.
    if (prefs.blocked('text', () => edit.select({ kind: 'text', spread: sid, id }))) return
    const picked = placesel.texts.includes(id) || (edit.sel?.kind === 'text' && edit.sel.id === id)
    if (!picked) edit.select({ kind: 'text', spread: sid, id })
    this.menu = { x: e.clientX, y: e.clientY, kind: 'text', at: pointOnSheet(e) }
  }

  // Правый щелчок по пустому листу (099): «Вставить» в эту точку. Молчит там же, где клавиши буфера: ввод
  // надписи (родное меню текста), галерея раскладок, режим фона, крупный просмотр, пустой буфер.
  openSheet(e: MouseEvent) {
    if (drag.active || edit.editing || gallery.cur || bgmode.cur || modes.big) return
    if (!prefs.can('clipboard') || !clip.canPaste) return
    const at = pointOnSheet(e)
    if (!at) return
    e.preventDefault()
    this.menu = { x: e.clientX, y: e.clientY, kind: 'sheet', at }
  }

  // chosen — закрыто выбором пункта: щелчок, дошедший до поля под меню (второй щелчок двойного),
  // выделение снимать не должен.
  close(chosen = false) {
    this.menu = null
    if (chosen) markDone(400)
  }
}

export const photoui = new PhotoUi()
