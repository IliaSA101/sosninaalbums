import { Api, store } from '../../base/core/store.svelte'
import { errText, capFirst } from '../../base/words/errtext'
import { decor } from '../../panels/decor/decor.svelte'
import { looks } from '../looks/looks.svelte'
import { getApi } from '../../panels/decorbrowse/getapi'
import {
  addedText,
  cancelled,
  hasNet,
  notAddedText,
  removeNote,
  removeQuestion,
  removedText,
  replaceQuestion,
  sortPacks,
  type LibraryOffer,
  type PackAdd,
  type PackAsk,
  type PackAvail,
  type PackInfo,
  type PackList,
  type PackRemove,
} from './packs'
import { LibNet } from './packnet.svelte'

// Окно «Файл ▸ Наборы библиотеки…» (067 Ф2): список наборов, «Добавить из файла…», «Удалить». Открывается и поверх окон
// «Фоны», «Стикеры», «Развороты» (кнопка «Наборы…»). Наборы не часть альбома: правок проекта и шагов отмены нет.
// Итоги — тосты (они выше окон); окно хранит только ошибку чтения списка. Сеть (Ф3) — `net` (packnet.svelte.ts).
class PacksUi {
  open = $state(false)
  packs = $state<PackInfo[]>([])
  available = $state<PackAvail[]>([]) // «Можно скачать» (137): строки от Go
  all = $state<LibraryOffer>({ count: 0, bytes: 0 }) // «Скачать все (N, M МБ)»
  availAt = $state(0) // unix мс списка; 0 — первой проверки ещё не было
  listed = $state(false) // список наборов хоть раз читался (packs не пуст по умолчанию, а не «ещё не знаем»)
  reveal = $state(false) // прокрутить окно к «Можно скачать» (после «Обновить», тост «Показать»)
  loading = $state(false)
  busy = $state(false) // наша операция идёт
  goBusy = $state(false) // Go занят (обновление библиотеки): кнопки серые с подсказкой
  readOnly = $state(false)
  error = $state('')
  ask = $state<PackAsk | null>(null)
  readonly net = new LibNet(this)
  private req = 0

  get locked(): boolean {
    return this.busy || this.goBusy || this.net.updating
  }

  async show() {
    this.open = true
    this.error = ''
    this.ask = null
    this.packs = []
    this.available = []
    this.all = { count: 0, bytes: 0 }
    this.availAt = 0
    await this.load(true)
  }

  // «Показать» в тосте запуска: окно «Наборы» (откроем, если закрыто) прокручено к «Можно скачать».
  async showAvail() {
    if (!this.open) await this.show()
    this.reveal = true
  }

  // Ни одного набора из сети не стоит (список уже читался): итог «Обновить» тогда «Список наборов получен» (137-22).
  get noNet(): boolean {
    return this.listed && !hasNet(this.packs)
  }

  close() {
    this.open = false
    this.reveal = false
    this.ask = null
    this.net.stop()
    this.req++ // запоздавший ответ списка окну не нужен
  }

  // Список заново (открытие, library:changed). quiet — без мигания «Читаем список…» и без сброса ошибки.
  async load(first = false) {
    const my = ++this.req
    if (first) this.loading = true
    try {
      this.take((await Api.LibraryPacks()) as unknown as PackList, my)
      if (my === this.req) this.error = ''
    } catch (e) {
      if (my === this.req) this.error = capFirst(errText(e))
    } finally {
      if (my === this.req) this.loading = false
    }
  }

  private take(list: PackList, my = this.req) {
    if (my !== this.req) return
    this.packs = sortPacks(list.packs ?? [])
    this.available = list.available ?? []
    this.all = list.all ?? { count: 0, bytes: 0 }
    this.availAt = list.availAt ?? 0
    this.listed = true
    this.goBusy = !!list.busy
    this.readOnly = !!list.readOnly
    this.net.take(list.net)
    this.net.busy(this.goBusy)
  }

  // library:changed от Go (набор добавлен, удалён, обновлён): список и каталог открытого окна «Фоны/Стикеры/Развороты».
  // installed — названия поставленных наборов (при удалении пусто): следующее открытие окон встанет на их тему (073-1).
  changed(installed?: readonly string[] | null) {
    decor.note(installed)
    if (store.view.look) void store.resync() // View.look.found зависит от библиотеки: вид сам не придёт (131-116)
    looks.changed(installed) // окно «Макеты альбома»: открытое перечитывает каталог, закрытое встанет на тему свежего набора
    if (this.open) void this.load()
    if (decor.cur) void decor.refresh()
  }

  // library:previews от Go (144): докачаны листы образцов — открытые «Фоны…»/«Макеты альбома» перечитывают каталог (темы не меняют).
  // FetchPackPreviews отсюда не звать: докачка шлёт это же событие — выйдет петля.
  previews() {
    if (decor.cur) void decor.refresh()
    looks.changed()
  }

  // «Обновить» дал список без установок (library:changed не шлётся): строка «Можно скачать» и приглашение открытых окон
  // меняются сразу, образцы докачиваются. Это действие человека, а не library:previews — петли нет.
  listUpdated() {
    if (!decor.cur && !looks.open) return
    getApi.previews()
    this.previews()
  }

  // «Добавить из файла…»: диалог — в Go; закрыли без выбора — молча.
  async addFile() {
    if (this.locked || this.readOnly) return
    this.busy = true
    try {
      this.handle((await Api.AddPackFile()) as unknown as PackAdd, false)
    } catch (e) {
      store.notify(capFirst(errText(e)), 'error')
    } finally {
      this.busy = false
    }
  }

  // То же без диалога — путь к .zip (проверка из консоли браузера, где нативные диалоги не вызываются).
  async addPath(path: string) {
    if (this.locked || this.readOnly) return
    this.busy = true
    try {
      this.handle((await Api.AddPackPath(path)) as unknown as PackAdd, false)
    } catch (e) {
      store.notify(capFirst(errText(e)), 'error')
    } finally {
      this.busy = false
    }
  }

  private handle(r: PackAdd, replaced: boolean) {
    if (cancelled(r)) return
    if (r.ask === 'update') {
      this.ask = { kind: 'replace', token: r.token, title: r.pack.title, text: replaceQuestion(r) }
      return
    }
    const note = notAddedText(r)
    if (note) return store.notify(note)
    store.notify(addedText(r, replaced))
    void this.load()
  }

  dismiss() {
    this.ask = null
  }

  // «Заменить» в вопросе: Go ставит новую версию (вопрос устарел через 10 минут — ошибка текстом в тосте).
  async confirmReplace() {
    const a = this.ask
    if (a?.kind !== 'replace' || this.busy) return
    this.ask = null
    this.busy = true
    try {
      this.handle((await Api.ConfirmPack(a.token)) as unknown as PackAdd, true)
    } catch (e) {
      store.notify(capFirst(errText(e)), 'error')
    } finally {
      this.busy = false
    }
  }

  // «Показать папку»: проводник открывает Go; не занимает кнопки окна (ничего не меняет), ошибка — тостом.
  async showFolder() {
    try {
      await Api.ShowLibraryFolder()
    } catch (e) {
      store.notify(capFirst(errText(e)), 'error')
    }
  }

  // «Удалить» у строки: числа для вопроса считает Go (что освободится и что останется).
  async askRemove(p: PackInfo) {
    if (this.locked || this.readOnly || !p.removable) return
    this.busy = true
    try {
      const plan = (await Api.PlanRemovePack(p.id)) as unknown as PackRemove
      if (!this.open) return // окно закрыли, пока считалось
      this.ask = { kind: 'remove', id: p.id, title: p.title, text: removeQuestion(p, plan.addons), note: removeNote(plan) }
    } catch (e) {
      store.notify(capFirst(errText(e)), 'error')
    } finally {
      this.busy = false
    }
  }

  async confirmRemove() {
    const a = this.ask
    if (a?.kind !== 'remove' || this.busy) return
    this.ask = null
    this.busy = true
    try {
      this.take((await Api.RemovePack(a.id)) as unknown as PackList)
      store.notify(removedText(a.title))
    } catch (e) {
      store.notify(capFirst(errText(e)), 'error')
      void this.load()
    } finally {
      this.busy = false
    }
  }
}

export const packsUi = new PacksUi()
