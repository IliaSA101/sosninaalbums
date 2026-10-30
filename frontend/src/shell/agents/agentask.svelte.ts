import { EventsOn } from '../../../wailsjs/runtime/runtime'
import { store } from '../../base/core/store.svelte'
import { capFirst, errText } from '../../base/words/errtext'
import { newDoc } from '../newdoc/newdoc.svelte'
import { bridge, type AgentAsk, type AgentAskDone } from './agents'
import { ASK_TIMEOUT } from './agentstext'

// Кнопки вопроса не срабатывают первые 600 мс: вопрос приходит не от щелчка человека, и случайный Enter или щелчок
// по холсту, попавший в только что поднявшееся окно, не должны ответить за него (149-25, аудит В11).
export const ARM_MS = 600

// Вопрос агента «Создать альбом…» / «Добавить папку…» (149, design §3). Логика (один вопрос за раз, сроки, счёт отказов,
// выбор папки) — в Go; здесь окно: показать, принять ответ, закрыть по agents:askdone. Образец — newerask.svelte.ts.
class AgentAskState {
  cur = $state<AgentAsk | null>(null)
  // Прошли ARM_MS: щелчки по кнопкам и Enter действуют. До этого кнопки `aria-disabled`, не `disabled`: фокус на «Отмена» нужен сразу.
  armed = $state(false)
  // «Выбрать папку…» нажато: открыт выбор папки ОС, окно ждёт итога; повторные нажатия и «Отмена» не уходят в Go.
  choosing = $state(false)
  private timer = 0
  private offEvents: (() => void) | null = null
  private watchers = 0

  // Подписка на события Go; окно в App живёт всегда.
  watch(): () => void {
    if (this.watchers++ === 0) {
      const off = [EventsOn('agents:ask', (a: AgentAsk) => this.show(a)), EventsOn('agents:askdone', (d: AgentAskDone) => this.done(d))]
      this.offEvents = () => off.forEach((f) => f?.())
    }
    let released = false
    return () => {
      if (released) return
      released = true
      if (--this.watchers === 0) {
        this.offEvents?.()
        this.offEvents = null
        this.close()
      }
    }
  }

  show(a: AgentAsk) {
    clearTimeout(this.timer)
    this.cur = { ...a, lines: a.lines ?? [] }
    this.armed = false
    this.choosing = false
    this.timer = window.setTimeout(() => (this.armed = true), ARM_MS)
  }

  close() {
    clearTimeout(this.timer)
    this.timer = 0
    this.cur = null
    this.armed = false
    this.choosing = false
  }

  // Go закончил вопрос (ответ, срок, отказ, выход): окно закрывается. Чужой id (старый вопрос) не трогает нынешний.
  done(d: AgentAskDone) {
    if (this.cur?.id !== d.id) return
    this.close()
    if (d.why === 'timeout') store.notify(ASK_TIMEOUT, 'info')
  }

  // «Отмена», ✕, щелчок по фону, Esc: отказ любого вида. force — только Esc: он идёт и в первые 600 мс; ✕ и щелчок по фону
  // (мышь могла попасть в только что поднявшееся окно) и кнопки ждут конца 600 мс.
  async cancel(force = false) {
    const a = this.cur
    if (!a || this.choosing || (!this.armed && !force)) return
    this.close()
    try {
      await bridge.answer(a.id, false)
    } catch {
      // вопрос уже закрыт Go (срок, выход) — человеку сказать нечего
    }
  }

  // «Выбрать папку…»: create — новый альбом через замену проекта (набранный текст досылается до вызова), add — папка в открытый.
  // Отказ диалога папки, негодная папка, выключенный флажок и занятый черновик Go помечает State.cancelled: replaceState тогда
  // вид и выделение не трогает, как у «Файл ▸ Открыть» (files.go OpenProject) — окно просто закрывается.
  // «Выбрать другую…» (150): окно ОС в родителе показанного пути; итог и закрытие те же.
  async other() {
    const a = this.cur
    if (!a || this.choosing || !this.armed) return
    await this.pick(a, () =>
      a.kind === 'create' ? newDoc.replace(() => bridge.answerCreate(a.id, true)) : store.run(bridge.answerOther(a.id)),
    )
  }

  async accept() {
    const a = this.cur
    if (!a || this.choosing || !this.armed) return
    // create — новый альбом из выбранной (или показанной) папки, add и edit — в открытый альбом.
    await this.pick(a, () => (a.kind === 'create' ? newDoc.replace(() => bridge.answerCreate(a.id)) : store.run(bridge.answer(a.id, true))))
  }

  private async pick(a: AgentAsk, go: () => Promise<unknown>) {
    this.choosing = true
    try {
      await go()
    } catch (e) {
      store.notify(capFirst(errText(e)), 'error')
    } finally {
      if (this.cur?.id === a.id) this.close()
    }
  }
}

export const agentAsk = new AgentAskState()
