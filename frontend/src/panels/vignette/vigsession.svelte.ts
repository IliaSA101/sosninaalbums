import { capFirst, errText } from '../../base/core/errtext'
import { store } from '../../base/core/store.svelte'
import { sheetIn } from '../../base/geom/sheet'
import type { VigFamilyFilter } from '../../base/tpl/tplfilter'
import type { SpreadView, VigChoice, VigPreview, VigSetup, VignetteView } from '../../base/core/types'
import { applySetup, previewVignette, vigChoices } from './vigact'
import { dialogBlock, sameAsDialog, setupOf, varsFormOf, type VarsForm } from './vigdialog'
import { choiceOf, formOf, groupCount, isVignette, totalPlaces, type VigForm } from './vignette'

// Запросы окна не копятся (031 dialog §3): один в полёте, пока он идёт — запоминается только последнее
// значение, промежуточные пропускаются. Ответ на устаревшее значение не показывается: Go не заваливается
// стрелкой ↑, зажатой на «Учеников», а картинка не мигает старыми числами.
export class Latest<A, R> {
  value = $state<R | null>(null)
  error = $state('')
  pending = $state(false)
  calls = 0 // запросов ушло в Go (для проверки слияния)

  private running = false
  private wanted: { a: A } | null = null

  constructor(private ask: (a: A) => Promise<R>) {}

  request(a: A) {
    this.wanted = { a }
    this.pending = true
    if (!this.running) void this.pump()
  }

  private async pump() {
    this.running = true
    try {
      while (this.wanted) {
        const { a } = this.wanted
        this.wanted = null
        this.calls++
        try {
          const r = await this.ask(a)
          if (!this.wanted) {
            this.value = r
            this.error = ''
          }
        } catch (e) {
          // Ошибка — только у последнего значения; прежний ответ остаётся на экране.
          if (!this.wanted) this.error = capFirst(errText(e))
        }
      }
    } finally {
      this.running = false
      this.pending = false
    }
  }
}

export interface VigDeps {
  preview: (id: string, s: VigSetup) => Promise<VigPreview>
  choices: (id: string, f: Pick<VigForm, 'teachers' | 'pupils' | 'ratio'>) => Promise<VigChoice[]>
  apply: (id: string, s: VigSetup) => Promise<boolean>
  // Лист в проекте сейчас; нет — сравнение идёт со снимком при открытии.
  live?: (id: string) => (SpreadView & { vig: VignetteView }) | undefined
}

const REAL: VigDeps = {
  preview: previewVignette,
  choices: vigChoices,
  apply: applySetup,
  live: (id) => {
    const sp = sheetIn(store.view, id)
    return isVignette(sp) ? sp : undefined
  },
}

// Состояние открытого окна «Настроить виньетку»: форма и поля шапки берутся из листа при открытии и дальше
// живут сами (правки листа извне — Undo — форму не сбрасывают); в проект до «Применить» ничего не пишется.
export class VigSession {
  form = $state<VigForm>({ tpl: '', teachers: 0, pupils: 0, ratio: '3:4', ttext: '' })
  vars = $state<VarsForm>(varsFormOf(undefined))
  busy = $state(false)
  family = $state<VigFamilyFilter>('all') // фильтр видов над сеткой (049): при открытии окна «Все»
  readonly preview: Latest<VigSetup, VigPreview>
  readonly choices: Latest<Pick<VigForm, 'teachers' | 'pupils' | 'ratio'>, VigChoice[]>

  constructor(
    readonly id: string,
    private sheet: SpreadView & { vig: VignetteView },
    private deps: VigDeps = REAL,
  ) {
    this.form = formOf(sheet.vig)
    this.vars = varsFormOf(sheet.vars)
    this.preview = new Latest((s) => deps.preview(id, s))
    this.choices = new Latest((f) => deps.choices(id, f))
  }

  get setup(): VigSetup {
    return setupOf(this.form, this.vars)
  }

  // Запрос предпросмотра для текущей формы и полей шапки (зовёт $effect окна: он следит за ними).
  refreshPreview() {
    this.preview.request(setupOf(this.form, this.vars))
  }

  // Запрос карточек шаблонов: зависит только от чисел и пропорции — набор букв шапки его не дёргает.
  refreshChoices() {
    const { teachers, pupils, ratio } = this.form
    this.choices.request({ teachers, pupils, ratio })
  }

  // Выбранную в окне (ещё не применённую) карточку удалили ×: её больше нет среди карточек Go —
  // выбор возвращается к шаблону листа. Шаблон листа, удалённый из файла, Go отдаёт карточкой gone.
  fixTpl() {
    const list = this.choices.value
    if (!list || choiceOf(list, this.form.tpl)) return
    const sheetTpl = (this.deps.live?.(this.id) ?? this.sheet).vig.tpl
    if (this.form.tpl !== sheetTpl) this.form.tpl = sheetTpl
  }

  get items(): VigChoice[] {
    return this.choices.value ?? []
  }

  get now(): VigChoice | undefined {
    return choiceOf(this.items, this.form.tpl)
  }

  get total(): number {
    return totalPlaces(this.form, groupCount(this.form.tpl, this.items, this.sheet.vig))
  }

  get max(): number {
    return this.sheet.vig.max || 64
  }

  get unchanged(): boolean {
    // С живым листом на момент проверки: правка листа извне (Undo) не оставляет «Применить» закрытым по снимку.
    return sameAsDialog(this.form, this.vars, this.deps.live?.(this.id) ?? this.sheet)
  }

  get block(): string | null {
    return dialogBlock({ busy: this.busy, tpl: this.form.tpl, total: this.total, max: this.max, same: this.unchanged })
  }

  // «Применить»: true — записано, окно можно закрывать; ошибка Go — тост (в vigact), окно остаётся.
  async apply(): Promise<boolean> {
    if (this.block) return false
    this.busy = true
    try {
      return await this.deps.apply(this.id, this.setup)
    } finally {
      this.busy = false
    }
  }
}

// Какое окно открыто: id листа или null. Окно в App.svelte рисует сессию заново на каждое открытие.
class VigDialogUi {
  id = $state<string | null>(null)
  open(id: string) {
    this.id = id
  }
  close() {
    this.id = null
  }
}

export const vigDialog = new VigDialogUi()
