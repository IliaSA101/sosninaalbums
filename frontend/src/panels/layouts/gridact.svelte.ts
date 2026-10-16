// Действия сетки (спека 007/008): закрепить, вернуть «Авто», правка мест, отражение; окна «Вернуть авто?»
// и «Копировать сетку…». Состояние и вызовы Go — здесь; store про них не знает (основа не зовёт верх).
import * as Api from '../../../wailsjs/go/app/App'
import { store } from '../../base/core/store.svelte'
import type { LayoutScope, SpreadView } from '../../base/core/types'
import { COVER_ID } from '../../base/geom/sheet'
import { RESET_WARN } from '../gallery/gridgallery'

// Вопрос на ручной виньетке: tpl — смена шаблона (‹ ›, Пробел, плитка в ▦), reset — «Вернуть авто».
// run — само действие (вызов Go), его выполнит confirmAuto; текст окна — vigAskText.
export type VigAsk = { kind: 'tpl' | 'reset'; run: () => Promise<unknown> }

export const VIG_MANUAL_TOAST = 'Виньетка стала ручной — «Вернуть авто» в строке над листом вернёт места по шаблону'

// Тексты окна вопроса: заголовок, вопрос и кнопка подтверждения (страница — «Вернуть авто?»).
export function autoAskText(vig?: VigAsk): { title: string; ask: string; ok: string } {
  if (vig?.kind === 'tpl') return { title: 'Сменить шаблон?', ask: `${RESET_WARN} Сменить шаблон?`, ok: 'Сменить шаблон' }
  return { title: 'Вернуть авто?', ask: `${RESET_WARN} Вернуть авто?`, ok: 'Вернуть авто' }
}

class GridActions {
  // Вопрос «Ваши правки мест сбросятся. Вернуть авто?» (спека 008): разворот и ключ раскладки
  // (пусто — вернуть сохранённый). У ручной виньетки (038) — vig: тот же вопрос про шаблон.
  // Окно — AutoAskDialog, открыто, пока не null.
  autoAsk = $state<{ spread: string; key: string; vig?: VigAsk } | null>(null)
  // «Копировать сетку…» (U9): разворот-источник и область, окно CopyGridDialog открыто, пока не null.
  // Окно закрывает себя само; prune сбрасывает флаг, если источник пропал или перестал быть сеткой.
  // mode 'decor' (049 Ф2) — «Копировать оформление…»: то же окно; источнику сетка не нужна.
  copyGrid = $state<{ spread: string; from: LayoutScope; mode?: 'decor' } | null>(null)

  // Окно «Копировать оформление…» для листа spread; обложка не источник (её сторонки другого размера).
  openDecor(spread: string) {
    if (spread && spread !== COVER_ID) this.copyGrid = { spread, from: 'spread', mode: 'decor' }
  }

  // Вопрос и копирование сетки не переживают разворот, который исчез или вернулся в «Авто» (store.apply).
  prune(spreads: readonly SpreadView[]) {
    const a = this.autoAsk
    // Вопрос виньетки живёт, пока лист остаётся ручной виньеткой (после «Отменить» вопрос не нужен)
    if (a && !spreads.some((s) => s.id === a.spread && (a.vig ? s.vig?.manual : s.grid))) this.autoAsk = null
    const c = this.copyGrid
    // Оформлению сетка не нужна: окно живёт, пока жив сам лист-источник
    if (c && !spreads.some((s) => s.id === c.spread && (c.mode === 'decor' || s.grid))) this.copyGrid = null
  }

  // «Закрепить сетку» / «Вернуть авто» (без вопроса; к «Авто» из интерфейса — через backToAuto).
  async setGrid(on: boolean) {
    const s = store.spread
    if (s) await store.run(Api.SetGrid(s.id, on))
  }

  // Правка мест по ref (SetPlaces, ArrangePlaces, SetPlaceFit, AddPlace). У «Авто» первая такая
  // правка закрепляет разворот в Go; узнаём по ответу (grid был false, стал true) и даём «Вернуть».
  // pageToast=false — без тоста про «Авто» (Отразить, Удалить: у страницы его не было); тост виньетки остаётся.
  async placeEdit(spread: string, p: Promise<unknown>, pageToast = true): Promise<boolean> {
    const h = store
    const find = () => h.view.spreads.find((s) => s.id === spread)
    const was = find()?.grid ?? true
    // Виньетка закреплена всегда (grid = true): её «ручной» признак — vig.manual (038)
    const vig0 = find()?.vig
    const wasVig = vig0 ? !!vig0.manual : true
    if (!(await h.run(p))) return false
    const now = find()
    if (pageToast && !was && now?.grid) h.undoToast('Раскладка стала ручной — фото теперь меняются местами через ⇄')
    else if (!wasVig && now?.vig?.manual) h.undoToast(VIG_MANUAL_TOAST)
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

  // Действие на виньетке: ручная (vig.manual) — сначала вопрос, «Отмена» ничего не меняет; нетронутая —
  // сразу, без вопроса, как у страницы. true — выполнено сразу, false — ждёт ответа.
  async vigGate(spread: string, kind: VigAsk['kind'], run: () => Promise<unknown>): Promise<boolean> {
    if (!store.view.spreads.find((s) => s.id === spread)?.vig?.manual) {
      await run()
      return true
    }
    this.autoAsk = { spread, key: '', vig: { kind, run } }
    return false
  }

  async confirmAuto() {
    const a = this.autoAsk
    this.autoAsk = null
    if (!a) return
    if (a.vig) await a.vig.run()
    else await store.run(a.key ? Api.SetLayout(a.spread, a.key) : Api.SetGrid(a.spread, false))
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
    if ((await this.placeEdit(s.id, Api.MirrorGrid(s.id, scope), false)) && h.view.rev === before)
      h.notify('Раскладка симметрична: отражать нечего')
  }
}

export const gridAct = new GridActions()
