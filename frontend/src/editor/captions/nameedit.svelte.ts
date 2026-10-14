// Окошко «Имя» на холсте (спека 037): состояние «открыто для фото X» и разбор двойного щелчка по подписи.
import { store } from '../../base/core/store.svelte'
import type { Rect, SpreadView } from '../../base/core/types'
import { aabb } from '../../base/geom/textbox'
import { nameTargetAt, whoOf, type Who } from './nametarget'

interface Open {
  spread: string
  photo: string
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

  // Что делать окошку на виде разворота: 'close' — другой разворот или полей имени у фото больше нет
  // (подпись стала своей, фото ушло); заглушка ↔ подпись фото — одна цель, пока у неё есть who
  // (Go отдаёт who одинаково у обеих, view/captions.go).
  verdict(spread: SpreadView, currentId: string | undefined): 'keep' | 'close' {
    const c = this.cur
    if (!c) return 'keep'
    if (c.spread !== spread.id || currentId !== spread.id) return 'close'
    return whoOf(spread, c.photo).length > 0 ? 'keep' : 'close'
  }

  // Кнопка «Имя под фото» мини-панели: окошко под подписью или заглушкой фото.
  openFor(spread: SpreadView, photo: string) {
    const it = spread.captions.find((c) => c.photo === photo) ?? (spread.stubs ?? []).find((t) => t.photo === photo)
    const who = whoOf(spread, photo)
    if (it && who.length > 0) this.cur = { spread: spread.id, photo, who, box: aabb(it) }
  }

  // Двойной щелчок по развороту: попал в подпись/заглушку — окошко имени (или тост у пустого места), true;
  // мимо — false, остаются прежние жесты («вписать», правка своей подписи). root — рамка разворота.
  // Брать ли цель вообще (редактор, фон, уровень, что под курсором) — takesNameTarget, решает вызывающий.
  dbl(spread: SpreadView, e: MouseEvent, root: HTMLElement): boolean {
    const r = root.getBoundingClientRect()
    const t = nameTargetAt(spread, ((e.clientX - r.left) / r.width) * spread.w, ((e.clientY - r.top) / r.height) * spread.h)
    if (!t) return false
    if (!t.photo) {
      store.notify('Имя едет с фото: поставьте фото на место или вставьте список — «Имена…»')
      return true
    }
    this.cur = { spread: spread.id, photo: t.photo, who: t.who, box: t.box }
    return true
  }
}

export const nameEdit = new NameEdit()
