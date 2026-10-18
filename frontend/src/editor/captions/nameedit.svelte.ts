// Окошко «Имя» на холсте (спека 037): состояние «открыто для фото X» (071: или для пустого места) и разбор
// двойного щелчка по подписи.
import type { Rect, SpreadView } from '../../base/core/types'
import { aabb } from '../../base/geom/textbox'
import { nameTargetAt, whoOf, whoOfPlace, type Who } from './nametarget'

interface Open {
  spread: string
  photo: string // "" — окошко пустого места
  place: string // ID пустого места; у окошка фото ""
  who: Who[]
  box: Rect // мм разворота: от него окошко ставится под подпись
}

class NameEdit {
  cur = $state<Open | null>(null)

  close() {
    this.cur = null
  }

  // Закрыть окошко разворота при уходе с него (Spread стал mini и NameEditor размонтирован).
  release(spread: string) {
    if (this.cur?.spread === spread) this.cur = null
  }

  // Закрыть, если окошко принадлежит не текущему развороту (смена разворота при живом экземпляре Spread).
  sync(currentId: string | undefined) {
    if (this.cur && this.cur.spread !== currentId) this.cur = null
  }

  // Что делать окошку на виде разворота: 'close' — другой разворот или полей имени больше нет (подпись
  // стала своей, фото ушло; у места — его заглушки нет: на нём встало фото); заглушка ↔ подпись фото — одна
  // цель, пока у неё есть who (Go отдаёт who одинаково у обеих, view/captions.go). Табличка места при
  // правке остаётся заглушкой с who — окошко живо.
  verdict(spread: SpreadView, currentId: string | undefined): 'keep' | 'close' {
    const c = this.cur
    if (!c) return 'keep'
    if (c.spread !== spread.id || currentId !== spread.id) return 'close'
    const who = c.place ? whoOfPlace(spread, c.place) : whoOf(spread, c.photo)
    return who.length > 0 ? 'keep' : 'close'
  }

  // Кнопка «Имя под фото» мини-панели: окошко под подписью или заглушкой фото.
  openFor(spread: SpreadView, photo: string) {
    const it = spread.captions.find((c) => c.photo === photo) ?? (spread.stubs ?? []).find((t) => t.photo === photo)
    const who = whoOf(spread, photo)
    if (it && who.length > 0) this.cur = { spread: spread.id, photo, place: '', who, box: aabb(it) }
  }

  // Двойной щелчок по развороту: попал в подпись/заглушку — окошко имени (у пустого места — табличка места), true;
  // мимо — false, остаются прежние жесты («вписать», правка своей подписи). root — рамка разворота.
  // Брать ли цель вообще (редактор, фон, уровень, что под курсором) — takesNameTarget, решает вызывающий.
  dbl(spread: SpreadView, e: MouseEvent, root: HTMLElement): boolean {
    const r = root.getBoundingClientRect()
    const t = nameTargetAt(spread, ((e.clientX - r.left) / r.width) * spread.w, ((e.clientY - r.top) / r.height) * spread.h)
    if (!t) return false
    this.cur = { spread: spread.id, photo: t.photo, place: t.place, who: t.who, box: t.box }
    return true
  }
}

export const nameEdit = new NameEdit()
