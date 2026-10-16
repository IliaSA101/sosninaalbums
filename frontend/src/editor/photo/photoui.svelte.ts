// Состояние меню фото и крупного просмотра (спека 008). Меню открывает правый щелчок по фото
// или месту; ref — ID фото или места (как в placesel).
import { drag } from '../../base/drag/drag.svelte'
import { placesel } from '../places/placesel.svelte'
import { markDone } from '../places/placegesture'

class PhotoUi {
  menu = $state<{ x: number; y: number } | null>(null)
  // ID фото в крупном просмотре.
  preview = $state<string | null>(null)

  // Правый щелчок: выделенное остаётся, если щёлкнули по нему; иначе выделяется только щёлкнутое.
  open(e: MouseEvent, ref: string) {
    e.preventDefault()
    e.stopPropagation()
    if (drag.active) return
    if (!placesel.selected.some((b) => b.ref === ref || b.id === ref || b.photo === ref)) placesel.select([ref])
    this.menu = { x: e.clientX, y: e.clientY }
  }

  // Правый щелчок по стикеру (048): выделенное остаётся, если щёлкнули по нему; иначе выделяется только он.
  openSticker(e: MouseEvent, id: string) {
    e.preventDefault()
    e.stopPropagation()
    if (drag.active) return
    if (!placesel.stickers.includes(id)) placesel.pickSticker(id, false)
    this.menu = { x: e.clientX, y: e.clientY }
  }

  // chosen — закрыто выбором пункта: щелчок, дошедший до поля под меню (второй щелчок двойного),
  // выделение снимать не должен.
  close(chosen = false) {
    this.menu = null
    if (chosen) markDone(400)
  }
}

export const photoui = new PhotoUi()
