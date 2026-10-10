import * as Api from '../../../wailsjs/go/app/App'
import { EventsOn } from '../../../wailsjs/runtime/runtime'
import { SvelteSet } from 'svelte/reactivity'
import { capFirst, errText, withSkipped } from './errtext'
import { noHooks, type MarkMap, type StoreHooks } from './hooks'
import { backdropLost, pickCurrent } from './undo'
import { mergeSnapshot, noEvents } from './startstate'
import { prefs } from '../prefs/prefs.svelte'
import type {
  ExportOpts,
  FontFamily,
  GridResult,
  Photo,
  RecentProject,
  ScanProgress,
  Settings,
  SpreadView,
  State,
  View,
  Zone,
} from './types'

const emptyView: View = {
  spreads: [],
  settings: { pageW: 300, pageH: 300, margin: 12, gap: 3, layflat: true, dpi: 300, bg: '' },
  folders: [],
  path: '',
  dirty: false,
  canUndo: false,
  canRedo: false,
  rev: 0,
  maxSpreadPhotos: 7,
  safe: [],
  minDPI: 200,
  lowDPI: 0,
  emptyPlaces: 0,
  textMaxW: 284,
  backdropUsed: [],
  marksRev: 0,
}

// rev — ревизия альбома, на которую рассчитан тост с «Вернуть»: другая ревизия (Ctrl+Z, любая правка) гасит тост.
type Toast = { text: string; kind: 'info' | 'error'; action?: { label: string; run: () => void }; rev?: number }

class Store {
  view = $state<View>(emptyView)
  photos = $state<Photo[]>([])
  current = $state(0)
  // id текущего разворота: индекс «следует» за ним при перестановках/удалениях.
  currentId = ''
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
  selected = new SvelteSet<string>()
  // Зона последнего нажатия мыши: Ctrl+A и Del действуют по ней (App ставит по pointerdown).
  zone = $state<Zone>('library')
  anchor: string | null = null
  // Подключённый верх (hooks.ts): ставит shell/boot/wire.ts. Реактивно — ленты пересчитаются после подключения.
  hooks = $state.raw<StoreHooks>(noHooks)
  plug(h: StoreHooks) {
    this.hooks = h
  }
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
  thumbSize = $state(110)
  perSpread = $state(4)
  busy = $state(false)
  // Когда был переход: ряд разворотов (spread/spreadrow.svelte.ts) едет только сразу после go(),
  // а не при загрузке или удалении. navSeq растёт на каждый go() — ряд видит и переход на тот же разворот
  // (со слота «новый» назад).
  navAt = 0
  navSeq = $state(0)

  // Над группой какой страницы курсор в полосе над разворотом.
  hoverPage = $state<'left' | 'right' | null>(null)

  // Окно «Экспорт» (009 U8) открыто.
  exportOpen = $state(false)

  used = $derived(new Set(this.view.spreads.flatMap((s) => s.photos)))
  spread = $derived<SpreadView | undefined>(this.view.spreads[this.current])
  byId = $derived(new Map(this.photos.map((p) => [p.id, p])))
  // Фото на фоне: в used не входят (не «в альбоме»), но автосборка и фильтр их не берут (freephotos.ts).
  backdropUsed = $derived(new Set(this.view.backdropUsed ?? []))
  // Свои отметки фото (спека 004): ID → поля, поставленные в программе (library/ownmarks.svelte.ts).
  // Действующая отметка с учётом XMP — markOf(photo, store.marks[photo.id]).
  get marks(): MarkMap {
    return this.hooks.marks()
  }
  // Не в местах и не на фоне.
  free = $derived(this.hooks.free(this.photos, this.used, this.backdropUsed))
  // Лента до фильтра по отметкам (длина — «из M» в MarkFilter) и после него (libfilter).
  beforeMarks = $derived(this.filter === 'unused' ? this.free : this.photos)
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
      this.missing = ids ?? []
    })
    EventsOn('export:progress', (done: number, total: number) => (this.exporting = { done, total }))
    // Ошибки, которые Go находит сам (перетащенная папка и т.п.)
    EventsOn('notice', (text: string) => this.notify(text, 'error'))
    // note — сводка Go о пересчёте цвета (может быть пустой или отсутствовать у старого Go).
    EventsOn('export:done', (_dir: string, err: string, note?: string) => {
      this.exporting = null
      if (err) this.notify('Экспорт не удался: ' + errText(err), 'error')
      else
        this.notify(note ? `Развороты сохранены. ${note}` : 'Развороты сохранены', 'info', {
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
      this.missing = m.missing
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

  apply(v: View) {
    if (!v) return
    this.view = { ...v, folders: v.folders ?? [], spreads: v.spreads ?? [] }
    // «Вернуть» отменяет последнюю правку: после любой другой смены ревизии он уже не про то.
    if (this.toast?.rev !== undefined && v.rev !== this.toast.rev) this.toast = null
    const sp = this.view.spreads
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

  go(i: number) {
    const next = Math.max(0, Math.min(i, this.view.spreads.length - 1))
    this.navAt = performance.now()
    this.navSeq++
    this.current = next
    this.currentId = this.view.spreads[this.current]?.id ?? ''
    this.hoverPage = null
    this.hooks.went()
  }

  goId(id: string) {
    const i = this.view.spreads.findIndex((s) => s.id === id)
    if (i >= 0) this.go(i)
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
  // о фото, которые были в других разворотах (into — целевой разворот, null — новый).
  async fromStrip(ids: string[], into: string | null, op: () => Promise<unknown>): Promise<boolean> {
    const moved = ids.map((id) => this.view.spreads.findIndex((s) => s.id !== into && s.photos.includes(id))).filter((i) => i >= 0)
    if (!(await this.run(op()))) return false
    this.selected.clear()
    if (moved.length === 1) this.notify(`Фото перенесено из разворота ${moved[0] + 1}`)
    else if (moved.length > 1) this.notify(`Перенесено фото из других разворотов: ${moved.length}`)
    return true
  }

  async addToCurrent(ids: string[]) {
    if (!ids.length) return
    const s = this.spread
    if (!s) return this.newSpread(this.view.spreads.length, ids)
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

  // «Заполнить пустые места»: выделенные фото (иначе все не в альбоме) — в пустые места, начиная с текущего разворота.
  async fillPlaces() {
    const { ids, skipped } = this.hooks.pick(false)
    if (ids.length === 0) return
    try {
      const r = (await Api.FillPlaces(ids, this.spread?.id ?? '')) as unknown as GridResult
      this.apply(r.view)
      this.selected.clear()
      if (r.left > 0 || skipped > 0) this.notify(withSkipped(r.left > 0 ? `Не хватило мест: ${r.left} фото остались в ленте` : '', skipped))
    } catch (e) {
      this.notify(errText(e), 'error')
    }
  }

  async autoBuild() {
    // Выделенные — как есть: Go переносит уже стоящие в альбоме фото в новые развороты.
    const { ids, skipped } = this.hooks.pick(true)
    if (ids.length === 0) return
    const at = this.view.spreads.length
    const before = new Set(this.view.spreads.map((s) => s.id))
    this.busy = true
    let ok: boolean
    try {
      ok = await this.run(Api.AutoBuild(ids, at, this.perSpread))
    } finally {
      this.busy = false
    }
    if (!ok) return
    this.selected.clear()
    this.goNew(before, at)
    this.notify(withSkipped(`Собрано разворотов: ${this.view.spreads.filter((s) => !before.has(s.id)).length}`, skipped))
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
    if (this.exporting || this.view.spreads.length === 0) return
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

  // true — состояние заменено; false — ошибка (тост уже показан).
  async replaceState(p: Promise<unknown>): Promise<boolean> {
    // События, пришедшие до ответа (rescan после Open/New), свежее снимка.
    this.seen = noEvents()
    try {
      const st = (await p) as State
      this.hooks.reloaded() // другой альбом: отметки перечитать, даже если marksRev тот же
      this.apply(st.view)
      const cur = { photos: this.photos, scan: this.scan, missing: this.missing }
      this.photos = mergeSnapshot(cur, { photos: st.photos ?? [] }, this.seen).photos
      if (st.recent) this.recent = st.recent
      this.selected.clear()
      this.hooks.spreadsCleared()
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
