import * as Api from '../../../wailsjs/go/app/App'
import { EventsOn } from '../../../wailsjs/runtime/runtime'
import { collectDoneText, errText, exportDoneText } from '../words/errtext'
import type { Toast } from './toasts.svelte'
import type { ExportOpts } from './types'

// Ход сбора в папку: файлов и байт скопировано из всего к копированию (уже лежавшие не считаются).
export interface CollectProgress {
  doneFiles: number
  totalFiles: number
  doneBytes: number
  totalBytes: number
}

// Что фоновым работам нужно от Store (структурный тип: цикла импортов со store нет).
export interface JobsHost {
  readonly sheets: readonly unknown[]
  notify(text: string, kind?: Toast['kind'], action?: Toast['action']): void
}

// Фоновые работы Go: экспорт и его окна (события export:*) и сбор проекта в папку (collect:*, 069).
class Jobs {
  private host: JobsHost | null = null
  exporting = $state<{ done: number; total: number } | null>(null)
  // Окно «Экспорт» (009 U8) открыто.
  exportOpen = $state(false)
  // Памятка после выгрузки для Фотосферы Pro (047): не null — открыто окно «Готово для Фотосферы» (shell/export/FsferaDone).
  exportMemo = $state<string[] | null>(null)
  exportNote = $state('') // сводка Go о пересчёте цвета к той же памятке
  exportFolder = $state('') // имя папки книги из плана в момент запуска выгрузки (окно «Готово»)
  // Сбор в папку (069): ход копирования (не null — идёт) и окно «Собрать проект в папку» (shell/collect).
  collecting = $state<CollectProgress | null>(null)
  collectOpen = $state(false)
  private collectDone = 0 // сколько раз пришло collect:done: событие может обогнать ответ Collect

  // Store подключает себя при создании (store.svelte.ts).
  attach(host: JobsHost) {
    this.host = host
  }

  // Подписка на события Go; зовёт store.init().
  listen() {
    EventsOn('export:progress', (done: number, total: number) => (this.exporting = { done, total }))
    // note — сводка Go о пересчёте цвета (может быть пустой или отсутствовать у старого Go).
    // memo (047) — строки памятки для Фотосферы: вместо тоста открывается окно «Готово», тост нужен только при ошибке.
    EventsOn('export:done', (_dir: string, err: string, note?: string, memo?: string[] | null) => {
      const total = this.exporting?.total
      this.exporting = null
      if (err) this.host?.notify('Экспорт не удался: ' + errText(err), 'error')
      else if (memo?.length) {
        this.exportMemo = memo
        this.exportNote = note ?? ''
      } else
        this.host?.notify(exportDoneText(total, note), 'info', {
          label: 'Открыть папку',
          run: () => Api.ShowInExplorer(),
        })
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

  // «Экспорт» открывает окно настроек (ExportDialog); сам запуск — runExport.
  exportAll() {
    if (this.exporting || !this.host || this.host.sheets.length === 0) return
    this.exportOpen = true
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
      return { started: false, error: errText(e) }
    }
  }

  // true — экспорт пошёл; false — отказ (диалог папки закрыт, вопрос «Нет») или ошибка: окно остаётся открытым.
  async runExport(opt: ExportOpts): Promise<boolean> {
    try {
      const started = await Api.Export(opt)
      // total неизвестен (диапазон, «по страницам»): считает Go, первое export:progress его пришлёт.
      if (started) this.exporting = { done: 0, total: 0 }
      return started
    } catch (e) {
      this.host?.notify(errText(e), 'error')
      return false
    }
  }
}

export const jobs = new Jobs()
