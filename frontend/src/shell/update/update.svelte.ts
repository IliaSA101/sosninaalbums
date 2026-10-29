import { store } from '../../base/core/store.svelte'
import { capFirst, errText } from '../../base/words/errtext'
import { STAND_LINK_TEXT } from '../dialogs/about.svelte'
import { NO_UPDATE, UpdateGo, type UpdateInfo, type UpdateProgress } from './updateapi'
import {
  PAGE_LABEL,
  WHATS_NEW_LABEL,
  failedText,
  justText,
  offerToast,
  readyToast,
  retryAct,
  startedText,
  type UpdateAct,
} from './updatetext'

// Что нужно обновлению от верха программы (хук ставит UpdateWatch; тесту хватает пустого).
export interface UpdateHost {
  whatsNew(): void
}

// Обновление (111): состояние блока «О программе» и тосты. Обмен, подпись и установка — Go; здесь только показ.
// Правок альбома и шагов отмены нет; сбои — строкой в блоке или тостом.
export class UpdateUi {
  info = $state<UpdateInfo>({ ...NO_UPDATE })
  notesOpen = $state(false) // раскрыт «Что нового ▸»
  cancelling = $state(false) // «Отменить» нажата, ждём конца скачивания
  retry = $state<UpdateAct | undefined>(undefined) // что повторяет «Повторить» после ошибки; не задано — по данным Go
  host: UpdateHost = { whatsNew: () => {} }
  private req = 0
  private dl = false // наш DownloadUpdate ещё не вернулся
  private installing = false
  private offerShown = '' // версия, о которой тост уже был (событие и опрос при запуске не дублируются)

  // Принять состояние от Go (ответ метода или update:state). Первое «готово» — тост «Перезапустить».
  take(next: UpdateInfo) {
    const was = this.info.phase
    this.info = { ...NO_UPDATE, ...next }
    if (next.phase === 'error') {
      // Go присылает ошибку со строчной буквы (как errText).
      this.info.error = capFirst(this.info.error)
      this.retry = this.retryFor(was)
    } else this.retry = undefined
    if (next.phase !== 'downloading') this.cancelling = false
    if (next.phase !== 'available') this.notesOpen = false
    if (next.phase === 'ready' && was !== 'ready' && next.mode !== 'page') {
      const t = readyToast(next)
      store.notify(t.text, 'info', { label: t.label, run: () => void this.install() })
    }
  }

  // Запуск программы: состояние без сети и итог прошлого обновления (спрашиваем, а не ждём события:
  // событие ушло бы до того, как интерфейс подписался).
  async start() {
    let i: UpdateInfo
    try {
      i = await UpdateGo.UpdateInfo()
    } catch {
      return // не в окне Wails или Go без обновления: блок покажет «Проверить»
    }
    this.take(i)
    if (i.failed) store.notify(failedText(i.failed), 'error', { label: PAGE_LABEL, run: () => void this.openPage() })
    else if (i.just) store.notify(justText(i.just), 'info', { label: WHATS_NEW_LABEL, run: () => this.host.whatsNew() })
    // Проверка при запуске могла кончиться раньше, чем интерфейс подписался на update:offer: тост показываем по опросу.
    else if (i.atStart && i.phase === 'available') this.offered(i)
  }

  // update:offer — проверка при запуске нашла новее.
  offered(i: UpdateInfo) {
    if (i.phase !== 'available' || !i.version) return
    this.take(i)
    if (this.offerShown === i.version) return
    this.offerShown = i.version
    const t = offerToast(i)
    const run = t.act === 'page' ? () => void this.openPage() : () => void this.fromToast()
    store.notify(t.text, 'info', { label: t.label, run })
  }

  // update:state — смена фазы.
  changed(i: UpdateInfo) {
    this.take(i)
  }

  // update:progress — не чаще 4 раз в секунду. Ход без нашего вызова (начали из тоста до открытия окна) тоже показываем.
  progressed(p: UpdateProgress) {
    const ph = this.info.phase
    if (ph === 'ready' || ph === 'error' || ph === 'checking') return
    this.info = { ...this.info, phase: 'downloading', done: p.done, total: p.total }
  }

  // «Обновить» из тоста: скачивание в фоне, окно не открывается.
  private async fromToast() {
    if (this.dl) return
    store.notify(startedText(this.info.version))
    await this.download()
  }

  // Кнопки блока и тостов — одно место входа.
  act(a: UpdateAct) {
    switch (a) {
      case 'check':
        return this.check()
      case 'download':
        return this.download()
      case 'cancel':
        return this.cancel()
      case 'install':
        return this.install()
      case 'page':
        return this.openPage()
    }
  }

  // Что не удалось, то и повторяем: ошибка во время скачивания — скачать, во время проверки — проверить.
  // Иначе (ошибка пришла событием, прежней фазы не знаем) — по данным Go: version от старого предложения не различает.
  private retryFor(was: UpdateInfo['phase']): UpdateAct {
    if (this.info.mode === 'page') return 'check'
    if (was === 'downloading' || this.dl) return 'download'
    if (was === 'checking') return 'check'
    return retryAct(this.info)
  }

  private fail(e: unknown) {
    const was = this.info.phase
    this.info = { ...this.info, phase: 'error', error: capFirst(errText(e)) }
    this.retry = this.retryFor(was)
  }

  private async refresh() {
    try {
      this.take(await UpdateGo.UpdateInfo())
    } catch {
      // состояние остаётся прежним
    }
  }

  // «Проверить обновления»: всегда в сеть (флажок не мешает — это действие самого человека).
  async check() {
    const ph = this.info.phase
    if (ph === 'checking' || ph === 'downloading') return
    this.info = { ...this.info, phase: 'checking', error: '' }
    try {
      this.take(await UpdateGo.CheckUpdate())
    } catch (e) {
      this.fail(e)
    }
  }

  // «Обновить»: ход приходит событиями, итог — ответом. Повторный щелчок, пока идёт скачивание, ничего не делает.
  async download() {
    if (this.dl) return
    this.dl = true
    this.cancelling = false
    this.info = { ...this.info, phase: 'downloading', done: 0, total: 0, error: '' }
    try {
      this.take(await UpdateGo.DownloadUpdate())
    } catch (e) {
      // Отмена ошибкой не приходит: Go отвечает снимком прежнего предложения (take выше).
      this.fail(e)
    } finally {
      this.dl = false
      this.cancelling = false
    }
  }

  // «Отменить»: Go обрывает скачивание, .part удаляется.
  async cancel() {
    if (this.info.phase !== 'downloading' || this.cancelling) return
    this.cancelling = true
    try {
      await UpdateGo.CancelUpdate()
    } catch (e) {
      this.cancelling = false
      return store.notify(capFirst(errText(e)), 'error')
    }
    if (!this.dl) await this.refresh() // скачивание началось не здесь: ответа метода не будет, ждать нечего
  }

  // «Перезапустить и обновить»: Go сохранит альбом, запустит установщик и закроет программу.
  async install() {
    if (this.info.phase !== 'ready' || this.installing) return
    this.installing = true
    try {
      await UpdateGo.InstallUpdate()
    } catch (e) {
      this.installing = false
      store.notify(capFirst(errText(e)), 'error')
    }
  }

  // На стенде QA Go браузер не открывает (osx quiet) — тост вместо молчания, как в «О программе».
  async openPage() {
    if (store.stand) return store.notify(STAND_LINK_TEXT)
    try {
      await UpdateGo.OpenUpdatePage()
    } catch (e) {
      store.notify(capFirst(errText(e)), 'error')
    }
  }

  // Флажок «Проверять при запуске»: сразу в блоке, при ошибке Go возвращается прежнее.
  async setAtStart(on: boolean) {
    const my = ++this.req
    const prev = this.info.atStart
    this.info = { ...this.info, atStart: on }
    try {
      const i = await UpdateGo.SetUpdateCheck(on)
      if (my === this.req) this.take(i)
    } catch (e) {
      if (my === this.req) this.info = { ...this.info, atStart: prev }
      store.notify(capFirst(errText(e)), 'error')
    }
  }
}

export const updateUi = new UpdateUi()
