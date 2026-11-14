import { Api, store } from '../../base/core/store.svelte'
import { jobs } from '../../base/core/jobs.svelte'
import { capFirst } from '../../base/words/errtext'
import { edit } from '../../editor/text/edit.svelte'
import { folderStub, infoStale, loadAll, saveAll, type CollectInfo } from './collect'

// Окно «Собрать проект в папку» (спека 069): сводка Go по двум вариантам и запуск. Сам ход копирования и
// итог — в jobs (collecting, collect:done); окно открыто, пока jobs.collectOpen.
class CollectUi {
  all = $state(loadAll()) // «Все фото из папок альбома»
  album = $state<CollectInfo | null>(null) // сводка «Только фото альбома»
  full = $state<CollectInfo | null>(null) // сводка «Все фото из папок»
  busy = $state(false) // вопросы и выбор папки у Go
  error = $state('') // отказ Go до копирования: тост под затемнением не виден — показываем в окне
  private req = 0

  // Сводка выбранного варианта.
  info = $derived(this.all ? this.full : this.album)

  // Каждое открытие — заново: выбор из localStorage, прежние ответы и ошибка не показываются.
  reset() {
    this.req++
    this.all = loadAll()
    this.album = null
    this.full = null
    this.busy = false
    this.error = ''
  }

  // Обе сводки сразу: у каждого варианта в окне свои числа. Номер запроса отсекает запоздавший ответ.
  async refresh() {
    const my = ++this.req
    try {
      const [a, f] = await Promise.all([Api.CollectInfo(false), Api.CollectInfo(true)])
      if (infoStale(my, this.req)) return
      this.album = a as CollectInfo
      this.full = f as CollectInfo
    } catch {
      // Без сводки окно работает: вариант виден, «Собрать…» доступно (числа покажет Go в вопросах).
    }
  }

  setAll(all: boolean) {
    this.all = all
    this.error = ''
    saveAll(all)
  }

  close() {
    if (jobs.collecting) return // идёт копирование: закрывает только «Остановить»
    jobs.collectOpen = false
  }

  stop() {
    void Api.CancelCollect()
  }

  // «Собрать…»: отказ пользователя оставляет окно как есть; ошибка Go до копирования — в окно; ход — в jobs.
  async go() {
    if (this.busy || jobs.collecting) return
    this.busy = true
    this.error = ''
    try {
      await edit.idle() // набранный текст попадает в файл альбома
      const r = await jobs.runCollect(this.all)
      if (r.error) this.error = capFirst(r.error)
    } finally {
      this.busy = false
    }
  }

  // Имя папки в подсказке — из ответа Go; пока ответа нет (или он не пришёл) — заглушка.
  get folder(): string {
    return (this.album ?? this.full)?.folderName || folderStub(!!store.view.path, store.projectName)
  }
}

export const collectUi = new CollectUi()
