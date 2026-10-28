// Состояние окна «Фоны» (048): что открыто и куда ставить, каталог от Go, вопросы ✎ и ×, загрузка своих.
// Окно рисует DecorDialog; правки проекта идут через store.run (один шаг отмены у Go). Каталог, переименование
// и скрытие — не правки проекта: отмены нет, как у «Мои шаблоны».
import { Api, store } from '../../base/core/store.svelte'
import { errText } from '../../base/words/errtext'
import type { BackdropScope, GridTarget } from '../../base/core/types'
import { isCover, isSheet, sheetIn } from '../../base/geom/sheet'
import { stickerById } from '../../editor/sticker/sticker'
import { prefs } from '../../base/prefs/prefs.svelte'
import { placesel } from '../../editor/places/placesel.svelte'
import {
  ALL,
  cleanName,
  firstSource,
  freshSource,
  importSummary,
  importTheme,
  standingId,
  standingSticker,
  type DecorCatalog,
  type DecorImport,
  type DecorItem,
  type DecorKind,
} from './decor'
import { NO_SEL, type BrowseSel } from '../decorbrowse/browse'
import { albumStanding } from '../albumbg/albumbg'
import { bgNums, bgPickDefault, bgPickShown, bgSelShown, bgTargets, type BgPick } from './bgtargets'
import { spreadsel } from '../timeline/spreadsel.svelte'
import { mergeImport, rasterSvgs } from './svgraster'
import { frameStanding, frameTargets, frameWhereDefault, frameWhereIn, type FrameWhere } from './frames'
import { frameApi } from './frameapi'
import { DecorPut } from './decorput.svelte'
import type { OwnAsk } from '../albumbg/albumown'

// put (067) — вопрос перед постановкой разворота: уходят фото в ленту или заменяются стикеры и надписи.
// bgMany (077) — то же для фона на набор листов: уходят фото фона.
// albumOwn (115) — фон альбома, а у count разворотов свой фон: оставить или сделать их как в альбоме.
export type DecorAsk =
  | { kind: 'rename'; id: string }
  | { kind: 'hide'; id: string; name: string }
  | { kind: 'put'; id: string; text: string }
  | { kind: 'bgMany'; id: string; text: string; targets: GridTarget[] }
  | { kind: 'albumOwn'; id: string; pages: boolean; own: OwnAsk; count: number }

// Открытое окно: что ставим (вид), на каком листе и в какую область. replace — ID стикера, которому ищут замену
// («Заменить…» в карточке «Стикер»); нет — стикер добавляется. Режим — replacing(): стикера на листе нет — добавление (0.28-133-78).
// album — окно «Фон альбома» (098, из настроек и карточки листа): без строки «На», «Куда» — разворот или страницы.
export type DecorCur = { kind: DecorKind; spread: string; scope: BackdropScope; replace?: string; album?: boolean }

const PACK_KINDS = 4 // видов окна с плитками наборов: фоны, стикеры, развороты, рамочки

class DecorUi {
  cur = $state<DecorCur | null>(null)
  cat = $state<DecorCatalog | null>(null)
  theme = $state('')
  // 133: отбор «Набор · Вид · Стиль» помнится до конца сеанса; смена вида окна — всё «Все» (0.28-133-14)
  filters = $state<BrowseSel>({ ...NO_SEL })
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
  // Постановка по щелчку (decorput): состояние окна читает и пишет через this
  private readonly put = new DecorPut(this)
  // 073-1: названия поставленных за сеанс наборов; окно встаёт на тему первого из них один раз для каждого вида.
  private fresh = new Set<string>()
  private shown = new Set<DecorKind>()
  private namesLoaded = false
  private stLoaded = false
  private frLoaded = false

  show(kind: DecorKind, spread: string, scope: BackdropScope, replace?: string) {
    // Темы фонов и стикеров разные: с другим видом начинаем с первой темы, а не с запомненной; отбор тоже «Все».
    if (this.cat && this.cat.kind !== kind) {
      this.theme = ''
      this.filters = { ...NO_SEL }
    }
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

  // ID заменяемого стикера, пока он стоит на листе. Ctrl+Z под окном мог его убрать (Ctrl+Shift+Z вернёт): без стикера
  // окно ведёт себя как «Добавить…» — постановка добавляет, шапка и подсказки без «Заменить» (0.28-133-78).
  replacing = (): string | undefined => {
    const c = this.cur
    return c?.replace && stickerById(sheetIn(store.view, c.spread), c.replace) ? c.replace : undefined
  }

  // Плитка, что стоит в окне сейчас (scope — действующая область листа): фон альбома, рамочка, стикер или фон листа.
  standing(scope: BackdropScope): string {
    const c = this.cur
    if (!c || c.kind === 'spread') return ''
    const sp = sheetIn(store.view, c.spread)
    const items = this.cat?.items ?? []
    if (this.albumNow()) return albumStanding(store.view.albumBg, items)
    if (c.kind === 'frame') return frameStanding(sp, this.frameIds(), items)
    return c.kind === 'sticker' ? standingSticker(sp, c.replace, items) : standingId(sp, scope, items)
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

  // Свежие наборы для открывающегося окна; каждому виду — один раз, показаны всем видам с плитками наборов — забываются.
  // Плитки наборов бывают во всех четырёх видах (фоны, стикеры, развороты, рамочки): до 133 здесь стояло «трём из четырёх».
  private takeFresh(kind: DecorKind): ReadonlySet<string> | undefined {
    if (!this.fresh.size || this.shown.has(kind)) return undefined
    const out = new Set(this.fresh)
    this.shown.add(kind)
    if (this.shown.size >= PACK_KINDS) {
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

  // Ряд отбора: один из трёх выборов; выбор помнится, даже если в теме такого варианта нет.
  setFilter<K extends keyof BrowseSel>(key: K, value: BrowseSel[K]) {
    this.filters = { ...this.filters, [key]: value }
  }

  // «Показать весь набор» в просмотре: «Все» источники, только этот набор, вид и стиль — «Все».
  showPack(packId: string) {
    this.theme = ALL
    this.filters = { ...NO_SEL, pack: packId }
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
      this.theme = firstSource(cat, this.theme, fresh)
      // Прыжок к свежему набору: старый отбор не должен его спрятать
      if (freshSource(cat, fresh)) this.filters = { ...NO_SEL }
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

  // Щелчок по плитке: окно закрывается сразу (как галерея ⊞), правку ставит Go одним шагом (decorput).
  pick(item: DecorItem): Promise<void> {
    return this.put.pick(item)
  }
  // «На все» в тосте после второго подряд того же фона
  putAll(id: string, scope: BackdropScope, from: string, seq?: number): Promise<void> {
    return this.put.putAll(id, scope, from, seq)
  }
  // «Поставить» в вопросах о фоне на набор и о развороте
  confirmBgMany(): Promise<void> {
    return this.put.confirmBgMany()
  }
  answerAlbumOwn(all: boolean): Promise<void> {
    return this.put.answerAlbumOwn(all)
  }
  confirmPut(): Promise<void> {
    return this.put.confirmPut()
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
      if (res.added.length && this.cur) {
        this.setTheme(importTheme(this.theme))
        // 133: набор, стиль, «Уголки», «Разделители» прячут свои — загруженное выглядело бы потерянным; отбор «Все» (0.28-133-77)
        this.filters = { ...NO_SEL }
      }
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
