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

// Склейка и разрез через ручной разворот (103): те же окно и галочка кадра, свои тексты и действие.
// ids — группы пары (при склейке — две подряд, при разрезе — одна); run — вызов Go с галочкой keep.
// faces (130) — «Подогнать по лицам?» на виньетке: то же окно, число правленых кадров приходит в отчёте Go.
export type GlueAsk = {
  kind: 'join' | 'cut' | 'faces'
  title: string
  ask: string
  ok: string
  ids: string[]
  run: (keep: boolean) => Promise<unknown>
}

// Тексты окна вопроса: заголовок, вопрос и кнопка подтверждения (страница — «Вернуть авто?»).
export function autoAskText(vig?: VigAsk, glue?: GlueAsk): { title: string; ask: string; ok: string } {
  if (glue) return glue
  if (vig?.kind === 'tpl') return { title: 'Сменить шаблон?', ask: `${RESET_WARN} Сменить шаблон?`, ok: 'Сменить шаблон' }
  return { title: 'Вернуть авто?', ask: `${RESET_WARN} Вернуть авто?`, ok: 'Вернуть авто' }
}

// Подпись галочки «Вернуть авто?» (098): сколько фото сохранят свой кадр.
export const keepText = (n: number) => `Сохранить кадр правленых фото: ${n}`

// Вопрос склейки и разреза жив, пока группы на месте (при склейке — подряд) и хоть одна из них ручная.
function glueAlive(g: GlueAsk, spreads: readonly SpreadView[]): boolean {
  const at = spreads.findIndex((s) => s.id === g.ids[0])
  if (at < 0 || g.ids.some((id, k) => spreads[at + k]?.id !== id)) return false
  return g.ids.some((_, k) => spreads[at + k].grid)
}

class GridActions {
  // Вопрос «Ваши правки мест сбросятся. Вернуть авто?» (спека 008): разворот и ключ раскладки
  // (пусто — вернуть сохранённый). У ручной виньетки (038) — vig: тот же вопрос про шаблон.
  // Окно — AutoAskDialog, открыто, пока не null. crops — сколько фото получат свой кадр (098, CropsToKeep),
  // keep — галочка «Сохранить кадр правленых фото» (по умолчанию включена; видна при crops > 0).
  autoAsk = $state<{ spread: string; key: string; vig?: VigAsk; glue?: GlueAsk; crops: number; keep: boolean } | null>(null)
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
    if (a && !(a.glue ? glueAlive(a.glue, spreads) : spreads.some((s) => s.id === a.spread && (a.vig ? s.vig?.manual : s.grid))))
      this.autoAsk = null
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
  // Число правленых фото спрашиваем у Go до окна: от него зависит, есть ли в окне галочка (098).
  async backToAuto(spread: string, key = '') {
    if (!store.view.spreads.find((s) => s.id === spread)?.grid) {
      if (key) await store.run(Api.SetLayout(spread, key))
      return
    }
    const crops = await Api.CropsToKeep(spread).catch(() => 0)
    // Пока ждали Go, с разворота могли уйти: вопрос про чужой лист не открываем
    if (store.currentId !== spread) return
    this.autoAsk = { spread, key, crops, keep: true }
  }

  // Склейка или разрез через ручной (103): число правленых кадров по ручным из пары — у Go, затем окно.
  async askGlue(g: GlueAsk) {
    const manual = g.ids.filter((id) => store.view.spreads.find((s) => s.id === id)?.grid)
    const crops = (await Promise.all(manual.map((id) => Api.CropsToKeep(id).catch(() => 0)))).reduce((a, n) => a + n, 0)
    // Пока ждали Go, пара могла измениться (Ctrl+Z, правка): вопрос про исчезнувшее не открываем
    if (!glueAlive(g, store.view.spreads)) return
    this.autoAsk = { spread: g.ids[0], key: '', glue: g, crops, keep: true }
  }

  // То же окно, когда число правленых кадров уже известно (подгон по лицам, 130): Go считает его в отчёте.
  openAsk(g: GlueAsk, crops: number) {
    if (glueAlive(g, store.view.spreads)) this.autoAsk = { spread: g.ids[0], key: '', glue: g, crops, keep: true }
  }

  setKeep(on: boolean) {
    if (this.autoAsk) this.autoAsk.keep = on
  }

  // Действие на виньетке: ручная (vig.manual) — сначала вопрос, «Отмена» ничего не меняет; нетронутая —
  // сразу, без вопроса, как у страницы. true — выполнено сразу, false — ждёт ответа.
  async vigGate(spread: string, kind: VigAsk['kind'], run: () => Promise<unknown>): Promise<boolean> {
    if (!store.view.spreads.find((s) => s.id === spread)?.vig?.manual) {
      await run()
      return true
    }
    this.autoAsk = { spread, key: '', vig: { kind, run }, crops: 0, keep: false }
    return false
  }

  async confirmAuto() {
    const a = this.autoAsk
    this.autoAsk = null
    if (!a) return
    if (a.glue) await a.glue.run(a.crops > 0 && a.keep)
    else if (a.vig) await a.vig.run()
    // Один вызов вместо SetGrid(false) и SetLayout: Go сначала решает судьбу кадров, потом строит раскладку (098-19)
    else await store.run(Api.BackToAuto(a.spread, a.key, a.crops > 0 && a.keep))
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
