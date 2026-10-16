import { Api, store } from '../../base/core/store.svelte'
import { edit } from '../../editor/text/edit.svelte'
import { errText, capFirst } from '../../base/core/errtext'
import type { NamesFile, NamesOpts, NamesPlan, NamesResult } from './nametypes'
import { sheetIn } from '../../base/geom/sheet'

// Пауза после ввода перед запросом предпросмотра к Go.
export const PREVIEW_DELAY = 150

// Окно «Имена» (030, 043): список или имена файлов → имена фото листа. Разбор, заголовки и сопоставление считает Go:
// предпросмотр (PreviewNames) и вставка (PasteNames) идут по одному плану. Пока окно открыто, App не исполняет клавиши.
// Ошибки Go — в самом окне (тост под затемнением не виден).
class NamesUi {
  open = $state(false)
  spreadId = $state('')
  from = $state<'list' | 'files'>('list')
  text = $state('')
  firstFirst = $state(false) // «Имя первым»: помнится до конца сеанса
  caption = $state(true) // «Подписать фото», если у листа нет шаблона подписи (сц. 8а)
  busy = $state(false)
  loading = $state(false) // диалог файла открыт
  error = $state('') // ошибка файла или вставки
  planError = $state('') // ошибка предпросмотра (список слишком длинный…)
  file = $state<NamesFile | null>(null) // что загружено из файла: строка сведений над полем
  plan = $state<NamesPlan | null>(null)

  private timer: ReturnType<typeof setTimeout> | undefined
  private seq = 0 // номер запроса: ответ на устаревший ввод отбрасывается
  private fresh = false // первый запрос после открытия — без паузы

  async show(spreadId?: string) {
    const sp = sheetIn(store.view, spreadId ?? store.spread?.id ?? '')
    if (!sp) return
    await edit.idle() // набранный текст сначала записывается
    this.spreadId = sp.id
    this.from = 'list'
    this.text = ''
    this.caption = true
    this.error = ''
    this.planError = ''
    this.file = null
    this.plan = null
    this.fresh = true
    this.open = true
  }

  close() {
    this.open = false
    clearTimeout(this.timer)
    this.seq++ // ответ, который ещё в пути, не нужен
  }

  // Что уходит в Go. В режиме «Имена файлов» текст поля не нужен.
  opts(): NamesOpts {
    return { from: this.from, text: this.from === 'files' ? '' : this.text, firstFirst: this.firstFirst, caption: this.caption }
  }

  // Вызывается из $effect окна: читает все входы предпросмотра (они становятся зависимостями) и заказывает запрос.
  refresh() {
    void this.spreadId
    void store.view.rev // лист правили (например, «Настроить виньетку…») — план другой
    const o = this.opts()
    if (!this.open) return
    clearTimeout(this.timer)
    const delay = this.fresh ? 0 : PREVIEW_DELAY
    this.fresh = false
    const id = this.spreadId
    this.timer = setTimeout(() => void this.ask(id, o), delay)
  }

  private async ask(id: string, o: NamesOpts) {
    const n = ++this.seq
    try {
      const plan = (await Api.PreviewNames(id, o)) as unknown as NamesPlan
      if (n !== this.seq) return
      this.plan = plan
      this.planError = ''
    } catch (e) {
      if (n !== this.seq) return
      this.plan = null
      this.planError = capFirst(errText(e))
    }
  }

  // «Загрузить таблицу…»: диалог и чтение файла — в Go. Файл становится текстом поля, проект не меняется;
  // отказ от диалога оставляет прежний текст.
  async loadFile(): Promise<boolean> {
    if (this.loading) return false
    this.loading = true
    this.error = ''
    try {
      const f = (await Api.PickNamesFile()) as unknown as NamesFile
      if (f.cancelled) return false
      this.text = f.text
      this.file = f
      return true
    } catch (e) {
      this.error = capFirst(errText(e))
      return false
    } finally {
      this.loading = false
    }
  }

  // Есть что вставлять: хоть одно место получит имя.
  get canPaste(): boolean {
    return !this.busy && !this.planError && !!this.plan?.rows.some((r) => r.who)
  }

  // «Имя первым» ничего не меняет, когда фамилия и имя лежат в отдельных столбцах.
  get orderOff(): boolean {
    return this.from === 'list' && !!this.plan && !this.plan.order
  }

  // Вставка одним шагом отмены; итог — тостом.
  async apply(): Promise<boolean> {
    if (this.busy) return false
    this.busy = true
    this.error = ''
    try {
      const before = store.view.rev
      const r = (await Api.PasteNames(this.spreadId, this.opts())) as unknown as NamesResult
      store.apply(r.view)
      this.close()
      // Те же имена уже стоят: rev не вырос, шага отмены нет — тост «Вставлены» был бы неправдой.
      if (r.view.rev === before) {
        store.notify('Без изменений')
        return true
      }
      store.notify(`Имена вставлены. Подписано: ${r.named} из ${r.seats}, лишних строк: ${r.extra}`)
      return true
    } catch (e) {
      this.error = capFirst(errText(e))
      return false
    } finally {
      this.busy = false
    }
  }
}

export const namesUi = new NamesUi()
