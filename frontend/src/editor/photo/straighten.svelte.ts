// Режим «Выпрямить по линии» (спека 045): окно, не проект — в undo и файл не попадает. Фотограф ведёт линию
// вдоль горизонта; в Go уходит вектор в мм листа, угол считает Go (StraightenPhoto / StraightenBackdrop).
// Рисует линию StraightenLayer, полосу режима — StraightenBar.
import { Api, store } from '../../base/core/store.svelte'
import { prefs } from '../../base/prefs/prefs.svelte'
import { drag } from '../../base/drag/drag.svelte'
import { modalStack } from '../../base/ui/modalstack.svelte'
import { bgmode } from '../backdrop/bgmode.svelte'
import { placesel } from '../places/placesel.svelte'
import { isLine, keepStraighten, lineMM, photoUnder, type StraightenTarget } from './straighten'
import type { Pt } from '../../base/geom/textbox'
import type { SpreadView } from '../../base/core/types'

class Straighten {
  cur = $state<StraightenTarget | null>(null)

  constructor() {
    // Выход, когда цель пропала: сменились выделение или разворот, фото убрали, режим фона закрыт, уровень
    // выключил «Наклон», началось перетаскивание.
    $effect.root(() => {
      $effect(() => this.check())
    })
  }

  // Выделено ровно одно фото и ничего больше (группа и фото с текстом — не цель): как canStraighten в меню.
  private single(): boolean {
    return store.hooks.selectedRefs().length === 1 && placesel.texts.length === 0 && placesel.stickers.length === 0
  }

  // Цель на месте? Нет — выход. Эффект зовёт при любой смене выделения, разворота, режима фона, уровня
  // и перетаскивания (бросок из ленты иначе попал бы в «разворот», а не в место или фон).
  check() {
    const c = this.cur
    if (!c) return
    const alive = keepStraighten(c, store.spread, store.selectedCell, bgmode.cur) && (c.kind !== 'photo' || this.single())
    if (!alive || !prefs.can('tilt') || drag.active) this.exit()
  }

  // Вход для выделенного фото (кнопка «Выпрямить по линии», меню, «Правка ▸ Повернуть ▸»).
  startPhoto(photo = store.selectedCell) {
    const s = store.spread
    if (!prefs.can('tilt')) return
    if (!s || !photo || !this.single() || !s.photos.includes(photo)) {
      store.notify('Выделите одно фото', 'info')
      return
    }
    this.cur = { kind: 'photo', spread: s.id, photo }
  }

  // Вход для фото на фоне: режим фона уже включён.
  startBackdrop() {
    const c = bgmode.cur
    if (!c || !prefs.can('tilt')) return
    this.cur = { kind: 'backdrop', spread: c.spread, scope: c.scope }
  }

  exit() {
    this.cur = null
  }

  // Нажатие мимо цели (p — мм листа): выход; для фото ещё и выделяем другое фото под точкой, как обычный щелчок.
  // Пустое место — только выход.
  miss(spread: SpreadView, p: Pt) {
    const c = this.cur
    this.exit()
    if (c?.kind !== 'photo') return
    const other = photoUnder(spread, p, c.photo)
    if (other) store.selectedCell = other
  }

  // Esc закрывает режим (и гасит событие, чтобы не снять выделение и не выйти из режима фона). true — клавиша наша.
  // Открыто окно (F1, меню) — Esc его: слой ловит клавишу раньше окна и не должен гасить режим за ним.
  key(e: { key: string }): boolean {
    if (!this.cur || e.key !== 'Escape' || modalStack.any()) return false
    this.exit()
    return true
  }

  // Конец протяжки: a и b — точки в px слоя, k — мм на px. Короткая линия — ничего, режим остаётся.
  // Иначе одна правка и выход (в Go холостая правка не даёт шага отмены).
  async finish(a: Pt, b: Pt, k: number): Promise<boolean> {
    const c = this.cur
    if (!c || !isLine(a, b)) return false
    const { dx, dy } = lineMM(a, b, k)
    this.exit()
    await store.run(c.kind === 'photo' ? Api.StraightenPhoto(c.spread, c.photo, dx, dy) : Api.StraightenBackdrop(c.spread, c.scope, dx, dy))
    return true
  }
}

export const straighten = new Straighten()
