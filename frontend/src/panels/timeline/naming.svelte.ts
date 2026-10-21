import { drag } from '../../base/drag/drag.svelte'
import { prefs } from '../../base/prefs/prefs.svelte'
import { dblOwner, type Click } from './spreadname'

// Поле названия разворота в ленте (094 §3): один владелец на ленту. Открывают двойной щелчок по номеру,
// F2 (команда renameSpread) и «Название…» в меню правого щелчка; закрывает само поле (SpreadName).
type Target = { closest: (sel: string) => { getAttribute: (name: string) => string | null } | null } | null

export type ClickEv = {
  target: unknown
  clientX: number
  clientY: number
  timeStamp: number
  ctrlKey: boolean
  shiftKey: boolean
  metaKey: boolean
  altKey: boolean
  stopPropagation: () => void
  preventDefault: () => void
}

class Naming {
  id = $state<string | null>(null)
  private first: Click | null = null

  start(id: string) {
    this.id = id
  }

  // Закрывает только своё поле: blur старого после открытия нового не должен гасить новое.
  stop(id?: string) {
    if (id === undefined || this.id === id) this.id = null
  }

  // Захват щелчка на корне ленты (094 §3.1). Второй щелчок рядом и вовремя гасится, куда бы ни попал
  // (лента успела уехать), и открывает поле разворота первого. true — щелчок погашен.
  capture(e: ClickEv): boolean {
    const now = { x: e.clientX, y: e.clientY, t: e.timeStamp }
    const owner = dblOwner(this.first, now)
    this.first = null
    if (owner) {
      e.stopPropagation()
      e.preventDefault()
      if (prefs.can('names') && drag.clickAllowed()) this.start(owner)
      return true
    }
    // Первый — только простой щелчок по номеру: с модификаторами это выделение групп.
    if (e.ctrlKey || e.shiftKey || e.metaKey || e.altKey) return false
    const t = e.target as Target
    const id = t?.closest('.num') ? t.closest('[data-spread]')?.getAttribute('data-spread') : null
    if (id) this.first = { id, ...now }
    return false
  }
}

export const naming = new Naming()
