// Режим дуги наклона на фото (спеки 059, 063): окно, не проект — в undo, файл и настройки не попадает.
// Кнопка мини-панели включает его, на выделенном фото сверху встаёт дуга (TiltDial) вместо ползунка масштаба и ⟳.
import { store } from '../../base/core/store.svelte'
import { prefs } from '../../base/prefs/prefs.svelte'
import { drag } from '../../base/drag/drag.svelte'
import { modalStack } from '../../base/ui/modalstack.svelte'
import { gallery } from '../../panels/gallery/gallery.svelte'
import { bgmode } from '../backdrop/bgmode.svelte'
import { guide } from '../../shell/guide/guidestate.svelte'
import { miniSpot } from '../places/minispot.svelte'
import { placesel } from '../places/placesel.svelte'
import { framelive } from './framelive.svelte'
import { photoui } from './photoui.svelte'
import { straighten } from './straighten.svelte'

// Режим жив, пока выделено ровно одно фото (без текстов и стикеров), оно на месте, уровень даёт «Наклон»,
// ничего не переносят и режим фона закрыт. Владелец 2026-10-16: выделили другое фото (same: false) — выход;
// дуге не стало места (room: false) — тоже. Чистая проверка под тест.
export function tiltAlive(c: {
  refs: number
  texts: number
  stickers: number
  photo: string | null
  missing: boolean
  canTilt: boolean
  dragging: boolean
  backdrop: boolean
  same?: boolean
  room?: boolean
}): boolean {
  return (
    c.refs === 1 &&
    c.texts === 0 &&
    c.stickers === 0 &&
    !!c.photo &&
    !c.missing &&
    c.canTilt &&
    !c.dragging &&
    !c.backdrop &&
    c.same !== false &&
    c.room !== false
  )
}

// ⟳ места спрятан у фото в режиме: на его месте стоит дуга (владелец 2026-10-16).
export const hidesRotate = (on: boolean, photo: string | undefined, selected: string | null): boolean => on && !!photo && photo === selected

// Показ строки «Наклон, °» в карточке — тем же ходом, что кнопка «Показать в карточке» в справочнике F1 (спека 033):
// якорь data-guide="tilt", подсветка. Подменяется в тестах.
export const tiltRow = {
  show: (): void => void guide.show('tilt', 'ui:tilt'),
}

class TiltMode {
  on = $state(false)
  // Идёт протяжка кружка: Esc тогда отменяет её (обработчик TiltDial), а не выходит из режима.
  dialing = $state(false)
  private photo = '' // фото, на котором включили режим

  constructor() {
    $effect.root(() => {
      $effect(() => this.check())
    })
  }

  alive(): boolean {
    const photo = store.selectedCell
    return tiltAlive({
      refs: store.hooks.selectedRefs().length,
      texts: placesel.texts.length,
      stickers: placesel.stickers.length,
      photo: photo && store.spread?.photos.includes(photo) ? photo : null,
      same: !this.on || photo === this.photo,
      // Пока панель перерисовывается (photo пуст) сведений нет — режим не гасим.
      room: !(this.on && miniSpot.photo === this.photo && miniSpot.arc.mode === 'none'),
      missing: !!photo && store.missing.includes(photo),
      canTilt: prefs.can('tilt'),
      dragging: drag.active,
      backdrop: !!bgmode.cur,
    })
  }

  // Кнопка мини-панели: включить (только если есть на чём) или выключить. Мелкое фото, где дуге нет места, —
  // дуга не рисуется: кнопка ведёт к строке «Наклон, °» в карточке (владелец 2026-10-17).
  toggle() {
    if (this.on) {
      this.exit()
      return
    }
    if (!this.alive()) return
    const photo = store.selectedCell ?? ''
    if (miniSpot.photo === photo && miniSpot.arc.mode === 'none') {
      tiltRow.show()
      return
    }
    this.photo = photo
    this.on = true
  }

  exit() {
    this.on = false
    this.dialing = false
  }

  // Эффект зовёт при любой смене выделения, разворота, уровня, переноса и режима фона.
  check() {
    if (this.on && !this.alive()) this.exit()
  }

  // Esc: выход из режима (выделение остаётся). Не наш, пока открыто окно, галерея или меню фото, идёт протяжка кружка,
  // сдвиг ✥ или включено «по линии», либо Esc уже занят (defaultPrevented). Остальных (слой, палитра) отсекает фаза
  // всплытия в MiniPanel: они забирают Esc в захвате.
  key(e: { key: string; defaultPrevented?: boolean }): boolean {
    if (!this.on || e.key !== 'Escape' || e.defaultPrevented || modalStack.any() || gallery.cur) return false
    if (this.dialing || straighten.cur || photoui.menu || framelive.cur?.pan) return false
    this.exit()
    return true
  }
}

export const tiltmode = new TiltMode()
