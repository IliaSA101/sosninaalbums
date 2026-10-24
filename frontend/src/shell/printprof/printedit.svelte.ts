import { Api, store } from '../../base/core/store.svelte'
import type { PrintEdit, PrintEditCheck, PrintImport, PrintShare } from '../../base/core/types'
import { modes } from '../../base/prefs/view.svelte'
import { capFirst, errText } from '../../base/words/errtext'
import { printList } from './printlist.svelte'
import {
  MAX_FORMATS,
  COPIED_LINE,
  addedTitle,
  isOwnId,
  removeQuestion,
  removedText,
  REMOVE_NOTE,
  replacedText,
  sameText,
  savedText,
  sharedText,
  summaryLine,
} from './printwords'

// Вопрос поверх окна: замена своей типографии файлом или удаление (образец — «Наборы библиотеки»).
export type PrintAsk =
  | { kind: 'replace'; token: string; title: string; text: string; note: string }
  | { kind: 'remove'; id: string; title: string; text: string; note: string }

// Блок «добавлено из файла» над полями.
export interface PrintMade {
  title: string
  lines: string[]
}

const blank = (): PrintEdit => ({
  name: '',
  base: '',
  dpi: 300,
  layflat: false,
  safe: 10,
  bleed: 3,
  formats: [],
  spreads: 1,
  prefix: '',
  quality: 100,
  url: '',
})

// Поле числа даёт null, пока оно пустое: Go получает 0 («как в альбоме», «нет»), а ошибку пишет сам.
const num = (v: number | null | undefined): number => (typeof v === 'number' && Number.isFinite(v) ? v : 0)

// Поля окна — в том виде, как уходят в Go.
export function toEdit(e: PrintEdit): PrintEdit {
  return {
    name: e.name ?? '',
    base: e.base ?? '',
    dpi: num(e.dpi),
    layflat: !!e.layflat,
    safe: num(e.safe),
    bleed: num(e.bleed),
    spreads: num(e.spreads),
    prefix: e.prefix ?? '',
    quality: num(e.quality),
    url: e.url ?? '',
    formats: (e.formats ?? []).map((f) => ({
      name: f.name ?? '',
      pxW: num(f.pxW),
      pxH: num(f.pxH),
      coverW: num(f.coverW),
      coverH: num(f.coverH),
    })),
  }
}

// Окно «Файл ▸ …» — «Типография ▸ Своя типография…» (123 Б3, макет docs/private/mockups/0.26/b3-print-profile-window.html).
// Поля — черновик; расчёт (мм, ошибки, «Сейчас альбом даёт…») каждый раз у Go (CheckPrintProfile), локально ничего не считается.
// «Сохранить» создаёт новую типографию и ставит её альбому (один шаг отмены у Go); встроенную своей не перекрыть.
class PrintEditUi {
  open = $state(false)
  loading = $state(false)
  busy = $state(false)
  e = $state<PrintEdit>(blank())
  check = $state<PrintEditCheck | null>(null)
  ask = $state<PrintAsk | null>(null)
  made = $state<PrintMade | null>(null)
  private checkReq = 0
  private loadReq = 0

  get ownBase(): boolean {
    return isOwnId(printList.infos, this.e.base)
  }

  get baseName(): string {
    return printList.infos.find((i) => i.id === this.e.base)?.name ?? ''
  }

  // Открыть на основе типографии base ("" — пустая заготовка без обложки).
  async show(base: string) {
    if (!(await this.fill(base, null))) return
    this.open = true
  }

  close() {
    this.open = false
    this.ask = null
    this.made = null
    this.loadReq++
    this.checkReq++
  }

  // Заготовка из Go для основы base; false — Go отказал (ошибка тостом, окно как было).
  private async fill(base: string, made: PrintMade | null): Promise<boolean> {
    const my = ++this.loadReq
    this.loading = true
    try {
      await printList.load()
      const e = (await Api.PrintProfileEdit(base)) as unknown as PrintEdit
      if (my !== this.loadReq) return false
      this.e = { ...blank(), ...e, formats: e.formats ?? [] }
      this.check = null
      this.ask = null
      this.made = made
      return true
    } catch (err) {
      if (my === this.loadReq) store.notify(capFirst(errText(err)), 'error')
      return false
    } finally {
      if (my === this.loadReq) this.loading = false
    }
  }

  // «Основа» сменилась: поля заново из новой основы (как в макете).
  async setBase(id: string) {
    if (id === this.e.base) return
    await this.fill(id, null)
  }

  // Живой расчёт у Go; ответ прежнего запроса отбрасывается.
  async recheck() {
    const my = ++this.checkReq
    try {
      const c = (await Api.CheckPrintProfile(toEdit(this.e) as never)) as unknown as PrintEditCheck
      if (my === this.checkReq) this.check = { ...c, errs: c.errs ?? {}, formats: c.formats ?? [], lines: c.lines ?? [] }
    } catch (err) {
      if (my === this.checkReq) store.notify(capFirst(errText(err)), 'error')
    }
  }

  get canAdd(): boolean {
    return this.e.formats.length < MAX_FORMATS
  }

  addRow() {
    if (!this.canAdd) return
    this.e.formats.push({ name: '', pxW: 0, pxH: 0, coverW: 0, coverH: 0 })
  }

  delRow(i: number) {
    this.e.formats.splice(i, 1)
  }

  setQuality(q: number) {
    this.e.quality = q
  }

  get canSave(): boolean {
    return !!this.check?.ok && !this.busy && !this.loading
  }

  // «Сохранить»: новая типография и сразу альбому.
  async save() {
    if (!this.canSave) return
    const name = this.e.name.trim()
    this.busy = true
    try {
      const res = await Api.SavePrintProfile(toEdit(this.e) as never, true)
      await store.take(res.view as never)
      modes.setPresetGroup(name)
      printList.saves++
      store.notify(savedText(name))
      this.close()
    } catch (err) {
      store.notify(capFirst(errText(err)), 'error')
    } finally {
      this.busy = false
    }
  }

  // «Добавить из файла…»: диалог — в Go (в тестах не вызывается).
  async importFile() {
    if (this.busy) return
    this.busy = true
    try {
      await this.handle((await Api.ImportPrintProfile()) as unknown as PrintImport, false)
    } catch (err) {
      store.notify(capFirst(errText(err)), 'error')
    } finally {
      this.busy = false
    }
  }

  private async handle(r: PrintImport, replaced: boolean) {
    if (r.canceled) return
    if (r.ask === 'same') {
      store.notify(sameText(r.info.name))
      return
    }
    if (r.ask === 'replace') {
      this.ask = {
        kind: 'replace',
        token: r.token,
        title: r.info.name,
        text: r.question,
        note: summaryLine(r.lines ?? []),
      }
      return
    }
    if (replaced) {
      store.notify(replacedText(r.info.name, r.info.version))
      await this.fill(r.info.id, null)
      return
    }
    const lines = [...(r.lines ?? []), ...(r.copied ? [COPIED_LINE] : [])]
    await this.fill(r.info.id, { title: addedTitle(r.info.name), lines })
  }

  // «Заменить» в вопросе: Go ставит версию из файла (вопрос устарел — ошибкой тостом).
  async confirmReplace() {
    const a = this.ask
    if (a?.kind !== 'replace' || this.busy) return
    this.ask = null
    this.busy = true
    try {
      await this.handle((await Api.ConfirmImportPrintProfile(a.token)) as unknown as PrintImport, true)
    } catch (err) {
      store.notify(capFirst(errText(err)), 'error')
    } finally {
      this.busy = false
    }
  }

  // «Поделиться файлом…» — только у своей; диалог записи — в Go.
  async share() {
    if (!this.ownBase || this.busy) return
    this.busy = true
    try {
      const r = (await Api.SharePrintProfile(this.e.base)) as unknown as PrintShare
      if (!r.canceled) store.notify(sharedText(r.name))
    } catch (err) {
      store.notify(capFirst(errText(err)), 'error')
    } finally {
      this.busy = false
    }
  }

  // «Удалить» — только у своей; сначала вопрос.
  askRemove() {
    if (!this.ownBase || this.busy) return
    this.ask = {
      kind: 'remove',
      id: this.e.base,
      title: this.baseName,
      text: removeQuestion(this.baseName, this.e.base),
      note: REMOVE_NOTE,
    }
  }

  dismiss() {
    this.ask = null
  }

  // «Удалить» в вопросе: альбомы не меняются (пометка изделия остаётся), окно встаёт на первую встроенную.
  async confirmRemove() {
    const a = this.ask
    if (a?.kind !== 'remove' || this.busy) return
    this.ask = null
    this.busy = true
    try {
      await store.take((await Api.DeletePrintProfile(a.id)) as never)
      store.notify(removedText(a.title))
      await printList.load()
      await this.fill(printList.infos.find((i) => i.source === 'builtin')?.id ?? '', null)
    } catch (err) {
      store.notify(capFirst(errText(err)), 'error')
    } finally {
      this.busy = false
    }
  }
}

export const printEdit = new PrintEditUi()
