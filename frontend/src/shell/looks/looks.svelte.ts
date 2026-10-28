// Состояние окна «Макеты альбома» (131 U1). Окно показывает макет справа и ничего не ставит, пока не нажато «Применить»
// (0.27-131-12). Плитка, тема и число разворотов живут в экземпляре до конца сеанса: «Файл ▸ Макеты альбома…» открывает их снова.
// Серию, числа и план считает Go (lookApi); окно хранит выбор и ответы. Три счётчика запросов — каталог, карточка, числа:
// ответ с устаревшим номером отбрасывается, и пересчёт чисел (sync) не отменяет идущую загрузку карточки.
import { store } from '../../base/core/store.svelte'
import { errText } from '../../base/words/errtext'
import { modes } from '../../base/prefs/view.svelte'
import { ALL, cleanName, firstTheme } from '../../panels/decor/decor'
import type { GetItem } from '../../panels/decorbrowse/decorget'
import { getApi } from '../../panels/decorbrowse/getapi'
import { lookApi } from './lookapi'
import { COVER_ID } from '../../base/geom/sheet'
import { putQuestion, putToast } from '../../panels/decor/spreadtpl'
import { applyToast, clampSpreads, defaultSpreads, effMode, moveStep, needsAsk, strip, tileStep, type FullMode } from './looks'
import type { LookCatalog, LookCount, LookInfo, LookItem, LookMode, LookOpts, LookPlan } from './looktypes'

// rename / hide — ✎ и × у своих макетов (как в «Развороты»); apply — вопрос о замене своего; step — вопрос «Поставить на…»
// (spread/step запоминаются: шаг ставится на тот лист, о котором спросили).
export type LooksAsk =
  | { kind: 'rename'; id: string }
  | { kind: 'hide'; id: string; name: string }
  | { kind: 'apply'; plan: LookPlan; o: LookOpts } // o — запрос, о котором спросили: ставится именно он (plan.spreads — число Go после постановки, не «сколько добавить»)
  | { kind: 'step'; id: string; spread: string; step: number; text: string }
  | { kind: 'get'; tile: GetItem } // 144: «Скачать набор?» перед скачиванием образца макета

class LooksUi {
  open = $state(false)
  cat = $state<LookCatalog | null>(null)
  theme = $state('')
  loading = $state(false)
  demoOffer = $state(false) // каталог пуст, а в «Можно скачать» есть пример (142): пустое окно ведёт к нему
  sel = $state('') // id выбранной плитки; '' — не выбрана
  spreads = $state(0) // «Разворотов» в режиме «весь альбом»
  addN = $state(1) // «Разворотов» в режиме «Добавить»: сколько повторяемых добавить (0.28-131-67)
  pick = $state<FullMode>('bg') // «Что сделать» (только в собранном альбоме)
  step = $state(1) // шаг в предпросмотре: 0 — обложка, 1…24
  info = $state<LookInfo | null>(null)
  cnt = $state<LookCount | null>(null)
  plan = $state<LookPlan | null>(null) // PlanLook(series): full и prev для окна
  addPlan = $state<LookPlan | null>(null) // PlanLook(add, addN): места добавляемых разворотов
  busy = $state(false)
  ask = $state<LooksAsk | null>(null)
  name = $state('') // поле имени в ✎ — отдельно от ask: bind:value отпускает поле позже окна
  private reqCat = 0
  private reqInfo = 0
  private reqCnt = 0
  private rev = -1
  opened = 0 // номер открытия окна: скачанный макет узнаёт по нему, что окно закрывали (144)
  // 073-1: названия поставленных за сеанс наборов; окно встаёт на тему первого из них один раз при открытии.
  private fresh = new Set<string>()

  get item(): LookItem | undefined {
    return this.cat?.items.find((i) => i.id === this.sel)
  }
  get mode(): LookMode {
    return effMode(this.plan, this.pick)
  }
  get chips() {
    return strip(this.info)
  }
  // Число в поле «Разворотов»: в «Добавить» своё, иначе — разворотов серии.
  get shownN(): number {
    return this.mode === 'add' ? this.addN : this.spreads
  }
  // Места под полем: серии — из LookCount, «Добавить» — из плана добавления.
  get shownPlaces(): number | null {
    return (this.mode === 'add' ? this.addPlan?.places : this.cnt?.places) ?? null
  }
  // Карточка и план пришли: только тогда «Применить» живая (между сменой плитки и ответом Go держится старое).
  get ready(): boolean {
    return !!this.sel && !!this.info && !!this.plan
  }

  // Сколько фото возьмёт подбор: выделенные, иначе свободные ленты (как у «Автосборки», без тоста).
  photos(): number {
    return store.selectedIds.length || store.hooks.pickFree().length
  }

  // K взят из выделения (иначе — из свободных фото ленты): от этого зависит слово в подписи подбора.
  get fromSelection(): boolean {
    return store.selectedIds.length > 0
  }

  async show() {
    this.open = true
    this.opened++
    this.ask = null
    this.rev = store.view.rev
    getApi.previews() // 144: образцы нескачанных наборов докачиваются, пока окно открыто
    const fresh = this.fresh.size ? new Set(this.fresh) : undefined
    this.fresh.clear()
    await this.refresh(fresh)
  }

  close() {
    this.open = false
    this.ask = null
    this.reqCat++ // запоздавшие ответы окну не нужны
    this.reqInfo++
    this.reqCnt++
  }

  // «Изменить…» у формата: как кнопка «Формат» шапки — окно закрывается, «Настройки альбома» раскрываются; выбор остаётся.
  openFormat() {
    this.close()
    modes.openSection('settings')
  }

  // library:changed (набор добавлен, удалён, обновлён): названия поставленных запоминаются для следующего открытия,
  // открытое окно перечитывает каталог, тему не меняет (как «Развороты»).
  changed(installed?: readonly string[] | null) {
    installed?.forEach((t) => this.fresh.add(t))
    if (this.open) void this.refresh()
  }

  // Каталог заново (открытие, ✎ ×, library:changed); выбор, которого больше нет, сбрасывается.
  // fresh — только при открытии: тема первого свежего набора.
  async refresh(fresh?: ReadonlySet<string>) {
    const my = ++this.reqCat
    this.loading = true
    try {
      const cat = await lookApi.catalog()
      if (my !== this.reqCat) return
      this.cat = cat
      this.theme = firstTheme(cat, this.theme, fresh)
      this.demoOffer = cat.items.length === 0 ? await this.offered() : false
      if (my !== this.reqCat) return
    } catch (e) {
      if (my === this.reqCat) store.notify(errText(e), 'error')
      return
    } finally {
      if (my === this.reqCat) this.loading = false
    }
    if (this.sel && !this.item) this.drop()
    if (this.sel) await this.load(false)
  }

  // Ошибка чтения списка наборов окну макетов не помеха: остаётся прежний текст пустого окна.
  private async offered(): Promise<boolean> {
    try {
      return await lookApi.demoOffer()
    } catch {
      return false
    }
  }

  setTheme(id: string) {
    this.theme = id
  }

  // Щелчок по плитке показывает макет справа. Повторный щелчок по выбранной — ничего.
  // Ответы прежнего макета снимаются сразу: до ответа Go «Применить» серая.
  async choose(id: string) {
    if (id === this.sel) return
    this.sel = id
    this.clear()
    await this.load(true)
  }

  private clear() {
    this.info = this.cnt = this.plan = this.addPlan = null
  }

  private drop() {
    this.sel = ''
    this.clear()
    this.spreads = 0
    this.addN = 1
    this.reqInfo++ // идущие ответы прежнего макета не оживят выбор
    this.reqCnt++
  }

  // Карточка макета (шаги, пределы) и числа. fresh — другая плитка: числа и шаг по умолчанию.
  private async load(fresh: boolean) {
    const my = ++this.reqInfo
    this.reqCnt++ // числа прежнего запроса не нужны: после карточки они считаются заново
    const id = this.sel
    try {
      const info = await lookApi.info(id)
      if (my !== this.reqInfo) return
      this.info = info
      this.spreads =
        fresh || this.spreads <= 0
          ? this.item?.steps
            ? clampSpreads(this.item.steps, info)
            : defaultSpreads(info)
          : clampSpreads(this.spreads, info)
      if (fresh) this.addN = 1
      const first = tileStep(this.item ?? {})
      if (fresh || !this.chips.some((c) => c.i === this.step))
        this.step = this.chips.some((c) => c.i === first) ? first : (this.chips[0]?.i ?? 1)
    } catch (e) {
      if (my === this.reqInfo) store.notify(errText(e), 'error')
      return
    }
    await this.recount()
  }

  // Число мест и план при новом числе разворотов; своя нумерация, карточку не отменяет.
  private async recount() {
    const my = ++this.reqCnt
    const id = this.sel
    if (!id) return
    const n = this.spreads
    try {
      const [cnt, plan] = await Promise.all([lookApi.count(id, n, this.photos()), lookApi.plan(id, { mode: 'series', spreads: n })])
      if (my !== this.reqCnt) return
      // В собранном альбоме с «Добавить» места считает план добавления (число — addN), а не серия
      const add = plan.full && this.pick === 'add' ? await lookApi.plan(id, { mode: 'add', spreads: this.addN }) : null
      if (my !== this.reqCnt) return
      this.cnt = cnt
      this.plan = plan
      this.addPlan = add
    } catch (e) {
      if (my === this.reqCnt) store.notify(errText(e), 'error')
    }
  }

  async setSpreads(n: number) {
    if (!this.info) return
    if (this.mode === 'add') {
      const v = clampSpreads(n, { min: 1, max: this.info.max })
      if (v === this.addN) return
      this.addN = v
    } else {
      const v = clampSpreads(n, this.info)
      if (v === this.spreads) return
      this.spreads = v
    }
    await this.recount()
  }

  // «Подобрать под N выбранных/свободных фото»: число разворотов серии от Go (LookCount.fit). В «Добавить» ссылки нет.
  async fit() {
    if (this.mode === 'add' || !this.cnt || this.cnt.fit <= 0) return
    await this.setSpreads(this.cnt.fit)
  }

  async setPick(m: FullMode) {
    if (m === this.pick) return
    this.pick = m
    if (this.sel && this.info) await this.recount()
  }

  go(delta: number) {
    this.step = moveStep(this.chips, this.step, delta)
  }

  // Правка альбома событиями Go (формат в панели деталей, другая правка): числа и план — заново.
  // План и числа снимаются до ответа: «Применить» серая, пока не пришли свежие.
  sync(rev: number) {
    if (!this.open || rev === this.rev) return
    this.rev = rev
    if (!this.sel) return
    this.cnt = this.plan = this.addPlan = null
    void this.recount()
  }

  // «Применить»: свежий PlanLook выбранного режима (не тот, что показан в окне); вопрос (U2) — только если заменится своё.
  // Без вопроса — ставим сразу. Плитка сменилась, пока ждали план, — выходим.
  async apply() {
    if (!this.ready || this.busy) return
    const id = this.sel
    const o = { mode: this.mode, spreads: this.shownN }
    this.busy = true
    try {
      const p = await lookApi.plan(id, o)
      if (id !== this.sel || !this.open) return
      if (needsAsk(p)) {
        this.ask = { kind: 'apply', plan: p, o }
        return
      }
      await this.doApply(id, o)
    } catch (e) {
      store.notify(errText(e), 'error')
    } finally {
      this.busy = false
    }
  }

  // Ответ «Применить» на вопрос.
  async confirm() {
    if (this.ask?.kind !== 'apply' || !this.sel || this.busy) return
    const o = this.ask.o
    this.ask = null
    this.busy = true
    try {
      await this.doApply(this.sel, o)
    } catch (e) {
      store.notify(errText(e), 'error')
    } finally {
      this.busy = false
    }
  }

  // Один вызов Go — один шаг отмены. Окно закрывается после успеха; ошибка оставляет его открытым.
  // Тост берёт числа из ответа Go; версия не выросла — повтор, «Вернуть» не нужен.
  private async doApply(id: string, o: LookOpts) {
    const name = this.item?.name ?? ''
    const before = store.view.rev
    let plan: LookPlan | undefined
    const ok = await store.run(
      lookApi.apply(id, o).then((r) => {
        plan = r.plan
        return r.view
      }),
    )
    if (!ok) return
    this.close()
    if (!plan) return
    const t = applyToast(name, plan, store.view.rev !== before, o.spreads)
    if (t.undo) store.undoToast(t.text)
    else store.notify(t.text)
  }

  // Лист, на который встанет показанный шаг: текущий (обложка — COVER_ID). Шаг 0 — обложка, остальные — развороты.
  get stepTarget(): { spread: string; cover: boolean; no: number } | null {
    const sp = store.spread
    if (!sp) return null
    const cover = store.onCover
    return { spread: cover ? COVER_ID : sp.id, cover, no: store.current + 1 }
  }
  // Обложка встаёт только на обложку, остальные шаги — на разворот; иначе кнопка серая.
  get stepOk(): boolean {
    const t = this.stepTarget
    return !!t && !!this.sel && !!this.info && !this.busy && t.cover === (this.step === 0)
  }

  // «Поставить на разворот N»: сухой прогон 067; чисел для вопроса нет — ставим сразу.
  async askStep() {
    const t = this.stepTarget
    if (!t || !this.stepOk) return
    const { sel: id, step } = this
    this.busy = true
    try {
      const plan = await lookApi.planStep(t.spread, id, step)
      if (id !== this.sel || !this.open) return
      const text = putQuestion(plan, store.spread, this.info?.steps[step - 1]?.places ?? 0)
      if (text) {
        this.ask = { kind: 'step', id, spread: t.spread, step, text }
        return
      }
      await this.putStep(id, t.spread, step, t.cover)
    } catch (e) {
      store.notify(errText(e), 'error')
    } finally {
      this.busy = false
    }
  }

  async confirmStep() {
    const a = this.ask
    if (a?.kind !== 'step' || this.busy) return
    this.ask = null
    this.busy = true
    try {
      await this.putStep(a.id, a.spread, a.step, a.spread === COVER_ID)
    } catch (e) {
      store.notify(errText(e), 'error')
    } finally {
      this.busy = false
    }
  }

  // Постановка: места и оформление одним шагом отмены, окно закрывается после успеха (как «Развороты»).
  private async putStep(id: string, spread: string, step: number, cover: boolean) {
    const before = store.view.rev
    let res: { left: number; missing: number } | undefined
    const ok = await store.run(
      lookApi.applyStep(spread, id, step).then((r) => {
        res = r
        return r.view
      }),
    )
    if (!ok || !res) return
    this.close()
    const t = putToast(store.view.rev !== before, res.left, res.missing, cover)
    if (t.undo) store.undoToast(t.text)
    else store.notify(t.text)
  }

  // Правый щелчок по «+» ▸ «Разворот макета «…»»: Go ставит следующий повторяемый шаг одной правкой. Как addVignette:
  // id до вызова, после — переход на новый разворот (фотограф видит, что добавилось); повторный вызов, пока идёт первый, молчит.
  // Тост «Вернуть» оставлен: макетный разворот, в отличие от виньетки, не открывает окна, и отмена нужна на виду.
  // Холостая правка (шага нет, rev тот же) — без перехода и тоста.
  private adding = false
  async newSpread() {
    if (this.adding) return
    this.adding = true
    try {
      const before = store.view.rev
      const ids = new Set(store.view.spreads.map((s) => s.id))
      if (!(await store.run(lookApi.newSpread())) || store.view.rev === before) return
      const i = store.view.spreads.findIndex((s) => !ids.has(s.id))
      if (i >= 0) store.go(i)
      const name = store.view.look?.name
      store.undoToast(name ? `Разворот макета «${name}» добавлен` : 'Разворот макета добавлен')
    } finally {
      this.adding = false
    }
  }

  askRename(it: Pick<LookItem, 'id' | 'name'>) {
    this.name = it.name
    this.ask = { kind: 'rename', id: it.id }
  }
  askHide(it: Pick<LookItem, 'id' | 'name'>) {
    this.ask = { kind: 'hide', id: it.id, name: it.name }
  }
  dismiss() {
    this.ask = null
  }

  // 144: щелчок по плитке скачивания — вопрос «Скачать набор?» (ход и показ — lookget.svelte).
  askGet(tile: GetItem) {
    this.ask = { kind: 'get', tile }
  }

  // 144: макет скачанного набора — каталог перечитан, тема карточки, карточка выбрана и показана справа. Ничего не применяет:
  // «Применить» — кнопкой формы (0.27-131-12). Каталога с этим макетом нет (скрыт, набор без него) — молча.
  async showGot(item: Pick<LookItem, 'id' | 'theme'>) {
    await this.refresh()
    if (!this.open || !this.cat?.items.some((i) => i.id === item.id)) return
    this.theme = this.cat.themes.some((t) => t.id === item.theme) ? item.theme : ALL
    await this.choose(item.id)
  }

  // «Сохранить» / «Убрать»: вопрос закрывается сразу, каталог перечитывается после успеха. Отмены нет (библиотека, не альбом).
  async commit(): Promise<boolean> {
    const a = this.ask
    if (!a || (a.kind !== 'rename' && a.kind !== 'hide')) return false
    this.ask = null
    try {
      if (a.kind === 'rename') await lookApi.rename(a.id, cleanName(this.name))
      else await lookApi.hide(a.id)
    } catch (e) {
      store.notify(errText(e), 'error')
      return false
    }
    await this.refresh()
    return true
  }
}

export const looks = new LooksUi()
