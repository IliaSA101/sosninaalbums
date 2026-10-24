import * as Api from '../../../wailsjs/go/app/App'
import { EventsOn } from '../../../wailsjs/runtime/runtime'
import { capFirst, collectDoneText, errText, exportDoneText, exportStoppedText } from '../words/errtext'
import type { Toast } from './toasts.svelte'
import type { ExportOpts, PrintPlan } from './types'

// Ход сбора в папку: файлов и байт скопировано из всего к копированию (уже лежавшие не считаются).
export interface CollectProgress {
  doneFiles: number
  totalFiles: number
  doneBytes: number
  totalBytes: number
}

// Окно «Открываем альбом» появляется через столько после первого open:progress: 150 мс счёта у Go + эта пауза = 0,4 с,
// как в спеке 073; короткий прогрев (последнее событие пришло раньше) окна не показывает.
export const OPEN_SHOW_AFTER = 250

// Что фоновым работам нужно от Store (структурный тип: цикла импортов со store нет).
export interface JobsHost {
  readonly sheets: readonly unknown[]
  notify(text: string, kind?: Toast['kind'], action?: Toast['action']): void
}

// Экспорт закрыт: листов нет и выгрузка не идёт. Идущую выгрузку окно хода обязано показывать
// и при пустом альбоме (удалили все развороты) — иначе «Остановить» не достать.
export const exportShut = (sheets: number, exporting: boolean) => sheets === 0 && !exporting

// Фоновые работы Go: экспорт и его окна (события export:*) и сбор проекта в папку (collect:*, 069).
class Jobs {
  private host: JobsHost | null = null
  exporting = $state<{ done: number; total: number } | null>(null)
  // Окно «Экспорт» (009 U8) открыто.
  exportOpen = $state(false)
  // Памятка после выгрузки для типографии (047, 081): не null — открыто окно «Готово для …» (shell/export/PrintDone).
  exportMemo = $state<string[] | null>(null)
  exportNote = $state('') // сводка Go о пересчёте цвета к той же памятке
  exportPrint = $state<PrintPlan | null>(null) // план типографии в момент запуска выгрузки: заголовок, сайт и папка окна «Готово»
  exportStopping = $state(false) // «Остановить» нажато, export:done ещё не пришёл (073)
  private exportDone = 0 // сколько раз пришло export:done: событие может обогнать ответ Export
  // Сбор в папку (069): ход копирования (не null — идёт) и окно «Собрать проект в папку» (shell/collect).
  collecting = $state<CollectProgress | null>(null)
  collectOpen = $state(false)
  private collectDone = 0 // сколько раз пришло collect:done: событие может обогнать ответ Collect
  // Открытие большого альбома (073): прогрев раскладок у Go (open:progress); окно — shell/dialogs/OpenProgress.
  opening = $state<{ done: number; total: number } | null>(null)
  openShown = $state(false) // с первого события прошло OPEN_SHOW_AFTER — окно можно рисовать
  private openTimer: ReturnType<typeof setTimeout> | undefined

  // Store подключает себя при создании (store.svelte.ts).
  attach(host: JobsHost) {
    this.host = host
  }

  // Подписка на события Go; зовёт store.init().
  listen() {
    EventsOn('export:progress', (done: number, total: number) => (this.exporting = { done, total }))
    // note — сводка Go о пересчёте цвета (может быть пустой или отсутствовать у старого Go).
    // memo (047) — строки памятки типографии: вместо тоста открывается окно «Готово», тост нужен только при ошибке.
    // stopped (073) — остановили кнопкой: note готовой фразой, тост красный без «Открыть папку»; окно хода закрывается само.
    EventsOn('export:done', (_dir: string, err: string, note?: string, memo?: string[] | null, stopped?: boolean) => {
      const total = this.exporting?.total
      this.exportDone++
      this.exporting = null
      this.exportStopping = false
      this.exportOpen = false
      if (err) this.host?.notify('Экспорт не удался: ' + errText(err), 'error')
      else if (stopped) this.host?.notify(exportStoppedText(note), 'error')
      else if (memo?.length) {
        this.exportMemo = memo
        this.exportNote = note ?? ''
      } else
        this.host?.notify(exportDoneText(total, note), 'info', {
          label: 'Открыть папку',
          run: () => Api.ShowInExplorer(),
        })
    })
    // Последнее событие Go — всегда (total, total): оно же гасит окно.
    EventsOn('open:progress', (done: number, total: number) => {
      if (total <= 0 || done >= total) return this.endOpening()
      if (!this.opening) this.openTimer = setTimeout(() => (this.openShown = this.opening !== null), OPEN_SHOW_AFTER)
      this.opening = { done, total }
    })
    EventsOn('collect:progress', (doneFiles: number, totalFiles: number, doneBytes: number, totalBytes: number) => {
      this.collecting = { doneFiles, totalFiles, doneBytes, totalBytes }
    })
    // Окно закрывается само: итог — тост (ошибку Go пишет готовой фразой, остановку и успех — в note).
    EventsOn('collect:done', (_dir: string, err: string, note: string, stopped: boolean) => {
      this.collectDone++
      this.collecting = null
      this.collectOpen = false
      const r = collectDoneText(err ?? '', note ?? '', !!stopped)
      this.host?.notify(r.text, r.kind, r.open ? { label: 'Открыть папку', run: () => void Api.ShowCollected() } : undefined)
    })
  }

  // Окно открытия гаснет: (total, total) от Go, ответ GetState/Open (смена альбома) или ошибка открытия.
  endOpening() {
    clearTimeout(this.openTimer)
    this.opening = null
    this.openShown = false
  }

  // «Экспорт» открывает окно (ExportDialog): настройки, а пока выгрузка идёт — её ход (073); сам запуск — runExport.
  exportAll() {
    if (!this.host || exportShut(this.host.sheets.length, !!this.exporting)) return
    this.exportOpen = true
  }

  // «Остановить» в окне хода: Go оборвёт выгрузку после текущего разворота и пришлёт export:done со stopped.
  stopExport() {
    if (!this.exporting || this.exportStopping) return
    this.exportStopping = true
    void Api.CancelExport()
  }

  // «Файл ▸ Собрать проект в папку…» открывает окно настроек (CollectDialog); пока сбор идёт — нет.
  collectShow() {
    if (!this.collecting) this.collectOpen = true
  }

  // Запуск сбора (069). started — копирование пошло; started=false и error='' — отказ пользователя
  // (вопрос «Нет», диалог папки закрыт); error — текст Go до копирования (окно остаётся открытым, ошибку рисует оно).
  async runCollect(all: boolean): Promise<{ started: boolean; error: string }> {
    const seen = this.collectDone
    try {
      const started = await Api.Collect({ all })
      // Пустой альбом копируется мгновенно: collect:done мог прийти раньше ответа — полосу уже не рисуем.
      // collect:progress мог прийти раньше ответа — его счётчик заглушкой не затираем.
      if (started && this.collectDone === seen) {
        if (!this.collecting) this.collecting = { doneFiles: 0, totalFiles: 0, doneBytes: 0, totalBytes: 0 }
        this.collectOpen = true
      }
      return { started, error: '' }
    } catch (e) {
      return { started: false, error: capFirst(errText(e)) }
    }
  }

  // true — экспорт пошёл; false — отказ (диалог папки закрыт, вопрос «Нет») или ошибка: окно остаётся открытым.
  async runExport(opt: ExportOpts): Promise<boolean> {
    const seen = this.exportDone
    try {
      const started = await Api.Export(opt)
      // total неизвестен (диапазон, «по страницам»): считает Go, первое export:progress его пришлёт.
      // Мгновенная выгрузка: export:done мог прийти раньше ответа — ход уже не рисуем.
      if (started && this.exportDone === seen) this.exporting = this.exporting ?? { done: 0, total: 0 }
      return started
    } catch (e) {
      this.host?.notify(errText(e), 'error')
      return false
    }
  }
}

export const jobs = new Jobs()
