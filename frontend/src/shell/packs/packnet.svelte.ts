import { Api, store } from '../../base/core/store.svelte'
import { errText, capFirst } from '../../base/words/errtext'
import { READ_ONLY_HINT } from '../../panels/decor/decor'
import {
  BACKGROUND_TEXT,
  CANCELLED_TEXT,
  NO_NET,
  REVOKED_TEXT,
  isCancelled,
  isGone,
  oneText,
  oneTone,
  resetText,
  startAction,
  startText,
  updateText,
  updateTone,
  type LibraryProgress,
  type LibraryStart,
  type LibraryUpdate,
  type NetAction,
} from './packnet'
import { BUSY_HINT, type LibraryNet, type PackAvail } from './packs'

// Что сети окна нужно от самого окна «Наборы» (структурный тип: цикла импортов с packs.svelte нет).
export interface NetHost {
  open: boolean
  goBusy: boolean
  readonly locked: boolean
  readonly readOnly: boolean
  readonly noNet: boolean // ни одного набора из сети не стоит (список уже читался)
  load(): Promise<void>
  showAvail(): Promise<void> // окно «Наборы» открыто и прокручено к «Можно скачать»
  reveal: boolean
  listUpdated(): void // «Обновить» дал список: открытые «Фоны…»/«Макеты» докачивают образцы и перечитывают каталог
}

const POLL_MS = 2000 // чужая операция (проверка при запуске) идёт: окно перечитывает «занято», пока не кончится

// Сеть библиотеки (067 Ф3, 137): согласие, «Обновить библиотеку», «Скачать» в строке и «Скачать все», ход и «Отменить»,
// «Проверять при запуске», «Не обращаться к сети». Сам обмен — Go: без согласия он в сеть не ходит и отказывает ошибкой.
// Правок альбома и шагов отмены нет; итоги — тосты (выше окон), ошибки — тостом (решение 0.16 №40).
export class LibNet {
  net = $state<LibraryNet>({ ...NO_NET })
  updating = $state(false) // наше «Обновить библиотеку» идёт
  cancelling = $state(false) // «Отменить» нажата, ждём ответа Go
  progress = $state<LibraryProgress | null>(null)
  consent = $state(false) // открыт вопрос «Обратиться к GitHub?»
  consentStart = $state(false) // флажок «Проверять при запуске» в вопросе
  pending = $state<NetAction>({ kind: 'update' }) // что делать после согласия: слово на кнопке вопроса — по нему
  private req = 0
  revokes = 0 // сколько раз нажато «Не обращаться к сети»: ролики F1 по смене числа узнают об отзыве во время своей загрузки
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

  // library:start: итог проверки при запуске одним тостом (Toasts держит один тост — два подряд стёрли бы друг друга).
  // Кнопка: есть предложенные обновления — «Скачать» (согласие уже есть: событие шлёт Go только с ним), иначе «Показать» новые.
  started(s: LibraryStart) {
    const text = startText(s)
    if (!text) return
    const act = startAction(s)
    store.notify(
      text,
      'info',
      act === 'download'
        ? { label: 'Скачать', run: () => void this.guardedRun() }
        : act === 'show'
          ? { label: 'Показать', run: () => void this.host.showAvail() }
          : undefined,
    )
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
    this.begin({ kind: 'update' })
  }

  // «Скачать все (N, M МБ)» в разделе «Можно скачать».
  downloadAll() {
    this.begin({ kind: 'all' })
  }

  // «Скачать» в строке раздела; серая (why) и занятая библиотека сюда не доходят — кнопка отключена, но и тут проверка.
  download(a: Pick<PackAvail, 'id' | 'title' | 'why'>) {
    if (a.why) return
    this.begin({ kind: 'one', id: a.id, title: a.title })
  }

  // Общий вход трёх кнопок: нет согласия — тот же вопрос «Обратиться к GitHub?», слово кнопки — по действию (137-12).
  private begin(act: NetAction) {
    if (this.host.locked || this.host.readOnly) return
    if (!this.net.allowed) {
      this.pending = act
      this.consentStart = false
      this.consent = true
      return
    }
    void this.run(act)
  }

  // Флажок «Проверять при запуске»: проверка без согласия не работает — флажок сначала спрашивает его.
  async setStart(on: boolean) {
    if (!this.net.allowed) {
      this.pending = { kind: 'update' } // флажок — про проверку при запуске: после согласия обновляем, как раньше
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

  // «Разрешить и обновить» / «Разрешить и скачать»: согласие и флажок — в config.json у Go, затем то, что нажали.
  async confirmConsent() {
    if (!this.consent || this.updating) return
    const start = this.consentStart
    const act = this.pending
    this.consent = false
    if (this.blocked()) return
    try {
      this.take((await Api.SetLibraryNet(true, start)) as unknown as LibraryNet)
    } catch (e) {
      return store.notify(capFirst(errText(e)), 'error')
    }
    await this.run(act)
  }

  // «Не обращаться к сети»: снимает согласие и флажок, обрывает идущую загрузку (её ответ — «отменено»).
  async revoke() {
    this.revokes++
    if (this.updating) this.revoked = true
    try {
      this.take((await Api.SetLibraryNet(false, false)) as unknown as LibraryNet)
      store.notify(REVOKED_TEXT)
    } catch (e) {
      store.notify(capFirst(errText(e)), 'error')
    }
  }

  // «Вернуть» под списком: Go убирает пометку «удалён вами», «Скачать все» снова ставит эти наборы; пример встаёт сразу (demoBack).
  // Строки раздела перечитываются.
  async resetSkip() {
    if (this.host.locked || this.host.readOnly) return
    const was = this.net
    try {
      this.take((await Api.ResetLibrarySkip()) as unknown as LibraryNet)
      store.notify(resetText(was, this.net))
      if (this.host.open) void this.host.load()
    } catch (e) {
      // Служба занята (ErrBusy «библиотека уже обновляется»): пропуск не тронут, список перечитываем — «занято» видно в кнопках
      store.notify(capFirst(errText(e)), 'error')
      if (this.host.open) void this.host.load()
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

  // «Скачать и поставить» в галерее (144): тот же ход и тост, что у «Скачать» в строке, но итог возвращается вызывающему.
  // Ход виден полосой над плитками галереи (PackRun), поэтому тост «Скачиваем наборы…» не нужен. null — не встал или идёт другой ход.
  async fetchOne(a: Pick<PackAvail, 'id' | 'title'>): Promise<LibraryUpdate | null> {
    if (this.updating) {
      store.notify(BUSY_HINT) // плитки при ходе серые, но вызов мог прийти и мимо них
      return null
    }
    return this.run({ kind: 'one', id: a.id, title: a.title }, true)
  }

  // Ход (обновление, «Скачать все», «Скачать»): ход приходит событиями, итог — ответом. «Отменено» — не сбой, а обычный тост.
  // Возвращает ответ Go; сбой, отмена и занятость — null. inView — ход виден вызывающему окну: тост «Скачиваем…» не нужен.
  async run(act: NetAction = { kind: 'update' }, inView = false): Promise<LibraryUpdate | null> {
    if (this.updating) return null
    this.updating = true
    this.cancelling = false
    this.revoked = false
    this.progress = { stage: 'catalog', title: '', done: 0, total: 0 }
    if (!this.host.open && !inView) store.notify(BACKGROUND_TEXT) // окна нет — иначе ход нигде не виден
    let more = false // в разделе есть что скачать — окно прокрутится к нему
    let got: LibraryUpdate | null = null
    try {
      const r = (await this.call(act)) as unknown as LibraryUpdate
      got = r
      if (act.kind === 'one') store.notify(oneText(r, act.title), oneTone(r))
      else {
        store.notify(updateText(r, { open: this.host.open, noNet: this.host.noNet, kind: act.kind }), updateTone(r))
        more = (r.available ?? 0) > 0
        if (act.kind === 'update') this.host.listUpdated() // library:changed без установок не приходит
      }
    } catch (e) {
      if (isCancelled(e)) {
        if (!this.revoked) store.notify(CANCELLED_TEXT) // при отзыве согласия тост отзыва уже сказал главное
      } else store.notify(capFirst(errText(e)), isGone(e) ? 'info' : 'error')
    } finally {
      this.updating = false
      this.revoked = false
      this.progress = null
      this.cancelling = false
      if (this.host.open) {
        await this.host.load() // список, согласие и «занято» — как их видит Go
        if (more) this.host.reveal = true
      }
    }
    return got
  }

  private call(act: NetAction): Promise<unknown> {
    if (act.kind === 'all') return Api.DownloadAllLibrary()
    if (act.kind === 'one') return Api.DownloadLibraryPack(act.id)
    return Api.UpdateLibrary()
  }
}
