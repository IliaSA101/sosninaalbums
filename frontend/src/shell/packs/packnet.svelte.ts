import { Api, store } from '../../base/core/store.svelte'
import { errText, capFirst } from '../../base/words/errtext'
import { READ_ONLY_HINT } from '../../panels/decor/decor'
import {
  BACKGROUND_TEXT,
  CANCELLED_TEXT,
  NO_NET,
  REVOKED_TEXT,
  SKIP_RESET_TEXT,
  isCancelled,
  offerText,
  updateText,
  type LibraryOffer,
  type LibraryProgress,
  type LibraryUpdate,
} from './packnet'
import type { LibraryNet } from './packs'

// Что сети окна нужно от самого окна «Наборы» (структурный тип: цикла импортов с packs.svelte нет).
export interface NetHost {
  open: boolean
  goBusy: boolean
  readonly locked: boolean
  readonly readOnly: boolean
  load(): Promise<void>
}

const POLL_MS = 2000 // чужая операция (проверка при запуске) идёт: окно перечитывает «занято», пока не кончится

// Сеть библиотеки (067 Ф3): согласие, «Обновить библиотеку», ход и «Отменить», «Проверять при запуске»,
// «Не обращаться к сети». Сам обмен — Go: без согласия он в сеть не ходит и отказывает ошибкой.
// Правок альбома и шагов отмены нет; итоги — тосты (выше окон), ошибки — тостом (решение 0.16 №40).
export class LibNet {
  net = $state<LibraryNet>({ ...NO_NET })
  updating = $state(false) // наше «Обновить библиотеку» идёт
  cancelling = $state(false) // «Отменить» нажата, ждём ответа Go
  progress = $state<LibraryProgress | null>(null)
  consent = $state(false) // открыт вопрос «Обратиться к GitHub?»
  consentStart = $state(false) // флажок «Проверять при запуске» в вопросе
  private req = 0
  private revoked = false // «Не обращаться к сети» нажато во время загрузки: «отменено» уже не новость
  private timer = 0

  constructor(private host: NetHost) {}

  // LibraryPacks().net: у старого Go поля нет — «согласия нет».
  take(n: LibraryNet | null | undefined) {
    this.req++ // ответ, посланный раньше, устарел
    this.net = { ...NO_NET, ...n }
  }

  // Go сообщил занятость: чужая операция кончилась — полоса уходит; идёт — слежение за ней.
  busy(busy: boolean) {
    if (busy && !this.updating) return this.poll()
    if (!busy && !this.updating) {
      this.progress = null
      this.cancelling = false
    }
  }

  private poll() {
    clearTimeout(this.timer)
    this.timer = window.setTimeout(() => {
      if (this.host.open) void this.host.load()
    }, POLL_MS)
  }

  // Окно закрыли: вопрос и слежение не нужны; идущая загрузка не прерывается (итог — тостом).
  stop() {
    clearTimeout(this.timer)
    this.consent = false
  }

  // library:progress (не чаще 4 раз в секунду). Чужая операция не оповещает о конце — её ведёт busy().
  progressed(p: LibraryProgress) {
    if (!this.updating) {
      if (!this.host.open) return
      this.host.goBusy = true
      this.poll()
    }
    this.progress = p
  }

  // library:offer при запуске: новое больше 100 МБ — тост с «Скачать» (согласие уже есть: предложение шлёт Go только с ним).
  offered(o: LibraryOffer) {
    store.notify(offerText(o), 'info', { label: 'Скачать', run: () => void this.guardedRun() })
  }

  // library:updated: проверка при запуске поставила наборы — тот же тост, что у кнопки (073-2), не красный.
  updated(r: LibraryUpdate) {
    store.notify(updateText(r))
  }

  // Входы мимо update() («Скачать» из тоста, согласие флажком) проверяют то же, что update():
  // только чтение — подсказка про более новую версию, занято — молча; Go в обоих случаях не зовётся.
  private blocked(): boolean {
    if (this.host.readOnly) {
      store.notify(READ_ONLY_HINT)
      return true
    }
    return this.host.locked
  }

  private guardedRun() {
    if (!this.blocked()) return this.run()
  }

  // «Обновить библиотеку»: нет согласия — сначала вопрос.
  update() {
    if (this.host.locked || this.host.readOnly) return
    if (!this.net.allowed) {
      this.consentStart = false
      this.consent = true
      return
    }
    void this.run()
  }

  // Флажок «Проверять при запуске»: проверка без согласия не работает — флажок сначала спрашивает его.
  async setStart(on: boolean) {
    if (!this.net.allowed) {
      this.consentStart = on
      this.consent = on
      return
    }
    const my = ++this.req
    try {
      const n = (await Api.SetLibraryNet(true, on)) as unknown as LibraryNet
      if (my === this.req) this.net = { ...NO_NET, ...n }
    } catch (e) {
      store.notify(capFirst(errText(e)), 'error')
    }
  }

  dismissConsent() {
    this.consent = false
  }

  // «Разрешить и обновить»: согласие и флажок — в config.json у Go, затем обновление.
  async confirmConsent() {
    if (!this.consent || this.updating) return
    const start = this.consentStart
    this.consent = false
    if (this.blocked()) return
    try {
      this.take((await Api.SetLibraryNet(true, start)) as unknown as LibraryNet)
    } catch (e) {
      return store.notify(capFirst(errText(e)), 'error')
    }
    await this.run()
  }

  // «Не обращаться к сети»: снимает согласие и флажок, обрывает идущую загрузку (её ответ — «отменено»).
  async revoke() {
    if (this.updating) this.revoked = true
    try {
      this.take((await Api.SetLibraryNet(false, false)) as unknown as LibraryNet)
      store.notify(REVOKED_TEXT)
    } catch (e) {
      store.notify(capFirst(errText(e)), 'error')
    }
  }

  // «Вернуть» под списком: Go убирает пропуск удалённых сетевых наборов, следующее обновление их вернёт.
  async resetSkip() {
    if (this.host.locked || this.host.readOnly) return
    try {
      this.take((await Api.ResetLibrarySkip()) as unknown as LibraryNet)
      store.notify(SKIP_RESET_TEXT)
    } catch (e) {
      store.notify(capFirst(errText(e)), 'error')
    }
  }

  // «Отменить» в полосе хода: Go обрывает скачивание, ответ UpdateLibrary придёт ошибкой «отменено».
  async cancel() {
    if (this.cancelling || (!this.updating && !this.progress)) return
    this.cancelling = true
    try {
      await Api.CancelLibrary()
    } catch (e) {
      this.cancelling = false
      store.notify(capFirst(errText(e)), 'error')
    }
  }

  // Обновление: ход приходит событиями, итог — ответом. «Отменено» — не сбой, а обычный тост.
  async run() {
    if (this.updating) return
    this.updating = true
    this.cancelling = false
    this.revoked = false
    this.progress = { stage: 'catalog', title: '', done: 0, total: 0 }
    if (!this.host.open) store.notify(BACKGROUND_TEXT) // окна нет — иначе ход нигде не виден
    try {
      store.notify(updateText((await Api.UpdateLibrary()) as unknown as LibraryUpdate))
    } catch (e) {
      if (isCancelled(e)) {
        if (!this.revoked) store.notify(CANCELLED_TEXT) // при отзыве согласия тост отзыва уже сказал главное
      } else store.notify(capFirst(errText(e)), 'error')
    } finally {
      this.updating = false
      this.revoked = false
      this.progress = null
      this.cancelling = false
      if (this.host.open) void this.host.load() // список, согласие и «занято» — как их видит Go
    }
  }
}
