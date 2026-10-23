// Состояние окна «Фоны» (048): что открыто и куда ставить, каталог от Go, вопросы ✎ и ×, загрузка своих.
// Окно рисует DecorDialog; правки проекта идут через store.run (один шаг отмены у Go). Каталог, переименование
// и скрытие — не правки проекта: отмены нет, как у «Мои шаблоны».
import { Api, store } from '../../base/core/store.svelte'
import { errText } from '../../base/words/errtext'
import type { BackdropScope, GridPlan, GridTarget } from '../../base/core/types'
import { isCover, isSheet, sheetIn, unitOf } from '../../base/geom/sheet'
import { prefs } from '../../base/prefs/prefs.svelte'
import { applyBackground } from '../../editor/backdrop/backdropact'
import { currentPage, sticker } from '../../editor/sticker/sticker.svelte'
import { placesel } from '../../editor/places/placesel.svelte'
import {
  cleanName,
  firstTheme,
  importSummary,
  importTheme,
  standingId,
  type DecorCatalog,
  type DecorImport,
  type DecorItem,
  type DecorKind,
  type TplResult,
} from './decor'
import {
  bgAskText,
  bgDoneText,
  bgMissing,
  bgNums,
  bgOfferText,
  bgPickDefault,
  bgPickShown,
  bgSelShown,
  bgStreak,
  bgTargets,
  isMany,
  type BgPick,
  type BgPut,
  type BgPlanInfo,
  type BgResult,
} from './bgtargets'
import { spreadsel } from '../timeline/spreadsel.svelte'
import { putQuestion, putToast } from './spreadtpl'
import { mergeImport, rasterSvgs } from './svgraster'
import { FRAME_NO_PLACES, frameTargets, frameWhereDefault, frameWhereIn, type FrameWhere } from './frames'
import { frameApi } from './frameapi'

// put (067) — вопрос перед постановкой разворота: уходят фото в ленту или заменяются стикеры и надписи.
// bgMany (077) — то же для фона на набор листов: уходят фото фона.
export type DecorAsk =
  | { kind: 'rename'; id: string }
  | { kind: 'hide'; id: string; name: string }
  | { kind: 'put'; id: string; text: string }
  | { kind: 'bgMany'; id: string; text: string; targets: GridTarget[] }

class DecorUi {
  // Открытое окно: что ставим (вид), на каком листе и в какую область. replace — ID стикера, которому ищут замену
  // («Заменить…» в карточке «Стикер»); нет — стикер добавляется.
  // album — окно «Фон альбома» (098, из настроек и карточки листа): без строки «На», «Куда» — разворот или страницы.
  cur = $state<{ kind: DecorKind; spread: string; scope: BackdropScope; replace?: string; album?: boolean } | null>(null)
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
  // 079: плитки «рамочки» — плитка и имя в разделе «Рамочка» карточки.
  frItems = $state<DecorItem[]>([])
  // 077: на какие листы ставить фон — выбор и поле номеров живут, пока окно открыто (show сбрасывает);
  // numsFocus растёт, когда щелчок пришёл при неверных номерах: поле берёт фокус.
  bgPick = $state<BgPick>('this')
  bgText = $state('')
  numsFocus = $state(0)
  // 098: «Куда» для фона альбома — на весь разворот или на каждую страницу; запоминается на сеанс окна (show берёт из альбома)
  albumPages = $state(false)
  // 079: «Куда» окна «Рамочки» — выделенные места или весь лист; сброс при открытии (show)
  frameWhere = $state<FrameWhere>('all')
  private req = 0
  // 100-8: серия «тот же фон на соседний лист»; фоны, о которых уже спрашивали, — до смены альбома (store.openSeq).
  private lastPut: BgPut | null = null
  private offered = new Set<string>()
  private seriesSeq = 0
  // 073-1: названия поставленных за сеанс наборов; окно встаёт на тему первого из них один раз для каждого вида.
  private fresh = new Set<string>()
  private shown = new Set<DecorKind>()
  private namesLoaded = false
  private stLoaded = false
  private frLoaded = false

  show(kind: DecorKind, spread: string, scope: BackdropScope, replace?: string) {
    // Темы фонов и стикеров разные: с другим видом начинаем с первой темы, а не с запомненной.
    if (this.cat && this.cat.kind !== kind) this.theme = ''
    this.cur = replace ? { kind, spread, scope, replace } : { kind, spread, scope }
    this.ask = null
    this.bgText = ''
    this.numsFocus = 0
    this.frameWhere = frameWhereDefault(this.frameSel(spread).length)
    this.albumPages = !!store.view.albumBg?.pages && !isSheet(store.view.settings)
    this.bgPick =
      kind === 'bg' && bgPickShown(sheetIn(store.view, spread), store.view.spreads.length) ? bgPickDefault(this.selIds().length) : 'this'
    void this.refresh(this.takeFresh(kind))
  }

  // 098: окно «Фон альбома» — лист для плиток берём обычный (не обложку: у неё свои размеры), область не нужна.
  showAlbum() {
    const sp = store.spread
    const id = sp && !isCover(sp) ? sp.id : (store.view.spreads[0]?.id ?? sp?.id)
    if (!id) return
    this.show('bg', id, 'spread')
    if (this.cur) this.cur = { ...this.cur, album: true }
    this.bgPick = 'album'
  }

  // Ставим фон альбома: окно «Фон альбома» или пункт «Весь альбом» в строке «На» (она есть не везде — см. bgPickShown).
  albumNow = (): boolean => {
    const c = this.cur
    if (c?.kind !== 'bg') return false
    return !!c.album || (this.bgPick === 'album' && bgPickShown(sheetIn(store.view, c.spread), store.view.spreads.length))
  }

  setAlbumPages(pages: boolean) {
    this.albumPages = pages && !isSheet(store.view.settings)
  }

  // 079: выделенные места листа (ID для ApplyBorder); выделение живёт на текущем листе, на чужом — пусто
  frameSel = (spread: string | undefined = this.cur?.spread): string[] =>
    spread && store.spread?.id === spread ? placesel.selected.map((b) => b.id) : []

  // Действующее «Куда» (выделение могло пропасть) и ID мест, на которые встанет рамочка
  frameWhereNow = (): FrameWhere => frameWhereIn(this.frameWhere, this.frameSel().length)
  frameIds = (): string[] => {
    const c = this.cur
    return c ? frameTargets(sheetIn(store.view, c.spread), this.frameWhereNow(), this.frameSel()) : []
  }

  // Все развороты (без обложки) и выделенные в ленте (077; без «Выделения группы» на уровне — пусто).
  allIds = (): string[] => store.view.spreads.map((s) => s.id)
  selIds = (): string[] => (prefs.can('groupSelect') ? spreadsel.ordered(this.allIds()) : [])

  // Цели фона по выбору в окне: «этот» (прежний путь) или набор. Ошибка номеров — только при выборе «на развороты».
  bgPlan(): { pick: BgPick; targets: GridTarget[]; error: string | null } {
    const c = this.cur
    if (!c) return { pick: 'this', targets: [], error: null }
    const all = this.allIds()
    const sel = this.selIds()
    const nums = bgNums(this.bgText, all)
    const pick = this.bgPick === 'sel' && !bgSelShown(sel.length) ? 'this' : this.bgPick
    const targets = bgTargets({ pick, cur: c.spread, scope: c.scope, sel, nums: nums.ids, all })
    return { pick, targets, error: pick === 'nums' ? nums.error : null }
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

  setFrameWhere(w: FrameWhere) {
    this.frameWhere = w
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
      const cat = c.kind === 'frame' ? await frameApi.catalog() : ((await Api.DecorCatalog(c.kind)) as unknown as DecorCatalog)
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
    if (cat.kind === 'frame') {
      this.frItems = cat.items
      this.frLoaded = true
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

  // Плитки рамочек для карточки: окно ещё не открывали — читаем каталог один раз.
  async ensureFrames() {
    if (this.frLoaded) return
    this.frLoaded = true
    try {
      this.remember(await frameApi.catalog())
    } catch {
      this.frLoaded = false // каталог недоступен — карточка покажет имя без плитки, позже попробуем снова
    }
  }

  // Плитка каталога «рамочки» по ключу плитки (sa:fr-…) или своей (my:…).
  frameItem(key: string | undefined): DecorItem | undefined {
    return key ? this.frItems.find((i) => i.id === key) : undefined
  }

  // Каталог рамочек уже прочитан (U5: без него «нет в библиотеке» не определить).
  get framesReady(): boolean {
    return this.frLoaded
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
    if (c.kind === 'bg') return this.pickBg(c, item)
    if (c.kind === 'frame') return this.pickFrame(c.spread, item)
    this.close()
    if (c.replace) await sticker.replace(c.spread, c.replace, item.id)
    else await sticker.add(c.spread, currentPage(), item.id)
  }

  // Рамочка (079): на выделенные места или на все места листа; окно закрывается сразу, правку ставит Go одним шагом
  // (та же рамочка уже везде — холостая, без шага). Нет мест — подсказка, окно остаётся.
  private async pickFrame(spread: string, item: DecorItem) {
    const ids = this.frameIds()
    if (ids.length === 0) {
      store.notify(FRAME_NO_PLACES)
      return
    }
    this.close()
    await store.run(frameApi.apply(spread, ids, item.id))
  }

  // Фон (077): «этот лист» — прежний путь (окно закрыто, своё — сразу режим фона); набор — сухой прогон, вопрос,
  // постановка одним вызовом. Неверные номера ничего не ставят: фокус в поле, ошибка под ним.
  private async pickBg(c: NonNullable<typeof this.cur>, item: DecorItem) {
    if (this.albumNow()) {
      // 098: одна правка Go — один шаг отмены; тот же фон повторно холостой (rev не меняется)
      const pages = this.albumPages && !isSheet(store.view.settings)
      this.close()
      await store.run(Api.SetAlbumBg(item.id, pages))
      return
    }
    const { pick, targets, error } = this.bgPlan()
    if (error) {
      this.numsFocus++
      return
    }
    if (!isMany(pick, targets, c.spread)) {
      this.close()
      await applyBackground(c.spread, c.scope, item.id)
      this.offerAll(c.spread, c.scope, item.id)
      return
    }
    this.lastPut = null
    if (this.busy) return
    this.busy = true
    try {
      const plan = (await Api.PlanBackgroundTo(targets, item.id)) as unknown as BgPlanInfo
      if (this.cur !== c) return // окно закрыли, пока считался вопрос
      const text = bgAskText(plan, unitOf(store.view.settings))
      if (text) this.ask = { kind: 'bgMany', id: item.id, text, targets }
      else await this.putMany(targets, item.id)
    } catch (e) {
      store.notify(errText(e), 'error')
    } finally {
      this.busy = false
    }
  }

  // 100-8: второй подряд тот же фон на соседний лист — тост «На все развороты?» (один раз на фон за открытие альбома).
  // Только если фон встал: холостая или отменённая постановка серии не продолжает.
  private offerAll(spread: string, scope: BackdropScope, id: string) {
    if (this.seriesSeq !== store.openSeq) {
      this.seriesSeq = store.openSeq
      this.lastPut = null
      this.offered.clear()
    }
    const sp = sheetIn(store.view, spread)
    const index = store.view.spreads.findIndex((s) => s.id === spread)
    if (!bgPickShown(sp, store.view.spreads.length) || index < 0 || standingId(sp, scope, this.bgItems) !== id) {
      this.lastPut = null
      return
    }
    const cur: BgPut = { item: id, scope, index }
    // Прошлый лист серии должен всё ещё стоять с этим фоном: отмену между постановками серия не переживает.
    const prev = this.lastPut ? store.view.spreads[this.lastPut.index] : undefined
    if (this.lastPut && (!prev || standingId(prev, this.lastPut.scope, this.bgItems) !== this.lastPut.item)) this.lastPut = null
    const miss = bgMissing(store.view.spreads, scope, id, this.bgItems)
    if (miss > 0 && bgStreak(this.lastPut, cur, this.offered)) {
      this.offered.add(id)
      const seq = store.openSeq
      store.notify(bgOfferText(miss, unitOf(store.view.settings)), 'info', {
        label: 'На все',
        run: () => void this.putAll(id, scope, spread, seq),
      })
      // Любая правка и Ctrl+Z гасят тост (как «Вернуть»): серия не переживает отмену.
      if (store.toast) store.toast.rev = store.view.rev
    }
    this.lastPut = cur
  }

  // «На все» в тосте: то же, что выбор «Все развороты» в окне — сухой прогон; уходят фото фона — вопрос в окне «Фоны»,
  // иначе сразу ставим. Один шаг отмены (ApplyBackgroundTo).
  async putAll(id: string, scope: BackdropScope, from: string, seq = store.openSeq) {
    if (this.busy || seq !== store.openSeq) return // другой альбом — тост не про него
    const all = this.allIds()
    const cur = sheetIn(store.view, from) ? from : (store.spread?.id ?? all[0])
    if (!cur) return
    const targets = bgTargets({ pick: 'all', cur, scope, sel: [], nums: [], all })
    this.busy = true
    try {
      const plan = (await Api.PlanBackgroundTo(targets, id)) as unknown as BgPlanInfo
      const text = bgAskText(plan, unitOf(store.view.settings))
      if (text) {
        this.show('bg', cur, scope)
        this.bgPick = 'all'
        this.ask = { kind: 'bgMany', id, text, targets }
      } else await this.putMany(targets, id)
    } catch (e) {
      store.notify(errText(e), 'error')
    } finally {
      this.busy = false
    }
  }

  // «Поставить» в вопросе о фоне на набор.
  async confirmBgMany() {
    const a = this.ask
    if (a?.kind !== 'bgMany' || this.busy) return
    this.ask = null
    this.busy = true
    try {
      await this.putMany(a.targets, a.id)
    } finally {
      this.busy = false
    }
  }

  // Один вызов Go — один шаг отмены. Окно закрывается после успеха; ошибка оставляет его открытым.
  private async putMany(targets: GridTarget[], id: string) {
    try {
      const r = (await Api.ApplyBackgroundTo(targets, id)) as unknown as BgResult
      if (!(await store.take(r.view))) return // сбой GetView: красный тост цепочки уже показан (087)
      this.close()
      store.notify(bgDoneText(r.spreads, unitOf(store.view.settings)))
    } catch (e) {
      store.notify(errText(e), 'error')
    }
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
      if (!(await store.take(r.view))) return
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
      // 100-1: окно переходит в тему, куда легло своё («Мои», если грузили из «Все» или «Недавних»)
      if (res.added.length && this.cur) this.setTheme(importTheme(this.theme))
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
    if (!a || !c || (a.kind !== 'rename' && a.kind !== 'hide')) return false
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
