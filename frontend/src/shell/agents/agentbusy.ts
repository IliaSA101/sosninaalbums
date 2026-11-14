import { store } from '../../base/core/store.svelte'
import { jobs } from '../../base/core/jobs.svelte'
import { drag } from '../../base/drag/drag.svelte'
import { canvas } from '../../editor/canvas/canvas.svelte'
import { edit } from '../../editor/text/edit.svelte'
import { settingsBusy } from '../../panels/details/settingsbusy.svelte'
import { relinking } from '../../panels/library/relinkbusy.svelte'
import { bridge } from './agents'

// «Человек занят» (149-16, 149-32, design §2.5): правка агента подождёт ≤ 2 с, потом ему скажут «повторите позже».
// Флаг — совет, не замок: Go считает его живым 30 с от последнего сигнала, поэтому пока занят — повторяем раз в 10 с.
export const BUSY_PING_MS = 10_000

// Занят: перенос и жесты на холсте, набор текста, идущая задача (сборка, выгрузка, сбор, открытие, перепривязка папки),
// окно «Автосборка» (store.autoDraft) и правка в «Настройках альбома» (непринятые правки или фокус в форме, не просто
// раскрытый раздел: он хранится в localStorage). Окна «ИИ-агенты», F1 и просмотр не мешают:
// смотреть — не править, поэтому весь modalstack не берём. Состояние читается снаружи (base/drag и base/modal не меняются).
export function userBusy(): boolean {
  return (
    drag.active ||
    canvas.busy ||
    edit.editing ||
    store.busy ||
    jobs.exporting !== null ||
    jobs.collecting !== null ||
    jobs.opening !== null ||
    !!store.autoDraft ||
    relinking.on ||
    settingsBusy()
  )
}

// Сигнал Go: set(true) — сразу и раз в BUSY_PING_MS, set(false) — один раз в конце. Повторный set с тем же значением ничего не шлёт.
// Ответ Go не нужен, ошибка связи не должна мешать работе: сигнал теряется молча (через 30 с Go сам считает «не занят»).
export class BusyPing {
  private on = false
  private timer = 0

  set(on: boolean) {
    if (on === this.on) return
    this.on = on
    clearInterval(this.timer)
    this.timer = 0
    this.send(on)
    if (on) this.timer = window.setInterval(() => this.send(true), BUSY_PING_MS)
  }

  // Окно закрыто (выход, перезагрузка): если были заняты — снимаем флаг.
  stop() {
    this.set(false)
  }

  private send(on: boolean) {
    Promise.resolve(bridge.busy(on)).catch(() => {})
  }
}
