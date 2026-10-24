import * as Api from '../../../wailsjs/go/app/App'
import { EventsOn } from '../../../wailsjs/runtime/runtime'
import { SvelteSet } from 'svelte/reactivity'
import { toggleKey } from './platform'
import { errText, withSkipped } from '../words/errtext'
import { Nav } from './nav.svelte'
import { jobs } from './jobs.svelte'
import { AutoRun } from '../auto/autorun.svelte'
import { SpreadOps } from './spreadops.svelte'
import { Toasts, stepToast, undoToast, type Toast } from './toasts.svelte'
import type { MarkMap, PickResult } from './hooks'
import { pickCurrent } from './undo'
import { deepFreeze } from './viewdelta'
import { ViewChain } from './viewchain'
import { keepSet } from './usedindex'
import { mergeSnapshot, noEvents } from './startstate'
import { movedFrom, movedText, type Moved } from './strip'
import { prefs } from '../prefs/prefs.svelte'
import { COVER_ID } from '../geom/sheet'
import type {
  AssocView,
  Family,
  FontFamily,
  GridResult,
  Photo,
  RecentProject,
  ScanProgress,
  Settings,
  State,
  Variety,
  View,
  Zone,
} from './types'

class Store extends Nav {
  photos = $state<Photo[]>([])
  // Текущий разворот, исчезнувший при правке (см. apply).
  private lostId = ''
  loaded = $state(false)
  // Версия программы из wails.json (Go отдаёт в GetState)
  version = $state('')
  stand = $state(false) // стенд QA: Go не открывает браузер и проводник (State.stand)
  // Шрифты текстовых блоков (State.fonts, спека 006); зарегистрированы в document.fonts.
  fonts = $state<FontFamily[]>([])
  // Недавние альбомы (State.recent): список «Файл ▸ Последние» и стартового окна (U2).
  recent = $state<RecentProject[]>([])
  // Связь .salbum (State.assoc, 112): вопрос в стартовом окне и пункт «Файл»; меняют SetAssoc/AssocLater (shell/dialogs/assoc).
  assoc = $state<AssocView>({ can: false, on: false, ask: false })
  // ID фото, которых нет на диске (State.missing и событие missing).
  missing = $state<string[]>([])
  // 117: отдельные файлы, которые найдены, но не читаются (ID → готовая фраза причины); State.unread и событие unread.
  unread = $state<Record<string, string>>({})
  // Имена файлов пропавших фото (ID → имя): в ленте их нет, берём у Go при смене missing (setMissing).
  missingNames = $state<Record<string, string>>({})
  private namesReq = 0
  // Номер открытия альбома (растёт на каждом Open/New): «Скрыть» полосы пропавших живёт до следующего открытия.
  openSeq = $state(0)
  selected = new SvelteSet<string>()
  // Зона последнего нажатия мыши: Ctrl+A и Del действуют по ней (App ставит по pointerdown).
  zone = $state<Zone>('library')
  anchor: string | null = null
  // Выделенное фото — первый ref выделения (places/placesel.svelte.ts); пустое место фотографией не считается.
  get selectedCell(): string | null {
    const r = this.hooks.selectedRefs()[0]
    return r && this.used.has(r) ? r : null
  }
  set selectedCell(id: string | null) {
    this.hooks.select(id ? [id] : [])
  }
  thumbs = $state({ done: 0, total: 0 })
  // Ход чтения папок (событие scan, начальное значение — из GetState).
  scan = $state<ScanProgress>({ active: false, walking: false, found: 0, read: 0 })
  // Тост: состояние и таймер — в Toasts; store отдаёт его как свой toast/notify (так зовут все).
  private readonly toasts = new Toasts()
  get toast(): Toast | null {
    return this.toasts.toast
  }
  set toast(t: Toast | null) {
    this.toasts.toast = t
  }
  filter = $state<'all' | 'unused'>('all')
  // Окно автосборки (027): открыто, пока не null; фото уже выбраны (выделенные или свободные без отклонённых).
  autoDraft = $state<PickResult | null>(null)
  readonly autorun = new AutoRun(this)
  busy = $state(false)
  byId = $derived(new Map(this.photos.map((p) => [p.id, p])))
  // Фото на фоне: в used не входят (не «в альбоме»), но автосборка и фильтр их не берут (freephotos.ts).
  // Состав не менялся — тот же объект, чтобы `free` (3200 фото) не пересчитывался на каждую правку (075-16).
  private bdPrev = new Set<string>()
  backdropUsed = $derived((this.bdPrev = keepSet(this.bdPrev, this.view.backdropUsed ?? [])))
  // Свои отметки фото (спека 004): ID → поля, поставленные в программе (library/ownmarks.svelte.ts).
  // Действующая отметка с учётом XMP — markOf(photo, store.marks[photo.id]).
  get marks(): MarkMap {
    return this.hooks.marks()
  }
  // Фото без скрытых папок (054): без фильтра по папкам — тот же массив.
  ordered = $derived(this.hooks.order(this.photos))
  inFolders = $derived(this.hooks.folders(this.ordered))
  // Фото показанной подборки (083); подборка не показана — тот же массив. Порядок фильтров:
  // папки → подборка → «Не в альбоме» → отметки.
  inBin = $derived(this.hooks.bin(this.inFolders))
  // Не в местах и не на фоне (среди показанных папок и подборки).
  free = $derived(this.hooks.free(this.inBin, this.used, this.backdropUsed))
  // Лента до фильтра по отметкам (длина — «из M» в MarkFilter) и после него (libfilter).
  beforeMarks = $derived(this.filter === 'unused' ? this.free : this.inBin)
  visible = $derived(this.hooks.visible(this.beforeMarks))
  // Выделенные и при этом видимые (с учётом фильтра) фото — в порядке ленты.
  selectedIds = $derived(this.visible.filter((p) => this.selected.has(p.id)).map((p) => p.id))
  projectName = $derived(
    this.view.path
      ? this.view.path
          .split(/[\\/]/)
          .pop()!
          .replace(/\.salbum$/i, '')
      : 'Черновик',
  )

  // События photos/scan/missing, пришедшие, пока ждём снимок состояния (startstate.ts).
  private seen = noEvents()
  private unreadSeen = false // событие unread пришло раньше снимка

  // Альбом заменён целиком: отметки и прошлая автосборка прежнего не нужны.
  private reloaded() {
    this.hooks.reloaded()
    this.autorun.reset()
  }

  async init() {
    this.seen = noEvents()
    this.unreadSeen = false
    EventsOn('library', (photos: Photo[]) => {
      this.seen.photos = true
      this.photos = photos ?? []
      const ids = new Set(this.photos.map((p) => p.id))
      for (const id of this.selected) if (!ids.has(id)) this.selected.delete(id)
    })
    EventsOn('view', (v: View) => this.apply(v))
    EventsOn('scan', (s: ScanProgress) => {
      this.seen.scan = true
      if (s) this.scan = s
    })
    EventsOn('thumbs', (done: number, total: number) => (this.thumbs = { done, total }))
    // 075-5: автосохранение шлёт только {dirty, path}, не вид
    EventsOn('saved', (v: { dirty: boolean; path: string }) => (this.view = { ...this.view, dirty: v.dirty, path: v.path }))
    EventsOn('missing', (ids: string[] | null) => {
      this.seen.missing = true
      this.setMissing(ids ?? [])
    })
    // 117: полная замена, не слияние; после смены добавлений Go шлёт заново
    EventsOn('unread', (m: Record<string, string> | null) => {
      this.unreadSeen = true
      this.unread = m ?? {}
    })
    // 082: полный список шрифтов (загрузка, ×, откат, шрифты из папки собранного альбома) заменяет прежний
    EventsOn('fonts', (list: FontFamily[] | null) => {
      this.fonts = list ?? []
      this.hooks.fontsLoaded(this.fonts, true)
    })
    // Ошибки, которые Go находит сам (перетащенная папка и т.п.)
    EventsOn('notice', (text: string) => this.notify(text, 'error'))
    jobs.listen()
    try {
      const st = (await Api.GetState()) as unknown as State
      this.reloaded()
      this.apply(st.view)
      // Событие, обогнавшее ответ, свежее снимка.
      const cur = { photos: this.photos, scan: this.scan, missing: this.missing }
      const m = mergeSnapshot(cur, { photos: st.photos ?? [], scan: st.scan, missing: st.missing ?? [] }, this.seen)
      this.photos = m.photos
      this.scan = m.scan
      this.setMissing(m.missing)
      if (!this.unreadSeen) this.unread = st.unread ?? {}
      this.version = st.version ?? ''
      this.stand = st.stand === true
      this.fonts = st.fonts ?? []
      prefs.load(st.prefs)
      this.settleStartWindow(st)
      if (st.keymap) this.hooks.keymapLoaded(st.keymap)
      this.recent = st.recent ?? []
      if (st.assoc) this.assoc = st.assoc
      // Не ждём: ошибка шрифта не должна мешать старту, текст пока рисуется запасным.
      this.hooks.fontsLoaded(this.fonts)
      // Ошибка открытия последнего проекта при старте: событие пришло бы до подписки
      if (st.notice) this.notify(st.notice, 'error')
      this.afterOpen(st)
      // 112: файл пришёл до подписки окна на `open:external` — очередь в State, а не событие.
      if (st.pending) this.hooks.external(st.pending)
      if (st.newer) this.hooks.newer(st.newer) // 112: старт на альбоме новее программы
    } finally {
      this.loaded = true
      jobs.endOpening() // альбом открыт: окно хода (073) гаснет, даже если (total, total) потерялось
    }
  }

  // Стартовое окно «Продолжить: …» поверх открытого файла лишнее (112): запуск по двойному щелчку — человек уже выбрал
  // альбом; файл новее программы не открылся, и окно «Альбом из новой версии» важнее (иначе оно лежало бы под
  // стартовым). Первый запуск (уровень ещё не выбран) окно сохраняет: «новее» ложится поверх него
  // (App.svelte: NewerAskDialog после StartDialog).
  private settleStartWindow(st: State) {
    if ((st.fromFile || st.newer) && !prefs.first) prefs.closeStart()
  }

  // Разовое после открытия альбома (запуск и меню): тост info (шрифты из папки альбома) и проверка свежих начертаний окном.
  private afterOpen(st: State) {
    if (st.info) this.notify(st.info, 'info')
    if (st.freshFonts?.length) this.hooks.freshFonts(st.freshFonts)
  }

  // Свежий список последних: «Сохранить как» и первое сохранение кладут альбом в список без события,
  // поэтому его перечитывают при открытии меню «Файл» и стартового окна (Go только читает config.json).
  async refreshRecent() {
    try {
      this.recent = (await Api.Recent()) as unknown as RecentProject[]
    } catch {
      // список — удобство, ошибка не должна мешать работе
    }
  }

  notify(text: string, kind: Toast['kind'] = 'info', action?: Toast['action']) {
    this.toasts.notify(text, kind, action)
  }

  // Все правки идут сюда: Go возвращает новое состояние альбома.
  async run(p: Promise<unknown>): Promise<boolean> {
    try {
      return await this.take((await p) as View)
    } catch (e) {
      this.notify(errText(e), 'error')
      return false
    }
  }

  // Ctrl+Z / Ctrl+Y: обложка (032) могла появиться или исчезнуть. Появилась — открываем её; исчезла —
  // встаём на разворот, где были до неё. Явный переход заодно ставит ряд на место (индексы сдвигаются на lead).
  async runHistory(p: Promise<unknown>): Promise<boolean> {
    const had = !!this.cover
    if (!(await this.run(p))) return false
    if (!had && this.cover) this.goCover()
    else if (had && !this.cover) this.go(this.current)
    return true
  }

  // Принять ответ правки. Дельта не к нашему виду: ждём пересинхрона, иначе вызывающий (rev, current)
  // увидит старый вид (087: два клиента). false — GetView упал, ошибку уже показала цепочка.
  async take(raw: View): Promise<boolean> {
    const w = this.chain.take(raw)
    return w ? await w : true
  }

  // Цепочка видов App (075, viewchain.ts): дубли, дельты, пересинхрон через GetView.
  private readonly chain = new ViewChain({
    current: () => this.view,
    show: (v) => this.show(v),
    fetch: () => Api.GetView() as unknown as Promise<View>,
    fail: (e) => this.notify(errText(e), 'error'),
  })

  // Событие `view` и тесты: без ожидания пересинхрона. Ответ правки — take/run.
  apply(raw: View) {
    void this.chain.take(raw)
  }

  private show(v: View) {
    const next = { ...v, folders: v.folders ?? [], loose: v.loose ?? [], spreads: v.spreads ?? [] }
    this.view = import.meta.env.DEV ? deepFreeze(next) : next
    this.settle()
  }

  // После смены вида: тост, обложка, текущий разворот, хук верха.
  private settle() {
    const v = this.view
    // «Вернуть» отменяет последнюю правку: после любой другой смены ревизии он уже не про то.
    if (this.toast?.rev !== undefined && v.rev !== this.toast.rev) this.toast = null
    const sp = v.spreads
    // Обложку удалили (или Undo её убрал) — возвращаемся на разворот; остались одни обложки — стоим на ней.
    if (this.onCover && !this.cover) this.onCover = false
    if (!this.onCover && sp.length === 0 && this.cover) this.onCover = true
    // Удалённый текущий запоминается: если Undo его вернёт, текущим станет он (pickCurrent).
    const pick = pickCurrent(
      sp.map((s) => s.id),
      this.currentId,
      this.current,
      this.lostId,
    )
    this.current = pick.index
    this.currentId = pick.id
    this.lostId = pick.lost
    if (pick.back >= 0 && pick.back !== this.current) this.go(pick.back)
    this.hooks.applied(this.view)
  }

  // Перейти на разворот, которого не было среди ids (только что созданный).
  goNew(before: Set<string>, fallback: number) {
    const i = this.view.spreads.findIndex((s) => !before.has(s.id))
    this.go(i >= 0 ? i : fallback)
  }

  // ---------- выделение в ленте фото ----------

  click(id: string, e: MouseEvent) {
    if (e.shiftKey && this.anchor) {
      const list = this.visible
      const a = list.findIndex((p) => p.id === this.anchor)
      const b = list.findIndex((p) => p.id === id)
      if (a >= 0 && b >= 0) {
        if (!toggleKey(e)) this.selected.clear()
        for (let i = Math.min(a, b); i <= Math.max(a, b); i++) this.selected.add(list[i].id)
        return
      }
    }
    if (toggleKey(e)) {
      if (this.selected.has(id)) this.selected.delete(id)
      else this.selected.add(id)
    } else {
      this.selected.clear()
      this.selected.add(id)
    }
    this.anchor = id
  }

  // Что тащим из ленты: всё выделенное, если взяли за выделенное фото.
  dragIds(id: string): string[] {
    if (this.selected.has(id)) return this.selectedIds
    this.selected.clear()
    this.selected.add(id)
    this.anchor = id
    return [id]
  }

  // ---------- действия ----------

  // Добавление фото из ленты: после успеха снимает выделение и сообщает
  // о фото, которые были в других листах (into — целевой лист, null — новый). Источник ищется среди всех
  // листов ряда, в том числе в обложке (032).
  // 098: фото, уже стоящее в альбоме, сперва проходит ворота (окно копии); op получает ID для Go.
  async fromStrip(ids: string[], into: string | null, op: (ids: string[]) => Promise<unknown>): Promise<boolean> {
    const sent = await this.hooks.gate(ids)
    if (!sent) return false
    const moved = this.movedFrom(sent, into)
    if (!(await this.run(op(sent)))) return false
    this.stripDone(moved)
    return true
  }

  // «До» правки: из каких листов (кроме into) пришли фото; разбор — strip.ts.
  movedFrom(ids: string[], into: string | null): Moved[] {
    return movedFrom(this.sheets, ids, into)
  }

  // «После» правки: снять выделение в ленте и сказать, откуда перенесены фото.
  stripDone(moved: Moved[]) {
    this.selected.clear()
    const text = movedText(moved, this.view)
    if (text) this.notify(text)
  }

  async addToCurrent(ids: string[]) {
    if (!ids.length) return
    const s = this.spread
    if (!s) return this.newSpread(this.view.spreads.length, ids)
    // У обложки нет ленты фото: их бросают на зону или в место.
    if (s.id === COVER_ID) return this.notify('На обложку фото бросают на зону или в место')
    // Ручной разворот: места, плитка и вопрос (042) — верх подключает хук; без него прежний путь.
    if (s.grid && (await this.hooks.dropGrid(s.id, ids))) return
    await this.fromStrip(ids, s.id, (sent) => Api.AddPhotos(s.id, sent, -1))
  }

  async newSpread(at: number, ids: string[], strip = true) {
    const before = new Set(this.view.spreads.map((s) => s.id))
    const op = (sent: string[]) => Api.NewSpread(at, sent)
    const ok = strip ? await this.fromStrip(ids, null, op) : await this.run(op(ids))
    if (ok) this.goNew(before, at)
  }

  // Пустой разворот щелчком по «+ Новый разворот» (спека 013). Пока запрос идёт, повтор игнорируется.
  private creating = false
  async addEmptySpread(at: number) {
    if (this.creating) return
    this.creating = true
    try {
      await this.newSpread(at, [], false)
    } finally {
      this.creating = false
    }
  }

  // Тосты «Вернуть» / «Убрать имена» — toasts.svelte.ts.
  undoToast(text: string, back?: string) {
    undoToast(this, text, back)
  }

  stepToast(lead: string, notes: string[], named: string[]) {
    stepToast(this, lead, notes, named)
  }

  // «Заполнить пустые места»: выделенные фото (иначе все не в альбоме) — в пустые места, начиная с текущего разворота.
  async fillPlaces() {
    const { ids, skipped } = this.hooks.pick(false)
    if (ids.length === 0) return
    try {
      const r = (await Api.FillPlaces(ids, this.spread?.id ?? '')) as unknown as GridResult
      this.apply(r.view)
      this.selected.clear()
      const lead =
        r.left > 0 || skipped > 0 ? withSkipped(r.left > 0 ? `Не хватило мест: ${r.left} фото остались в ленте` : '', skipped) : ''
      this.stepToast(lead, [], r.named ?? [])
    } catch (e) {
      this.notify(errText(e), 'error')
    }
  }

  // «Автосборка» и «Пересобрать…» (078) — base/auto/autorun.svelte.ts; здесь входы для окон, меню и тостов.
  openAuto() {
    this.autorun.open()
  }

  openAgain() {
    this.autorun.openAgain()
  }

  get lastAuto() {
    return this.autorun.last
  }

  // 080: окно пересборки ждёт ответа; совпавшие варианты; ссылка «Собрать свободные» / «Пересобрать прошлую»
  get autoWaiting() {
    return this.autorun.waiting
  }

  get autoSame() {
    return this.autorun.same
  }

  swapAuto() {
    this.autorun.swap()
  }

  autoBuild(...a: Parameters<AutoRun['build']>): Promise<boolean> {
    return this.autorun.build(...a)
  }

  // ---------- листы альбома ----------

  // Обложка, раскладки, удаление, разрез, склейка и имя — spreadops.svelte.ts; здесь входы с прежними именами.
  private readonly ops = new SpreadOps(this)

  get delQueue(): Promise<unknown> {
    return this.ops.delQueue
  }
  createCover = (photoID: string) => this.ops.createCover(photoID)
  deleteCover = () => this.ops.deleteCover()
  varyLayouts = (ids: readonly string[], variety: Variety, families: readonly Family[] = []) => this.ops.varyLayouts(ids, variety, families)
  deleteSpread = (id?: string) => this.ops.deleteSpread(id)
  deleteSpreads = (ids: readonly string[]) => this.ops.deleteSpreads(ids)
  split = (id: string, at: number, keep?: boolean) => this.ops.split(id, at, keep)
  merge = (id: string, keep?: boolean) => this.ops.merge(id, keep)
  renameSpread = (id: string, name: string) => this.ops.rename(id, name)

  async setSettings(s: Settings) {
    await this.run(Api.SetSettings(s))
  }

  // Список пропавших и их имена. Номер запроса отсекает запоздавший ответ.
  private setMissing(ids: string[]) {
    this.missing = ids
    const my = ++this.namesReq
    if (ids.length === 0) {
      this.missingNames = {}
      return
    }
    Api.MissingNames()
      .then((n) => {
        if (my === this.namesReq) this.missingNames = n ?? {}
      })
      .catch(() => {
        if (my === this.namesReq) this.missingNames = {}
      })
  }

  // true — состояние заменено (или диалог отменён, ничего не менялось); false — ошибка (тост уже показан).
  // Отмену диалога «Открыть» Go помечает явно (State.cancelled): событие view приходит раньше ответа,
  // так что сравнением path/rev её не отличить от удачного открытия.
  async replaceState(p: Promise<unknown>): Promise<boolean> {
    // События, пришедшие до ответа (rescan после Open/New), свежее снимка.
    this.seen = noEvents()
    this.unreadSeen = false
    try {
      const st = (await p) as State
      // 112: файл новее программы — Go шлёт окно вместе с cancelled; смотрим до раннего возврата.
      if (st.newer) this.hooks.newer(st.newer)
      if (st.cancelled === true) return true
      this.reloaded() // другой альбом: отметки перечитать, даже если marksRev тот же
      this.apply(st.view)
      const cur = { photos: this.photos, scan: this.scan, missing: this.missing }
      this.photos = mergeSnapshot(cur, { photos: st.photos ?? [] }, this.seen).photos
      if (st.recent) this.recent = st.recent
      if (!this.unreadSeen) this.unread = st.unread ?? {} // другой альбом — чужие причины не остаются
      this.afterOpen(st)
      this.selected.clear()
      this.hooks.spreadsCleared()
      this.openSeq++
      this.go(0)
      return true
    } catch (e) {
      this.notify(errText(e), 'error')
      return false
    } finally {
      jobs.endOpening() // смена альбома (в том числе отмена диалога и ошибка): окно хода (073) гаснет
    }
  }
}

export const store = new Store()
jobs.attach(store)
export { Api }
