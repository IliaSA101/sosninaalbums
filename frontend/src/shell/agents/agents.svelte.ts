import { tick } from 'svelte'
import { EventsOn } from '../../../wailsjs/runtime/runtime'
import { store } from '../../base/core/store.svelte'
import { capFirst, errText } from '../../base/words/errtext'
import {
  bridge,
  clientNames,
  emptyInfo,
  levelAfterNames,
  levelAfterPics,
  normInfo,
  pushCall,
  type AgentCallEvent,
  type AgentDenied,
  type AgentEdited,
  type AgentSetup,
  type AgentShow,
  type AgentsInfo,
} from './agents'
import { agentDenied, agentEdited, agentShow } from './agentedited'
import {
  KEY_SEE,
  askMissing,
  consentFor,
  groupAskMissing,
  rowKey,
  rowsOf,
  type Consent,
  type Mode,
  type PermChange,
  type PermGroup,
} from './agentperms'
import { BusyPing, userBusy } from './agentbusy'
import { COPIED, COPY_FAIL } from './agentstext'

// Вопрос перед включением доступа, уровня или правок: 'on' — «Разрешить агентам ИИ читать альбом?», 'names' и 'pics' — уровни 2 и 3,
// 'perm' — согласие на правки и «Всегда» у папки (150): какое именно, считает consent по отложенному изменению.
export type AgentsAsk = 'on' | 'names' | 'pics' | 'perm'
const FLASH_MS = 1600 // сколько подсвечена строка, к которой открыли окно

// Окно «Файл ▸ ИИ-агенты (MCP)…» (143): состояние от Go (`agents:state`, `agents:call`), решения — в Go.
// Доступ — данные программы, не альбома: правок проекта и шагов отмены нет. Образец — packs.svelte.ts (067).
class AgentsUi {
  open = $state(false)
  info = $state<AgentsInfo>(emptyInfo())
  ask = $state<AgentsAsk | null>(null)
  busy = $state(false)
  // Изменение разрешения, ждущее согласия (ask === 'perm').
  pending = $state<PermChange | null>(null)
  // Подсвеченная строка (rowKey / KEY_PLACES / KEY_SEE) и причина «Спрашивать» нет: tool или 'group:read'.
  flash = $state('')
  whyFor = $state<Record<string, true>>({})
  private flashTimer = 0
  private watchers = 0
  private offEvents: (() => void) | null = null
  private ping = new BusyPing()

  get on(): boolean {
    return this.info.status === 'on'
  }
  // Значок «ИИ» показывает точку, пока есть подключение.
  get live(): boolean {
    return this.on && this.info.clients.length > 0
  }
  get names(): string[] {
    return clientNames(this.info.clients)
  }
  // Какие согласия нужны отложенному изменению.
  get consent(): Consent {
    return this.pending ? consentFor(this.info.perms, this.info.edit, this.pending) : { edit: false, folder: false }
  }

  // Подписка на события Go; окно и значок считают её вместе — события слушаются, пока жив хоть один.
  // Первая подписка сразу читает состояние (значок нужен до открытия окна).
  watch(): () => void {
    if (this.watchers++ === 0) {
      this.offEvents = this.subscribe()
      void this.load()
    }
    let done = false
    return () => {
      if (done) return
      done = true
      if (--this.watchers === 0) {
        this.offEvents?.()
        this.offEvents = null
      }
    }
  }

  private subscribe(): () => void {
    // Значок «ИИ» живёт в шапке всегда, поэтому сигнал «человек занят» (перенос, набор, «Автосборка»…) идёт отсюда, а не из окна вопроса.
    const stopBusy = $effect.root(() => {
      $effect(() => {
        this.ping.set(userBusy())
      })
    })
    const off = [
      EventsOn('agents:state', (i: AgentsInfo) => this.take(i)),
      EventsOn('agents:call', (c: AgentCallEvent) => {
        this.info = { ...this.info, calls: pushCall(this.info.calls, c), today: c.today }
      }),
      // Правки агента (149): тосты и переходы живут, пока жив значок «ИИ» (он в шапке всегда), окно открывать не нужно.
      EventsOn('agents:edited', (e: AgentEdited) => agentEdited(e)),
      EventsOn('agents:show', (s: AgentShow) => agentShow(s)),
      EventsOn('agents:denied', (d: AgentDenied) => agentDenied(d, (tool) => void this.show(rowKey(tool)))),
    ]
    return () => {
      off.forEach((f) => f?.())
      stopBusy()
      this.ping.stop() // подписка снята посреди занятости — Go получает «не занят»
    }
  }

  private take(i: AgentsInfo) {
    this.info = normInfo(i)
  }

  async load() {
    try {
      this.take(await bridge.info())
    } catch (e) {
      store.notify(capFirst(errText(e)), 'error')
    }
  }

  // focus — строка, к которой прокрутить и которую подсветить (rowKey, KEY_PLACES): тост «Настроить», ссылки вопроса агента.
  // Окно уже открыто — открываем заново: оно должно встать выше вопроса, из которого пришли.
  async show(focus = '') {
    if (this.open) {
      this.open = false
      await tick()
    }
    this.open = true
    this.ask = null
    this.pending = null
    this.whyFor = {}
    await this.load()
    if (focus) this.highlight(focus)
  }

  close() {
    this.open = false
    this.ask = null
    this.pending = null
    clearTimeout(this.flashTimer)
    this.flash = ''
  }

  // Подсветка строки на FLASH_MS; прокрутку и раскрытие группы делает окно по этому ключу.
  highlight(key: string) {
    clearTimeout(this.flashTimer)
    this.flash = key
    this.flashTimer = window.setTimeout(() => (this.flash = ''), FLASH_MS)
  }

  want(a: AgentsAsk) {
    this.ask = a
  }

  dismiss() {
    this.ask = null
    this.pending = null
  }

  // «Разрешить» / «Показать» в вопросе: окно вопроса закрывается сразу, итог — по ответу Go.
  async confirm() {
    const a = this.ask
    const ch = this.pending
    this.ask = null
    this.pending = null
    if (a === 'on') await this.call(() => bridge.set(true))
    else if (a === 'names') await this.call(() => bridge.setLevel(levelAfterNames(true)))
    else if (a === 'pics') await this.call(() => bridge.setLevel(levelAfterPics(true)))
    else if (a === 'perm' && ch) await this.apply(ch)
  }

  // Флажки: включение спрашивает, снятие — сразу (снятие «имён» снимает и «картинки»).
  toggleNames(on: boolean) {
    if (on) this.want('names')
    else void this.call(() => bridge.setLevel(levelAfterNames(false)))
  }
  togglePics(on: boolean) {
    if (on) this.want('pics')
    else void this.call(() => bridge.setLevel(levelAfterPics(false)))
  }

  private apply(ch: PermChange): Promise<void> {
    return this.call(() => (ch.kind === 'tool' ? bridge.setPerm(ch.tool, ch.mode) : bridge.setPermGroup(ch.group, ch.mode)))
  }

  // Разрешения (150): выход правок из «Запретить» и «Всегда» у папки спрашивают согласие, остальное — сразу.
  private change(ch: PermChange) {
    const c = consentFor(this.info.perms, this.info.edit, ch)
    if (c.edit || c.folder) {
      this.pending = ch
      this.ask = 'perm'
    } else void this.apply(ch)
  }

  // Щелчок по значению строки. «Спрашивать» там, где его нет (чтение, запуск), значения не меняет: показывает причину,
  // а у чтения подсвечивает «Что видит агент» (owner-ux §2.7: вместо серой кнопки — где это сделать).
  pickTool(tool: string, mode: Mode) {
    const p = this.info.perms.find((x) => x.tool === tool)
    if (!p || this.busy) return
    if (mode === 'ask' && askMissing(p)) return this.explain(tool, p.group === 'read')
    if (p.mode !== mode) this.change({ kind: 'tool', tool, mode })
  }
  pickGroup(group: PermGroup, mode: Mode) {
    const rows = rowsOf(this.info.perms, group)
    if (this.busy || rows.length === 0) return
    if (mode === 'ask' && groupAskMissing(rows)) return this.explain(`group:${group}`, group === 'read')
    this.change({ kind: 'group', group, mode })
  }
  private explain(key: string, read: boolean) {
    this.whyFor = { ...this.whyFor, [key]: true }
    if (read) this.highlight(KEY_SEE)
  }

  choosePlace() {
    return this.call(() => bridge.choosePlace())
  }
  forgetPlaces() {
    return this.call(() => bridge.forgetPlaces())
  }

  // «Разрешить вопросы» после третьего отказа подряд: Go снимает тишину и шлёт новое состояние.
  async allowAsks() {
    if (this.busy) return
    this.busy = true
    try {
      await bridge.allowAsks()
    } catch (e) {
      store.notify(capFirst(errText(e)), 'error')
    } finally {
      this.busy = false
    }
    await this.load()
  }

  disable(): Promise<void> {
    return this.call(() => bridge.set(false))
  }
  clearLog(): Promise<void> {
    return this.call(() => bridge.clearLog())
  }

  private async call(fn: () => Promise<AgentsInfo>) {
    if (this.busy) return
    this.busy = true
    try {
      this.take(await fn())
    } catch (e) {
      store.notify(capFirst(errText(e)), 'error')
      await this.load() // состояние у Go могло не смениться — показываем его, а не наш прежний вид
    } finally {
      this.busy = false
    }
  }

  async copy(s: AgentSetup) {
    try {
      await navigator.clipboard.writeText(s.text)
      store.notify(COPIED, 'info')
    } catch {
      // Текст отказа буфера приходит от браузера по-английски — свой.
      store.notify(COPY_FAIL, 'error')
    }
  }

  // «Сохранить файл для Claude Desktop…»: диалог и запись в Go; отмена диалога — тишина, итог — текст Go.
  async saveFile() {
    try {
      const r = await bridge.saveExtension()
      if (!r.canceled) store.notify(r.text, 'info', undefined, 9000)
    } catch (e) {
      store.notify(capFirst(errText(e)), 'error')
    }
  }
}

export const agents = new AgentsUi()
