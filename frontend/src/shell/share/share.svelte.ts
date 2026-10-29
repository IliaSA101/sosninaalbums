import { Api, store } from '../../base/core/store.svelte'
import { edit } from '../../editor/text/edit.svelte'
import { capFirst, errText } from '../../base/words/errtext'
import {
  ShareGo,
  type ShareBridge,
  type ShareChangeArgs,
  type ShareDone,
  type ShareOptions,
  type ShareProgress,
  type ShareView,
} from './shareapi'
import { COPIED, COPY_FAIL, DONE_TOAST, REVOKED_TOAST, STOPPED_TOAST, failedToast } from './sharetext'

const WAIT_DELAY_MS = 400

// Окно «Файл ▸ Отправить по ссылке…» (146): состояние от Go (ShareState и события share:*), решения — в Go.
// Выгрузка проект не меняет: правок альбома и шагов отмены нет. Образец хода — экспорт (jobs.svelte.ts), копирования — окно агентов.
export class ShareUi {
  bridge: ShareBridge = ShareGo
  open = $state(false)
  view = $state<ShareView | null>(null)
  progress = $state<ShareProgress | null>(null)
  stopping = $state(false) // «Остановить» нажата, share:done ещё не пришёл
  working = $state(false) // вызов Go в работе: кнопки серые
  error = $state('') // отказ подключения — под полем
  changeError = $state<{ part: 'password' | 'term'; text: string } | null>(null) // отказ пароля или срока — строка под своей строкой
  waiting = $state(false) // Go ещё не ответил на Share: возможно, ждёт ответа на вопрос в своём окне
  private waitTimer: ReturnType<typeof setTimeout> | undefined
  private req = 0
  private epoch = 0 // номер альбома в окне: ответ вызова, ушедшего для прежнего альбома, отбрасывается
  private albumKey = ''

  // Вид для окна: ответ Go; запоздавший ответ прежнего запроса отбрасывается номером.
  async refresh(): Promise<void> {
    const my = ++this.req
    try {
      const v = await this.bridge.ShareState()
      if (my === this.req) this.view = v
    } catch (e) {
      store.notify(capFirst(errText(e)), 'error')
    }
  }

  async show() {
    this.open = true
    this.error = ''
    this.changeError = null
    this.view = null // без прежнего вида: старый экран не мелькает, пока Go отвечает
    await this.refresh()
  }

  close() {
    this.open = false
    this.changeError = null
  }

  // Другой альбом при открытой странице (OpenRecent, «Открыть…»): вызов, ушедший для прежнего, мог не вернуться —
  // кнопки не должны оставаться серыми. Запоздавший ответ отбрасывается, вид перечитывается для нового альбома.
  watchAlbum(key: string) {
    if (key === this.albumKey) return
    const first = this.albumKey === ''
    this.albumKey = key
    if (first) return
    this.epoch++
    this.req++
    this.working = false
    this.unwait()
    this.error = ''
    this.changeError = null
    if (!this.open) return
    this.view = null // форма и поля заводятся заново из нового альбома
    void this.refresh()
  }

  private take(v: ShareView) {
    this.req++ // ответ метода новее всякого ShareState, ушедшего раньше
    this.view = v
  }

  // Ошибки Go пишутся со строчной буквы; в тосте — с заглавной.
  private async guard<T>(fn: () => Promise<T>): Promise<T | undefined> {
    if (this.working) return undefined
    const ep = this.epoch
    this.working = true
    try {
      const r = await fn()
      return ep === this.epoch ? r : undefined
    } catch (e) {
      if (ep !== this.epoch) return undefined
      store.notify(capFirst(errText(e)), 'error')
      await this.refresh() // у Go состояние могло не смениться
      return undefined
    } finally {
      if (ep === this.epoch) this.working = false
    }
  }

  async connect(url: string, token: string) {
    if (this.working) return
    const ep = this.epoch
    this.working = true
    this.error = ''
    try {
      const v = await this.bridge.ShareConnect(url.trim(), token)
      if (ep === this.epoch) this.take(v)
    } catch (e) {
      if (ep === this.epoch) this.error = capFirst(errText(e))
    } finally {
      if (ep === this.epoch) this.working = false
    }
  }

  async forget() {
    const v = await this.guard(() => this.bridge.ShareForget())
    if (v) {
      this.error = ''
      this.take(v)
    }
  }

  // «Выгрузить», «Обновить по ссылке», «Продолжить», «Выгрузить заново»: вопросы и ход ведёт Go. false — ответ «Нет» на вопрос.
  async start(o: ShareOptions) {
    await edit.idle() // набранное в тексте должно попасть в сцену
    if (this.working) return // повторный вызов: подсказку первого не трогаем
    this.wait()
    try {
      const started = await this.guard(() => this.bridge.Share(o))
      if (started) await this.refresh() // busy: окно покажет ход
    } finally {
      this.unwait()
    }
  }

  // Нативный вопрос Go (цвет, поля, шрифты…) может лежать за главным окном: подсказка «ответьте» появляется не сразу,
  // чтобы не мигать, когда вопросов нет. Отказ в вопросе даёт ответ Share false — форма снова активна (T5-11).
  private wait() {
    clearTimeout(this.waitTimer)
    this.waitTimer = setTimeout(() => (this.waiting = true), WAIT_DELAY_MS)
  }

  private unwait() {
    clearTimeout(this.waitTimer)
    this.waiting = false
  }

  stop() {
    if (!this.view?.busy || this.stopping) return
    this.stopping = true
    void this.bridge.CancelShare()
  }

  // true — Go принял правку: редактор пароля или срока можно закрыть; при отказе он остаётся с введённым.
  // Отказ (короткий пароль, срок сверх предела…) — строкой под своей строкой, как отказ подключения, без тоста.
  async change(c: ShareChangeArgs): Promise<boolean> {
    if (this.working) return false
    const ep = this.epoch
    this.working = true
    this.changeError = null
    try {
      const v = await this.bridge.ShareChange(c)
      if (ep !== this.epoch) return false
      this.take(v)
      return true
    } catch (e) {
      if (ep !== this.epoch) return false
      this.changeError = { part: c.setPassword ? 'password' : 'term', text: capFirst(errText(e)) }
      await this.refresh() // у Go состояние могло не смениться
      return false
    } finally {
      if (ep === this.epoch) this.working = false
    }
  }

  // Идёт своя выгрузка: пункт меню остаётся доступным, чтобы открыть ход (как «Экспорт…» при идущем экспорте).
  get sharing(): boolean {
    return !!this.view?.busy && !this.view.busyOther
  }

  // Вопрос «Отозвать ссылку?» задаёт Go; «Нет» — вид прежний.
  async revoke() {
    const had = !!this.view?.link
    const v = await this.guard(() => this.bridge.ShareRevoke())
    if (!v) return
    this.take(v)
    if (had && !v.link) store.notify(REVOKED_TOAST, 'info')
  }

  async invent(): Promise<string> {
    return (await this.guard(() => this.bridge.SharePassword())) ?? ''
  }

  // «Скопировать для клиента»: текст собирает Go, в буфер кладёт интерфейс (как окно агентов).
  async copy() {
    let text: string
    try {
      text = await this.bridge.ShareText()
    } catch (e) {
      store.notify(capFirst(errText(e)), 'error')
      return
    }
    try {
      await navigator.clipboard.writeText(text)
      store.notify(COPIED, 'info')
    } catch {
      // Текст отказа буфера приходит от браузера по-английски — свой.
      store.notify(COPY_FAIL, 'error')
    }
  }

  async openLink() {
    await this.guard(() => this.bridge.OpenShare())
  }

  // «Сохранить…» при несохранённом альбоме: диалог файла — у Go; после сохранения окно перечитывает состояние.
  async save() {
    await edit.idle()
    await store.run(Api.SaveProject())
    await this.refresh()
  }

  // share:progress. Окно открыли после старта — вид ещё без busy, перечитываем.
  progressed(p: ShareProgress) {
    this.progress = p
    if (!this.view?.busy) void this.refresh()
  }

  // share:done {err, stopped} — без ссылки (разбор С3): ссылку окно берёт из ShareState.
  // Ход и «Останавливаем…» держатся, пока Go не ответил новым видом: иначе на миг мелькает «Готовим… / Остановить».
  finished(d: ShareDone): Promise<void> {
    if (d.err) store.notify(failedToast(errText(d.err)), 'error')
    else if (d.stopped) store.notify(STOPPED_TOAST, 'error')
    else store.notify(DONE_TOAST, 'info')
    if (!this.open) {
      this.view = null // закрытое окно перечитает состояние при открытии; устаревшее busy меню не держим
      this.progress = null
      this.stopping = false
      return Promise.resolve()
    }
    return this.refresh().then(() => {
      this.progress = null
      this.stopping = false
    })
  }
}

export const share = new ShareUi()
