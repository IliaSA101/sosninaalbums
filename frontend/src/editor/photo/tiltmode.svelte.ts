// Режим шкалы наклона на фото (спека 059): окно, не проект — в undo, файл и настройки не попадает.
// Кнопка мини-панели включает его, на выделенном фото встаёт шкала (TiltDial) вместо ползунка масштаба.
import { store } from '../../base/core/store.svelte'
import { prefs } from '../../base/prefs/prefs.svelte'
import { drag } from '../../base/drag/drag.svelte'
import { modalStack } from '../../base/ui/modalstack.svelte'
import { gallery } from '../../panels/gallery/gallery.svelte'
import { bgmode } from '../backdrop/bgmode.svelte'
import { placesel } from '../places/placesel.svelte'
import { framelive } from './framelive.svelte'
import { photoui } from './photoui.svelte'
import { straighten } from './straighten.svelte'

// Режим жив, пока выделено ровно одно фото (без текстов и стикеров), оно на месте, уровень даёт «Наклон»,
// ничего не переносят и режим фона закрыт. Чистая проверка под тест. Другое одиночное фото режим не гасит.
export function tiltAlive(c: {
  refs: number
  texts: number
  stickers: number
  photo: string | null
  missing: boolean
  canTilt: boolean
  dragging: boolean
  backdrop: boolean
}): boolean {
  return c.refs === 1 && c.texts === 0 && c.stickers === 0 && !!c.photo && !c.missing && c.canTilt && !c.dragging && !c.backdrop
}

class TiltMode {
  on = $state(false)
  // Идёт протяжка шкалы: Esc тогда отменяет её (обработчик TiltDial), а не выходит из режима.
  dialing = $state(false)

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
      missing: !!photo && store.missing.includes(photo),
      canTilt: prefs.can('tilt'),
      dragging: drag.active,
      backdrop: !!bgmode.cur,
    })
  }

  // Кнопка мини-панели: включить (только если есть на чём) или выключить.
  toggle() {
    if (this.on) this.exit()
    else if (this.alive()) this.on = true
  }

  exit() {
    this.on = false
    this.dialing = false
  }

  // Эффект зовёт при любой смене выделения, разворота, уровня, переноса и режима фона.
  check() {
    if (this.on && !this.alive()) this.exit()
  }

  // Esc: выход из режима (выделение остаётся). Не наш, пока открыто окно, галерея или меню фото, идёт протяжка шкалы,
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
