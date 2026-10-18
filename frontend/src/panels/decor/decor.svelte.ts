// Состояние окна «Фоны» (048): что открыто и куда ставить, каталог от Go, вопросы ✎ и ×, загрузка своих.
// Окно рисует DecorDialog; правки проекта идут через store.run (один шаг отмены у Go). Каталог, переименование
// и скрытие — не правки проекта: отмены нет, как у «Мои шаблоны».
import { Api, store } from '../../base/core/store.svelte'
import { errText } from '../../base/words/errtext'
import type { BackdropScope, GridPlan } from '../../base/core/types'
import { applyBackground } from '../../editor/backdrop/backdropact'
import { currentPage, sticker } from '../../editor/sticker/sticker.svelte'
import {
  cleanName,
  firstTheme,
  importSummary,
  importTheme,
  type DecorCatalog,
  type DecorImport,
  type DecorItem,
  type DecorKind,
  type TplResult,
} from './decor'
import { putQuestion, putToast } from './spreadtpl'
import { mergeImport, rasterSvgs } from './svgraster'

// put (067) — вопрос перед постановкой разворота: уходят фото в ленту или заменяются стикеры и надписи.
export type DecorAsk =
  { kind: 'rename'; id: string } | { kind: 'hide'; id: string; name: string } | { kind: 'put'; id: string; text: string }

class DecorUi {
  // Открытое окно: что ставим (вид), на каком листе и в какую область. replace — ID стикера, которому ищут замену
  // («Заменить…» в карточке «Стикер»); нет — стикер добавляется.
  cur = $state<{ kind: DecorKind; spread: string; scope: BackdropScope; replace?: string } | null>(null)
  cat = $state<DecorCatalog | null>(null)
  theme = $state('')
  loading = $state(false)
  ask = $state<DecorAsk | null>(null)
  // Имя в поле — отдельно от ask: окно закрывается раньше, чем bind:value отпустит поле.
  name = $state('')
  busy = $state(false) // идёт загрузка своих: кнопка серая
  // Имена своих по ключу — для карточки фона («Фото фона: имя»), грузятся при первой надобности.
  names = $state<Record<string, string>>({})
  // Плитки каталога «фоны» без окна: имя и цвет узора в карточке берутся из них (presetOf).
  bgItems = $state<DecorItem[]>([])
  // То же для каталога «стикеры» (048 Ф2): имя своего стикера и плитка в карточке «Стикер».
  stItems = $state<DecorItem[]>([])
  private req = 0
  // 073-1: названия поставленных за сеанс наборов; окно встаёт на тему первого из них один раз для каждого вида.
  private fresh = new Set<string>()
  private shown = new Set<DecorKind>()
  private namesLoaded = false
  private stLoaded = false

  show(kind: DecorKind, spread: string, scope: BackdropScope, replace?: string) {
    // Темы фонов и стикеров разные: с другим видом начинаем с первой темы, а не с запомненной.
    if (this.cat && this.cat.kind !== kind) this.theme = ''
    this.cur = replace ? { kind, spread, scope, replace } : { kind, spread, scope }
    this.ask = null
    void this.refresh(this.takeFresh(kind))
  }

  // library:changed: названия поставленных и обновлённых наборов. Удаление (пусто) ничего не меняет.
  note(installed: readonly string[] | null | undefined) {
    if (!installed?.length) return
    installed.forEach((t) => this.fresh.add(t))
    this.shown.clear()
  }

  // Свежие наборы для открывающегося окна; каждому виду — один раз, показаны всем трём — забываются.
  private takeFresh(kind: DecorKind): ReadonlySet<string> | undefined {
    if (!this.fresh.size || this.shown.has(kind)) return undefined
    const out = new Set(this.fresh)
    this.shown.add(kind)
    if (this.shown.size >= 3) {
      this.fresh.clear()
      this.shown.clear()
    }
    return out
  }

  close() {
    this.cur = null
    this.ask = null
    this.loading = false
    this.req++ // запоздавший ответ каталога окну не нужен
  }

  setScope(scope: BackdropScope) {
    if (this.cur) this.cur = { ...this.cur, scope }
  }

  setTheme(id: string) {
    this.theme = id
  }

  // Каталог заново (после ✎, ×, загрузки). Тема остаётся, если ещё есть. Номер запроса отсекает запоздавший ответ.
  // fresh — только при открытии окна (show): открытое окно тему не меняет.
  async refresh(fresh?: ReadonlySet<string>) {
    const c = this.cur
    if (!c) return
    const my = ++this.req
    this.loading = true
    try {
      const cat = (await Api.DecorCatalog(c.kind)) as unknown as DecorCatalog
      if (my !== this.req) return
      this.cat = cat
      this.theme = firstTheme(cat, this.theme, fresh)
      this.remember(cat)
    } catch (e) {
      if (my === this.req) store.notify(errText(e), 'error')
    } finally {
      if (my === this.req) this.loading = false
    }
  }

  // Имена своих и плитки узоров — из каталога вида «фоны»; после ✎ и загрузки обновляются вместе с ним.
  private remember(cat: DecorCatalog) {
    if (cat.kind === 'sticker') {
      this.stItems = cat.items
      this.stLoaded = true
      return
    }
    if (cat.kind !== 'bg') return
    this.bgItems = cat.items
    this.names = Object.fromEntries(cat.items.filter((i) => i.own).map((i) => [i.id, i.name]))
    this.namesLoaded = true
  }

  // Имя своего фона для карточки; окно ещё не открывали — читаем каталог один раз.
  async ensureNames() {
    if (this.namesLoaded) return
    this.namesLoaded = true
    try {
      this.remember((await Api.DecorCatalog('bg')) as unknown as DecorCatalog)
    } catch {
      this.namesLoaded = false // каталог недоступен — карточка покажет «Свой фон», позже попробуем снова
    }
  }

  nameOf(key: string | undefined): string | undefined {
    return key ? this.names[key] : undefined
  }

  // Имена своих стикеров для карточки «Стикер»: окно ещё не открывали — читаем каталог один раз.
  async ensureStickers() {
    if (this.stLoaded) return
    this.stLoaded = true
    try {
      this.remember((await Api.DecorCatalog('sticker')) as unknown as DecorCatalog)
    } catch {
      this.stLoaded = false // каталог недоступен — карточка покажет «Свой стикер», позже попробуем снова
    }
  }

  // Плитка каталога «стикеры» по ключу своего (или id пресета).
  stickerItem(id: string | undefined): DecorItem | undefined {
    return id ? this.stItems.find((i) => i.id === id) : undefined
  }

  // Щелчок по плитке: окно закрывается сразу (как галерея ⊞), правку ставит Go одним шагом. Фон — в область;
  // стикер — в центр страницы под курсором либо вместо выделенного («Заменить…»).
  async pick(item: DecorItem) {
    const c = this.cur
    if (!c) return
    if (c.kind === 'spread') return this.pickSpread(item)
    this.close()
    if (c.kind === 'bg') await applyBackground(c.spread, c.scope, item.id)
    else if (c.replace) await sticker.replace(c.spread, c.replace, item.id)
    else await sticker.add(c.spread, currentPage(), item.id)
  }

  // Развороты (067): сначала вопрос Go (сухой прогон), без чисел для вопроса — ставим сразу. Окно закрывается,
  // когда решено ставить; ошибка прогона оставляет его открытым. Второй щелчок, пока идёт прогон, не нужен.
  private async pickSpread(item: DecorItem) {
    const c = this.cur
    if (!c || this.busy) return
    this.busy = true
    try {
      const plan = (await Api.PlanSpreadTpl(c.spread, c.scope, item.id)) as unknown as GridPlan
      if (this.cur !== c) return // окно закрыли, пока считался вопрос
      const sp = store.view.spreads.find((s) => s.id === c.spread)
      const text = putQuestion(plan, sp, item.places ?? 0)
      if (text) {
        this.ask = { kind: 'put', id: item.id, text }
        return
      }
      this.close()
      await this.putSpread(c.spread, c.scope, item.id)
    } catch (e) {
      store.notify(errText(e), 'error')
    } finally {
      this.busy = false
    }
  }

  // «Поставить» в вопросе: окно разворотов закрывается вместе с ним.
  async confirmPut() {
    const a = this.ask
    const c = this.cur
    if (a?.kind !== 'put' || !c) return
    this.close()
    await this.putSpread(c.spread, c.scope, a.id)
  }

  // Постановка: один вызов Go — места и оформление одним шагом отмены. Версия не выросла — повтор, шага нет.
  private async putSpread(spread: string, scope: BackdropScope, id: string) {
    const before = store.view.rev
    try {
      const r = (await Api.ApplySpreadTpl(spread, scope, id)) as unknown as TplResult
      store.apply(r.view)
      const t = putToast(store.view.rev !== before, r.left, r.missing)
      if (t.undo) store.undoToast(t.text)
      else store.notify(t.text)
    } catch (e) {
      store.notify(errText(e), 'error')
    }
  }

  // «Загрузить своё…»: диалог выбора файлов в Go; итог — тост и свежий каталог.
  async upload() {
    const c = this.cur
    if (!c || this.busy || this.cat?.readOnly) return
    this.busy = true
    try {
      const go = (await Api.ImportDecor(c.kind, importTheme(this.theme))) as unknown as DecorImport
      // SVG от Go рисуем здесь по очереди; итог идёт в тот же тост, что и остальные файлы.
      const res = go.svg?.length ? mergeImport(go, await rasterSvgs(go.svg, Api.AddDecorPNG)) : go
      const sum = importSummary(res)
      if (sum) store.notify(sum.text, sum.error ? 'error' : 'info')
      if (this.cur) await this.refresh()
    } catch (e) {
      store.notify(errText(e), 'error')
    } finally {
      this.busy = false
    }
  }

  askRename(it: Pick<DecorItem, 'id' | 'name'>) {
    this.name = it.name
    this.ask = { kind: 'rename', id: it.id }
  }

  askHide(it: Pick<DecorItem, 'id' | 'name'>) {
    this.ask = { kind: 'hide', id: it.id, name: it.name }
  }

  dismiss() {
    this.ask = null
  }

  // «Сохранить» / «Убрать»: вопрос закрывается сразу, каталог перечитывается после успеха.
  async commit(): Promise<boolean> {
    const a = this.ask
    const c = this.cur
    if (!a || !c || a.kind === 'put') return false
    this.ask = null
    try {
      if (a.kind === 'rename') await Api.RenameDecor(c.kind, a.id, cleanName(this.name))
      else await Api.HideDecor(c.kind, a.id)
    } catch (e) {
      store.notify(errText(e), 'error')
      return false
    }
    await this.refresh()
    return true
  }
}

export const decor = new DecorUi()
