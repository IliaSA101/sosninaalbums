import * as Api from '../../../wailsjs/go/app/App'
import { EventsOn } from '../../../wailsjs/runtime/runtime'
import { SvelteSet } from 'svelte/reactivity'
import { capFirst, errText, exportDoneText, withSkipped } from '../words/errtext'
import { Nav } from './nav.svelte'
import type { MarkMap, PickResult } from './hooks'
import { backdropLost, pickCurrent } from './undo'
import { mergeSnapshot, noEvents } from './startstate'
import { movedFrom, movedText, type Moved } from './strip'
import { prefs } from '../prefs/prefs.svelte'
import { COVER_ID, unitOf } from '../geom/sheet'
import { model } from '../../../wailsjs/go/models'
import type {
  AutoRange,
  ExportOpts,
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

// rev — ревизия альбома, на которую рассчитан тост с «Вернуть»: другая ревизия (Ctrl+Z, любая правка) гасит тост.
type Toast = { text: string; kind: 'info' | 'error'; action?: { label: string; run: () => void }; rev?: number }

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
  exporting = $state<{ done: number; total: number } | null>(null)
  toast = $state<Toast | null>(null)
  filter = $state<'all' | 'unused'>('all')
  // Окно автосборки (027): открыто, пока не null; фото уже выбраны (выделенные или свободные без отклонённых).
  autoDraft = $state<PickResult | null>(null)
  busy = $state(false)
  // Окно «Экспорт» (009 U8) открыто.
  exportOpen = $state(false)
  // Памятка после выгрузки для Фотосферы Pro (047): не null — открыто окно «Готово для Фотосферы» (shell/export/FsferaDone).
  exportMemo = $state<string[] | null>(null)
  exportNote = $state('') // сводка Go о пересчёте цвета к той же памятке
  exportFolder = $state('') // имя папки книги из плана в момент запуска выгрузки (окно «Готово»)
  byId = $derived(new Map(this.photos.map((p) => [p.id, p])))
  // Фото на фоне: в used не входят (не «в альбоме»), но автосборка и фильтр их не берут (freephotos.ts).
  backdropUsed = $derived(new Set(this.view.backdropUsed ?? []))
  // Свои отметки фото (спека 004): ID → поля, поставленные в программе (library/ownmarks.svelte.ts).
  // Действующая отметка с учётом XMP — markOf(photo, store.marks[photo.id]).
  get marks(): MarkMap {
    return this.hooks.marks()
  }
  // Фото без скрытых папок (054): без фильтра по папкам — тот же массив.
  inFolders = $derived(this.hooks.folders(this.photos))
  // Не в местах и не на фоне (среди показанных папок).
  free = $derived(this.hooks.free(this.inFolders, this.used, this.backdropUsed))
  // Лента до фильтра по отметкам (длина — «из M» в MarkFilter) и после него (libfilter).
  beforeMarks = $derived(this.filter === 'unused' ? this.free : this.inFolders)
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

  private toastTimer = 0
  // События photos/scan/missing, пришедшие, пока ждём снимок состояния (startstate.ts).
  private seen = noEvents()

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
    EventsOn('saved', (v: View) => (this.view = { ...this.view, dirty: v.dirty, path: v.path }))
    EventsOn('missing', (ids: string[] | null) => {
      this.seen.missing = true
      this.setMissing(ids ?? [])
    })
    EventsOn('export:progress', (done: number, total: number) => (this.exporting = { done, total }))
    // Ошибки, которые Go находит сам (перетащенная папка и т.п.)
    EventsOn('notice', (text: string) => this.notify(text, 'error'))
    // note — сводка Go о пересчёте цвета (может быть пустой или отсутствовать у старого Go).
    // memo (047) — строки памятки для Фотосферы: вместо тоста открывается окно «Готово», тост нужен только при ошибке.
    EventsOn('export:done', (_dir: string, err: string, note?: string, memo?: string[] | null) => {
      const total = this.exporting?.total
      this.exporting = null
      if (err) this.notify('Экспорт не удался: ' + errText(err), 'error')
      else if (memo?.length) {
        this.exportMemo = memo
        this.exportNote = note ?? ''
      } else
        this.notify(exportDoneText(total, note), 'info', {
          label: 'Открыть папку',
          run: () => Api.ShowInExplorer(),
        })
    })
    try {
      const st = (await Api.GetState()) as unknown as State
      this.hooks.reloaded()
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
      this.recent = st.recent ?? []
      // Не ждём: ошибка шрифта не должна мешать старту, текст пока рисуется запасным.
      this.hooks.fontsLoaded(this.fonts)
      // Ошибка открытия последнего проекта при старте: событие пришло бы до подписки
      if (st.notice) this.notify(st.notice, 'error')
    } finally {
      this.loaded = true
    }
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
    // Ошибки Go пишутся со строчной буквы.
    this.toast = { text: kind === 'error' ? capFirst(text) : text, kind, action }
    clearTimeout(this.toastTimer)
    this.toastTimer = window.setTimeout(() => (this.toast = null), action ? 8000 : 4000)
  }

  // Все правки идут сюда: Go возвращает новое состояние альбома.
  async run(p: Promise<unknown>): Promise<boolean> {
    try {
      this.apply((await p) as View)
      return true
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

  apply(v: View) {
    if (!v) return
    this.view = { ...v, folders: v.folders ?? [], spreads: v.spreads ?? [] }
    // «Вернуть» отменяет последнюю правку: после любой другой смены ревизии он уже не про то.
    if (this.toast?.rev !== undefined && v.rev !== this.toast.rev) this.toast = null
    const sp = this.view.spreads
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
  private goNew(before: Set<string>, fallback: number) {
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
        if (!e.ctrlKey && !e.metaKey) this.selected.clear()
        for (let i = Math.min(a, b); i <= Math.max(a, b); i++) this.selected.add(list[i].id)
        return
      }
    }
    if (e.ctrlKey || e.metaKey) {
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

  // Тост с кнопкой «Вернуть» = Undo, как у разреза: безопасно, только если после правки ничего не менялось.
  // back — разворот, на который перейти после возврата.
  undoToast(text: string, back?: string) {
    const rev = this.view.rev
    this.notify(text, 'info', {
      label: 'Вернуть',
      run: async () => {
        // Идущее следом удаление ещё может менять альбом: сначала дожидаемся очереди, потом сверяем ревизию.
        await this.delQueue
        if (this.view.rev !== rev) return this.notify('Уже были другие изменения — используйте Ctrl+Z')
        if ((await this.run(Api.Undo())) && back) this.goId(back)
      },
    })
    if (this.toast) this.toast.rev = rev
  }

  // Итоговый тост правки, добавившей фото (042, 060). notes — заметки с «Вернуть» (отменяют весь шаг);
  // named — фото, получившие имя из названия файла: с заметками имена идут в тот же тост, без них у тоста
  // своя кнопка «Убрать имена» (снимает только имена, бросок остаётся). lead — прежний простой тост.
  stepToast(lead: string, notes: string[], named: string[]) {
    const names = named.length > 0 ? `Имена взяты из названий файлов: ${named.length}` : ''
    if (notes.length > 0) return this.undoToast([lead, ...notes, names].filter(Boolean).join('. '))
    if (!names) return lead ? this.notify(lead) : undefined
    const rev = this.view.rev
    this.notify([lead, names].filter(Boolean).join('. '), 'info', {
      label: 'Убрать имена',
      run: async () => {
        // Как у «Вернуть»: после чужой правки кнопка уже не про этот шаг.
        if (this.view.rev !== rev) return this.notify('Уже были другие изменения — имена не тронуты')
        await this.run(Api.ForgetFileNames(named))
      },
    })
    if (this.toast) this.toast.rev = rev
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

  // «Автосборка»: выбрать фото и открыть окно; пусто — тост с причиной уже показан.
  openAuto() {
    if (this.busy) return
    const p = this.hooks.pick(true)
    if (p.ids.length > 0) this.autoDraft = p
  }

  // «Собрать» в окне: ровно `spreads` разворотов, фото на каждом — по `range` («Авто» или «от … до …», серии; 051),
  // разнообразие раскладок — variety (041).
  async autoBuild(spreads: number, range: AutoRange, variety: Variety = 1): Promise<boolean> {
    // Выделенные — как есть: Go переносит уже стоящие в альбоме фото в новые развороты.
    const d = this.autoDraft
    this.autoDraft = null
    if (!d || d.ids.length === 0) return false
    const at = this.view.spreads.length
    const before = new Set(this.view.spreads.map((s) => s.id))
    this.busy = true
    let ok: boolean
    try {
      const opts = model.AutoOpts.createFrom({ spreads, ...range, variety })
      ok = await this.run(Api.AutoBuild([...d.ids], at, opts))
    } finally {
      this.busy = false
    }
    if (!ok) return false
    this.selected.clear()
    this.goNew(before, at)
    this.notify(
      withSkipped(
        `Собрано ${unitOf(this.view.settings).plural[2]}: ${this.view.spreads.filter((s) => !before.has(s.id)).length}`,
        d.skipped,
      ),
    )
    return true
  }

  // «Разнообразить раскладки» (041): Go меняет раскладки выбранных разворотов с учётом соседей, один шаг отмены.
  // Вид без изменений (rev тот же) — тост вместо немого ничего; иначе — тост с «Вернуть».
  async varyLayouts(ids: readonly string[], variety: Variety): Promise<boolean> {
    if (ids.length === 0 || this.busy) return false
    const was = new Map(this.view.spreads.map((s) => [s.id, s.layout]))
    const before = this.view.rev
    if (!(await this.run(Api.VaryLayouts([...ids], variety)))) return false
    if (this.view.rev === before) {
      this.notify('Раскладки уже разные — менять нечего')
      return false
    }
    const n = this.view.spreads.filter((s) => was.has(s.id) && was.get(s.id) !== s.layout).length
    this.undoToast(`Раскладки изменены: ${n}`)
    return true
  }

  // ---------- раскладки ----------

  // Щелчки быстрее ответа Go: каждое удаление ждёт предыдущее, а «текущий» берётся уже после него —
  // иначе все щелчки уходят с одним id и удаляется один разворот. run ловит ошибки, цепочка не рвётся.
  private delQueue: Promise<unknown> = Promise.resolve()

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

  // «Экспорт» открывает окно настроек (ExportDialog); сам запуск — runExport.
  exportAll() {
    if (this.exporting || this.sheets.length === 0) return
    this.exportOpen = true
  }

  // true — экспорт пошёл; false — отказ (диалог папки закрыт, вопрос «Нет») или ошибка: окно остаётся открытым.
  async runExport(opt: ExportOpts): Promise<boolean> {
    try {
      const started = await Api.Export(opt)
      // total неизвестен (диапазон, «по страницам»): считает Go, первое export:progress его пришлёт.
      if (started) this.exporting = { done: 0, total: 0 }
      return started
    } catch (e) {
      this.notify(errText(e), 'error')
      return false
    }
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
      this.hooks.reloaded() // другой альбом: отметки перечитать, даже если marksRev тот же
      this.apply(st.view)
      const cur = { photos: this.photos, scan: this.scan, missing: this.missing }
      this.photos = mergeSnapshot(cur, { photos: st.photos ?? [] }, this.seen).photos
      if (st.recent) this.recent = st.recent
      this.selected.clear()
      this.hooks.spreadsCleared()
      this.openSeq++
      this.go(0)
      return true
    } catch (e) {
      this.notify(errText(e), 'error')
      return false
    }
  }
}

export const store = new Store()
export { Api }
