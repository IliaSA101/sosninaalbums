// Окно «Сохранить альбом как макет…» (131 U3, сц. 12). Снимок альбома делает Go (SaveLook), альбом не меняется —
// отмены нет, как у «Сохранить разворот…». Каталог тем берётся у lookApi; после успеха открытое окно «Макеты альбома» перечитывается.
// «Сохранить» серо заранее: PlanSaveLook (сухой прогон SaveLook) зовётся при открытии и смене выбора; отказ Go — причина под полями (131-212).
import { store } from '../../base/core/store.svelte'
import { errText } from '../../base/words/errtext'
import { isSheet } from '../../base/geom/sheet'
import { NAME_MAX, cleanName } from '../../panels/decor/decor'
import { spreadsel } from '../../panels/timeline/spreadsel.svelte'
import { lookApi } from './lookapi'
import type { LookPlanSave } from './looktypes'
import { looks } from './looks.svelte'
import {
  DEFAULT_THEME,
  NO_SPREADS,
  clampPos,
  clampTo,
  countOf,
  defaultRepeat,
  defaultScope,
  hasRepeat,
  saveOpts,
  savedText,
  themeItems,
  type SaveScope,
} from './savelook'

class SaveLookUi {
  open = $state(false)
  name = $state('')
  theme = $state(DEFAULT_THEME)
  themes = $state<{ id: string; name: string }[]>([])
  scope = $state<SaveScope>('all')
  from = $state(2)
  to = $state(1)
  places = $state(true)
  drafts = $state(true)
  cover = $state(true)
  busy = $state(false)
  plan = $state<LookPlanSave | null>(null) // последний ответ PlanSaveLook; null — ещё не пришёл или вызов упал
  private planSeq = 0 // номер запроса: устаревший ответ не перетирает новый
  private pending: Promise<void> | null = null // идущий запрос: commit ждёт его, иначе Enter после правки видит старый отказ

  // Лист-проект (031) макетом не бывает (design §8); альбом без разворотов сохранять нечего.
  get can(): boolean {
    return !isSheet(store.view.settings) && store.view.spreads.length > 0
  }
  get hasCover(): boolean {
    return !!store.cover
  }
  // Развороты ленты в порядке альбома — обложки среди них нет.
  private get all(): string[] {
    return store.view.spreads.map((s) => s.id)
  }
  get chosen(): string[] {
    return this.scope === 'selected' ? spreadsel.ordered(this.all) : this.all
  }
  get n(): number {
    return countOf(this.scope, this.all.length, spreadsel.ordered(this.all).length)
  }
  get selected(): number {
    return spreadsel.ordered(this.all).length
  }
  get repeat(): boolean {
    return hasRepeat(this.from, this.to)
  }
  get ready(): boolean {
    return cleanName(this.name) !== '' && this.n > 0 && !this.busy && this.plan?.ok !== false
  }
  // Причина отказа Go (текст SaveLook как есть); пока всё допустимо или ответа нет — пусто.
  get refusal(): string {
    return this.plan?.ok === false ? this.plan.reason : ''
  }

  async show() {
    if (!this.can) {
      if (!isSheet(store.view.settings)) store.notify(NO_SPREADS) // только обложка: в меню пункт живой, развороты — нет
      return
    }
    this.name = ''
    this.theme = DEFAULT_THEME
    this.places = this.drafts = true
    this.cover = this.hasCover
    this.setScope(defaultScope(this.selected))
    this.open = true
    this.plan = null
    void this.refresh()
    try {
      const cat = await lookApi.catalog()
      this.themes = themeItems(cat)
    } catch {
      this.themes = themeItems(null) // каталог недоступен: «Мои» остаётся, ошибку покажет само сохранение
    }
  }

  close() {
    this.open = false
    this.planSeq++ // ответ, летящий к закрытому окну, не нужен
    this.plan = null
  }

  // Сухой прогон SaveLook на текущем выборе. Прежний ответ остаётся до нового (причина не мигает); сбой вызова не серит:
  // сохранение само вернёт ошибку Go тостом.
  private refresh(): Promise<void> {
    const p = this.askPlan(++this.planSeq)
    this.pending = p
    void p.then(() => {
      if (this.pending === p) this.pending = null
    })
    return p
  }

  private async askPlan(my: number): Promise<void> {
    let r: LookPlanSave | null
    try {
      r = (await lookApi.planSave(this.form())) ?? null
    } catch {
      r = null
    }
    if (my === this.planSeq && this.open) this.plan = r
  }

  private form() {
    return saveOpts(
      {
        theme: this.theme,
        name: cleanName(this.name).slice(0, NAME_MAX),
        ids: this.chosen,
        from: this.from,
        to: this.to,
        places: this.places,
        drafts: this.drafts,
        cover: this.cover,
      },
      this.hasCover,
    )
  }

  // Смена набора разворотов возвращает повторяемые к умолчанию (с 2 по N−1): прежние позиции относились к другому числу.
  setScope(s: SaveScope) {
    if (this.open && s === this.scope) return // повторный щелчок по выбранному не сбрасывает «с/по»
    this.scope = s
    const d = defaultRepeat(this.n)
    this.from = d.from
    this.to = d.to
    if (this.open) void this.refresh()
  }

  // Пересчёт — только если число изменилось: лишних вызовов Go на тот же ввод нет.
  setFrom(v: number) {
    const x = clampPos(v, this.n)
    if (x === this.from) return
    this.from = x
    void this.refresh()
  }
  setTo(v: number) {
    const x = clampTo(v, this.n)
    if (x === this.to) return
    this.to = x
    void this.refresh()
  }
  setFlags(k: 'places' | 'drafts' | 'cover', v: boolean) {
    if (this[k] === v) return
    this[k] = v
    void this.refresh()
  }

  async commit(): Promise<boolean> {
    while (this.pending) await this.pending // свежий ответ Go до проверки: Enter сразу после правки границы
    if (!this.open || !this.ready) return false
    const name = cleanName(this.name).slice(0, NAME_MAX)
    this.busy = true
    try {
      await lookApi.save(this.form())
    } catch (e) {
      store.notify(errText(e), 'error') // «Не больше 24 разных разворотов — выделите нужные в ленте», «уже есть в библиотеке»
      return false
    } finally {
      this.busy = false
    }
    this.open = false
    store.notify(savedText(name))
    looks.changed() // открытое окно «Макеты альбома» перечитает каталог
    return true
  }
}

export const saveLookUi = new SaveLookUi()
