import * as Api from '../../../wailsjs/go/app/App'
import { EventsOn } from '../../../wailsjs/runtime/runtime'
import { SvelteSet } from 'svelte/reactivity'
import { toggleKey } from './platform'
import { errText, withSkipped } from '../words/errtext'
import { Nav } from './nav.svelte'
import { jobs } from './jobs.svelte'
import { AutoRun } from './autorun.svelte'
import { Toasts, stepToast, undoToast, type Toast } from './toasts.svelte'
import type { MarkMap, PickResult } from './hooks'
import { backdropLost, pickCurrent } from './undo'
import { deepFreeze } from './viewdelta'
import { ViewChain } from './viewchain'
import { keepSet } from './usedindex'
import { mergeSnapshot, noEvents } from './startstate'
import { movedFrom, movedText, type Moved } from './strip'
import { prefs } from '../prefs/prefs.svelte'
import { COVER_ID } from '../geom/sheet'
import type { Family, FontFamily, GridResult, Photo, RecentProject, ScanProgress, Settings, State, Variety, View, Zone } from './types'

class Store extends Nav {
  photos = $state<Photo[]>([])
  // Текущий разворот, исчезнувший при правке (см. apply).
  private lostId = ''
  loaded = $state(false)
  // Версия программы из wails.json (Go отдаёт в GetState)
  version = $state('')
  // Шрифты текстовых блоков (State.fonts, спека 006); зарегистрированы в document.fonts.
  fonts = $state<FontFamily[]>([])
  // Недавние альбомы (State.recent): список «Файл ▸ Последние» и стартового окна (U2).
  recent = $state<RecentProject[]>([])
  // ID фото, которых нет на диске (State.missing и событие missing).
  missing = $state<string[]>([])
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
  private readonly autorun = new AutoRun(this)
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

  // Альбом заменён целиком: отметки и прошлая автосборка прежнего не нужны.
  private reloaded() {
    this.hooks.reloaded()
    this.autorun.reset()
  }

  async init() {
    this.seen = noEvents()
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
      this.version = st.version ?? ''
      this.fonts = st.fonts ?? []
      prefs.load(st.prefs)
      if (st.keymap) this.hooks.keymapLoaded(st.keymap)
      this.recent = st.recent ?? []
      // Не ждём: ошибка шрифта не должна мешать старту, текст пока рисуется запасным.
      this.hooks.fontsLoaded(this.fonts)
      // Ошибка открытия последнего проекта при старте: событие пришло бы до подписки
      if (st.notice) this.notify(st.notice, 'error')
      this.afterOpen(st)
    } finally {
      this.loaded = true
      jobs.endOpening() // альбом открыт: окно хода (073) гаснет, даже если (total, total) потерялось
    }
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
    const next = { ...v, folders: v.folders ?? [], spreads: v.spreads ?? [] }
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
  async fromStrip(ids: string[], into: string | null, op: () => Promise<unknown>): Promise<boolean> {
    const moved = this.movedFrom(ids, into)
    if (!(await this.run(op()))) return false
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
    await this.fromStrip(ids, s.id, () => Api.AddPhotos(s.id, ids, -1))
  }

  async newSpread(at: number, ids: string[], strip = true) {
    const before = new Set(this.view.spreads.map((s) => s.id))
    const op = () => Api.NewSpread(at, ids)
    const ok = strip ? await this.fromStrip(ids, null, op) : await this.run(op())
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

  // Обложка (032): бросок фото на плитку создаёт её с этим фото на лицевой (photoID '' — пустая) — один шаг отмены.
  // Пока запрос идёт, повторный щелчок по «+ Обложка» ничего не делает (иначе второй вызов — холостая правка с ошибкой).
  private coverMaking = false
  async createCover(photoID: string): Promise<boolean> {
    if (this.cover || this.view.settings?.sheet || this.busy || this.coverMaking) return false
    this.coverMaking = true
    try {
      if (!(await this.run(Api.CreateCover('', photoID)))) return false
    } finally {
      this.coverMaking = false
    }
    this.goCover()
    return true
  }

  // «Удалить обложку»: тост с «Вернуть», как у разворотов. Холостая правка (rev тот же) тоста не даёт.
  async deleteCover() {
    const before = this.view.rev
    if (!(await this.run(Api.DeleteCover())) || this.view.rev === before) return
    this.undoToast('Обложка удалена', COVER_ID)
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

  // «Автосборка» и «Пересобрать…» (078) — autorun.svelte.ts; здесь входы для окон, меню и тостов.
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

  // «Разнообразить раскладки» (041): Go меняет раскладки выбранных разворотов с учётом соседей, один шаг отмены.
  // families — тот же фильтр, что у автосборки (065). Вид без изменений (rev тот же) — тост вместо немого ничего; иначе — тост с «Вернуть».
  async varyLayouts(ids: readonly string[], variety: Variety, families: readonly Family[] = []): Promise<boolean> {
    if (ids.length === 0 || this.busy) return false
    const was = new Map(this.view.spreads.map((s) => [s.id, s.layout]))
    const before = this.view.rev
    if (!(await this.run(Api.VaryLayouts([...ids], variety, [...families])))) return false
    if (this.view.rev === before) {
      this.notify('Менять нечего: раскладки уже разные или других подходящих нет')
      return false
    }
    const n = this.view.spreads.filter((s) => was.has(s.id) && was.get(s.id) !== s.layout).length
    this.undoToast(`Раскладки изменены: ${n}`)
    return true
  }

  // ---------- раскладки ----------

  // Щелчки быстрее ответа Go: каждое удаление ждёт предыдущее, а «текущий» берётся уже после него —
  // иначе все щелчки уходят с одним id и удаляется один разворот. run ловит ошибки, цепочка не рвётся.
  delQueue: Promise<unknown> = Promise.resolve()

  // id не задан — текущий разворот на момент выполнения; меню ленты передаёт разворот, по которому щёлкнули.
  deleteSpread(id?: string): Promise<void> {
    const next = this.delQueue.then(() => this.deleteNow(id))
    this.delQueue = next.catch(() => {})
    return next
  }

  // Несколько групп ленты (спека 019 F): один запрос, один шаг Ctrl+Z. Очередь общая с deleteSpread.
  deleteSpreads(ids: readonly string[]): Promise<void> {
    const next = this.delQueue.then(() => this.deleteManyNow(ids))
    this.delQueue = next.catch(() => {})
    return next
  }

  private async deleteManyNow(ids: readonly string[]) {
    const gone = this.view.spreads.map((s) => s.id).filter((id) => ids.includes(id))
    if (gone.length === 0) return
    const before = this.view.rev
    if (!(await this.run(Api.DeleteSpreads(gone)))) return
    this.hooks.spreadsCleared()
    const left = new Set(this.view.spreads.map((s) => s.id))
    const n = gone.filter((id) => !left.has(id)).length
    // Холостая правка (rev тот же): тоста нет, иначе «Вернуть» отменило бы чужую правку.
    if (n > 0 && this.view.rev !== before) this.undoToast(`Удалено разворотов: ${n}`, gone[0])
  }

  private async deleteNow(id?: string) {
    // На обложке «Удалить разворот» — это удаление обложки (Go отвечает отказом на разворот с её id).
    if (id === COVER_ID || (!id && this.onCover)) return this.deleteCover()
    const s = id ? this.view.spreads.find((x) => x.id === id) : this.spread
    if (!s) return
    const before = this.view.rev
    // Холостая правка (rev тот же): тоста нет, иначе «Вернуть» отменило бы чужую правку.
    if (!(await this.run(Api.DeleteSpread(s.id))) || this.view.rev === before) return
    this.undoToast('Разворот удалён', s.id)
  }

  // ---------- разрез и склейка (лента альбома, спека 003) ----------

  // Делит разворот: фото [0:at] остаются, остальные — в новый разворот сразу после него.
  async split(id: string, at: number) {
    const before = this.view.rev
    if (!(await this.run(Api.SplitSpread(id, at)))) return
    if (this.view.rev === before) return
    this.goId(id)
    this.undoToast('Разворот разрезан', id)
  }

  // Клеит разворот со следующим; текущим становится объединённый (второй исчез).
  async merge(id: string) {
    const before = this.view.rev
    const i = this.view.spreads.findIndex((s) => s.id === id)
    // Фон второго разворота при склейке снимается, если у первого свой: даём «Вернуть».
    const lost = backdropLost(this.view.spreads[i], this.view.spreads[i + 1])
    if (!(await this.run(Api.MergeSpreads(id))) || this.view.rev === before) return
    this.goId(id)
    if (lost) this.undoToast('Фон второго разворота снят')
  }

  // Название разворота; пустое снимает. Правка — шаг undo, то же название — холостая в Go.
  renameSpread(id: string, name: string) {
    return this.run(Api.SetSpreadName(id, name))
  }

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
    try {
      const st = (await p) as State
      if (st.cancelled === true) return true
      this.reloaded() // другой альбом: отметки перечитать, даже если marksRev тот же
      this.apply(st.view)
      const cur = { photos: this.photos, scan: this.scan, missing: this.missing }
      this.photos = mergeSnapshot(cur, { photos: st.photos ?? [] }, this.seen).photos
      if (st.recent) this.recent = st.recent
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
