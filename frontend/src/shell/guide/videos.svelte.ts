// Ролики справочника F1 (спека 140, design §4): состояние окна поверх кэша роликов у Go. Правда о файлах — у Go
// (GuideVideos); здесь — ход скачивания, вопрос о сети и окно ролика. Альбом не трогается (ни правки, ни undo).
import { SvelteMap, SvelteSet } from 'svelte/reactivity'
import { EventsOn } from '../../../wailsjs/runtime/runtime'
import { Api, store } from '../../base/core/store.svelte'
import { errText, capFirst } from '../../base/words/errtext'
import { packsUi } from '../packs/packs.svelte'
import type { LibraryNet } from '../packs/packs'
import { getAction, type GuideVideoProgress, type VideoItem } from './videorow'

export type { GuideVideoProgress } // тип события guide:video (docs/architecture/events.md); описан в videorow.ts рядом с чистой логикой

// Ролика нет в библиотеке (ErrGone Go) — не сбой программы: тост обычный.
// caps-ok: сверка по тексту, не показ
const isGone = (e: unknown): boolean => /ролика пока нет в библиотеке/i.test(errText(e))

class Videos {
  readonly items = new SvelteMap<string, VideoItem>()
  busy = $state<string[]>([]) // id, которые качаются сейчас (один ролик или «все ролики»)
  progress = $state<GuideVideoProgress | null>(null)
  cancelling = $state(false)
  readonly failed = new SvelteSet<string>() // <video> не открылся: строка «не открылся» + «Показать файл»
  playing = $state<string | null>(null) // id в окне ролика
  consent = $state<string[] | null>(null) // вопрос «Обратиться к GitHub?»: что скачать после «Разрешить»
  consentStart = $state(false)
  private checking = false // между щелчком и началом хода идёт чтение согласия
  private seq = 0

  // Список роликов каталога из кэша Go (без сети; без каталога — пусто).
  async load() {
    const my = ++this.seq
    try {
      const list = ((await Api.GuideVideos()) as unknown as VideoItem[] | null) ?? []
      if (my !== this.seq) return
      const ids = new Set(list.map((i) => i.id))
      for (const id of [...this.items.keys()]) if (!ids.has(id)) this.items.delete(id)
      for (const i of list) this.items.set(i.id, i)
    } catch {
      // нет связи с Go — остаётся прежнее; строка «Ролик» не критична для справочника
    }
  }

  // guide:video — ход в байтах (Go не чаще 4 раз в секунду). Чужой ролик (не наш ход) не берём.
  listen(): () => void {
    const off = EventsOn('guide:video', (p: GuideVideoProgress) => {
      if (this.busy.includes(p.id)) this.progress = p
    })
    return () => off?.()
  }

  // «Скачать» / «все ролики»: нет согласия — сначала вопрос (то же, что у «Наборов»), иначе сразу ход.
  async get(ids: string[]) {
    if (this.busy.length || this.checking || ids.length === 0) return
    this.checking = true
    try {
      if (!packsUi.listed) await packsUi.load() // согласие общее с наборами; «Не обращаться к сети» снимает и его
      if (getAction(packsUi.net.net.allowed) === 'ask') {
        this.consentStart = false
        this.consent = ids
        return
      }
    } finally {
      this.checking = false
    }
    await this.run(ids)
  }

  dismissConsent() {
    this.consent = null
  }

  // «Разрешить и скачать»: согласие и флажок — в config.json у Go, затем то, что нажали.
  async confirmConsent() {
    const ids = this.consent
    if (!ids || this.busy.length) return
    this.consent = null
    this.checking = true // пока согласие пишется, «Скачать» вопрос второй раз не открывает
    try {
      packsUi.net.take((await Api.SetLibraryNet(true, this.consentStart)) as unknown as LibraryNet)
    } catch (e) {
      return store.notify(capFirst(errText(e)), 'error')
    } finally {
      this.checking = false
    }
    await this.run(ids)
  }

  private async run(ids: string[]) {
    this.busy = ids
    this.progress = null
    this.cancelling = false
    for (const id of ids) this.failed.delete(id)
    const revokes = packsUi.net.revokes
    try {
      const list = ((await Api.DownloadGuideVideos(ids)) as unknown as VideoItem[] | null) ?? []
      this.seq++ // запоздавший ответ load() не затрёт итог
      for (const i of list) this.items.set(i.id, i)
    } catch (e) {
      // «Отменить» — своё действие, тост не нужен; отзыв согласия в «Наборах» — тоже (там тост отзыва уже сказал главное);
      // Wails не передаёт список при reject — читаем заново
      if (!this.cancelling && packsUi.net.revokes === revokes) store.notify(capFirst(errText(e)), isGone(e) ? 'info' : 'error')
      await this.load()
    } finally {
      this.busy = []
      this.progress = null
      this.cancelling = false
    }
  }

  async cancel() {
    if (this.cancelling || this.busy.length === 0) return
    this.cancelling = true
    try {
      await Api.CancelGuideVideos()
    } catch (e) {
      this.cancelling = false
      store.notify(capFirst(errText(e)), 'error')
    }
  }

  // Справочник закрыли: вопрос, окно ролика и отметки сбоя не переживают его (ход скачивания идёт дальше, итог — в строке).
  reset() {
    this.consent = null
    this.playing = null
    this.failed.clear() // сбой окна мог быть случайным: при следующем открытии «Смотреть» снова доступно
  }

  play(id: string) {
    this.failed.delete(id)
    this.playing = id
  }

  stop() {
    this.playing = null
  }

  // Окно не смогло сыграть: окно закрывается, в карточке — «Ролик не открылся…» и «Показать файл».
  fail(id: string) {
    this.failed.add(id)
    if (this.playing === id) this.playing = null
  }

  // «Показать файл»: проводник открывает Go; ошибка — тостом.
  async reveal(id: string) {
    try {
      await Api.RevealGuideVideo(id)
    } catch (e) {
      store.notify(capFirst(errText(e)), 'error')
    }
  }
}

export const videos = new Videos()
