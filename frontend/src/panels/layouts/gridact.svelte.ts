// Действия сетки (спека 007/008): закрепить, вернуть «Авто», правка мест, отражение; окна «Вернуть авто?»
// и «Копировать сетку…». Состояние и вызовы Go — здесь; store про них не знает (основа не зовёт верх).
import * as Api from '../../../wailsjs/go/app/App'
import { store } from '../../base/core/store.svelte'
import type { LayoutScope, SpreadView } from '../../base/core/types'

class GridActions {
  // Вопрос «Ваши правки мест сбросятся. Вернуть авто?» (спека 008): разворот и ключ раскладки
  // (пусто — вернуть сохранённый). Окно — AutoAskDialog, открыто, пока не null.
  autoAsk = $state<{ spread: string; key: string } | null>(null)
  // «Копировать сетку…» (U9): разворот-источник и область, окно CopyGridDialog открыто, пока не null.
  // Окно закрывает себя само; prune сбрасывает флаг, если источник пропал или перестал быть сеткой.
  copyGrid = $state<{ spread: string; from: LayoutScope } | null>(null)

  // Вопрос и копирование сетки не переживают разворот, который исчез или вернулся в «Авто» (store.apply).
  prune(spreads: readonly SpreadView[]) {
    if (this.autoAsk && !spreads.some((s) => s.id === this.autoAsk!.spread && s.grid)) this.autoAsk = null
    if (this.copyGrid && !spreads.some((s) => s.id === this.copyGrid!.spread && s.grid)) this.copyGrid = null
  }

  // «Закрепить сетку» / «Вернуть авто» (без вопроса; к «Авто» из интерфейса — через backToAuto).
  async setGrid(on: boolean) {
    const s = store.spread
    if (s) await store.run(Api.SetGrid(s.id, on))
  }

  // Правка мест по ref (SetPlaces, ArrangePlaces, SetPlaceFit, AddPlace). У «Авто» первая такая
  // правка закрепляет разворот в Go; узнаём по ответу (grid был false, стал true) и даём «Вернуть».
  async placeEdit(spread: string, p: Promise<unknown>): Promise<boolean> {
    const h = store
    const was = h.view.spreads.find((s) => s.id === spread)?.grid ?? true
    if (!(await h.run(p))) return false
    if (!was && h.view.spreads.find((s) => s.id === spread)?.grid)
      h.undoToast('Раскладка стала ручной — фото теперь меняются местами через ⇄')
    return true
  }

  // Назад к «Авто» с сетки: места сбросятся, поэтому спрашиваем (по умолчанию «Отмена»).
  // key — выбранная в галерее раскладка; пусто — вернуть сохранённую. У «Авто» вопрос не нужен.
  async backToAuto(spread: string, key = '') {
    if (!store.view.spreads.find((s) => s.id === spread)?.grid) {
      if (key) await store.run(Api.SetLayout(spread, key))
      return
    }
    this.autoAsk = { spread, key }
  }

  async confirmAuto() {
    const a = this.autoAsk
    this.autoAsk = null
    if (a) await store.run(a.key ? Api.SetLayout(a.spread, a.key) : Api.SetGrid(a.spread, false))
  }

  cancelAuto() {
    this.autoAsk = null
  }

  // Отражение сетки; симметричная сетка не меняет rev — объясняем, чтобы кнопка не казалась сломанной.
  async mirrorGrid(scope: LayoutScope) {
    const h = store
    const s = h.spread
    if (!s) return
    const before = h.view.rev
    if ((await h.run(Api.MirrorGrid(s.id, scope))) && h.view.rev === before) h.notify('Раскладка симметрична: отражать нечего')
  }
}

export const gridAct = new GridActions()
