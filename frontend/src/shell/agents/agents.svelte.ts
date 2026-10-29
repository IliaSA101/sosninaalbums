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
  type AgentEdited,
  type AgentSetup,
  type AgentShow,
  type AgentsInfo,
} from './agents'
import { agentEdited, agentShow } from './agentedited'
import { BusyPing, userBusy } from './agentbusy'
import { COPIED, COPY_FAIL } from './agentstext'

// Вопрос перед включением доступа, уровня или правок: 'on' — «Разрешить агентам ИИ читать альбом?», 'names' и 'pics' — уровни 2 и 3,
// 'edit' — «Разрешить агентам ИИ менять альбом?» (149).
export type AgentsAsk = 'on' | 'names' | 'pics' | 'edit'

// Окно «Файл ▸ ИИ-агенты (MCP)…» (143): состояние от Go (`agents:state`, `agents:call`), решения — в Go.
// Доступ — данные программы, не альбома: правок проекта и шагов отмены нет. Образец — packs.svelte.ts (067).
class AgentsUi {
  open = $state(false)
  info = $state<AgentsInfo>(emptyInfo())
  ask = $state<AgentsAsk | null>(null)
  busy = $state(false)
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

  async show() {
    this.open = true
    this.ask = null
    await this.load()
  }

  close() {
    this.open = false
    this.ask = null
  }

  want(a: AgentsAsk) {
    this.ask = a
  }

  dismiss() {
    this.ask = null
  }

  // «Разрешить» / «Показать» в вопросе: окно вопроса закрывается сразу, итог — по ответу Go.
  async confirm() {
    const a = this.ask
    this.ask = null
    if (a === 'on') await this.call(() => bridge.set(true))
    else if (a === 'names') await this.call(() => bridge.setLevel(levelAfterNames(true)))
    else if (a === 'pics') await this.call(() => bridge.setLevel(levelAfterPics(true)))
    else if (a === 'edit') await this.call(() => bridge.setEdit(true))
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

  // Правки агентов: включение спрашивает, снятие — сразу.
  toggleEdit(on: boolean) {
    if (on) this.want('edit')
    else void this.call(() => bridge.setEdit(false))
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
