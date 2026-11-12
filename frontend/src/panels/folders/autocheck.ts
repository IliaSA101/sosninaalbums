import { Api } from '../../base/core/store.svelte'
import { drag } from '../../base/drag/drag.svelte'

// Точка вызова Go-метода App.CheckFolders(full) (088).
// full — ещё и пересчёт недостающих фото альбома (фокус, показ окна); порог и «одна за раз» решает Go.
export function checkFolders(full: boolean): Promise<void> {
  return Api.CheckFolders(full)
}

export interface AutoDeps {
  call: (full: boolean) => Promise<void>
  // Идёт перенос фото: проверка не начинается (088 п.9).
  busy: () => boolean
}

const TICK = 10_000 // таймер, пока окно активно
const SOON = 300 // склейка ошибок загрузки миниатюр в один запрос

// Автоматическая сверка папок (088): окно получило фокус или стало видимым — полная; раз в 10 с при фокусе —
// обычная; ошибка загрузки миниатюры — отложенная одна на пачку. Что менять, решает Go — здесь только когда звать.
export class AutoCheck {
  private stopFn: (() => void) | null = null
  private soonTimer: ReturnType<typeof setTimeout> | null = null
  private soonFull = false
  private doneFns: (() => void)[] = []

  constructor(private deps: AutoDeps) {}

  private fire(full: boolean) {
    if (this.deps.busy()) return
    // Сбой вызова (нет Go, остановка) не должен всплывать: следующая проверка всё догонит.
    this.deps.call(full).then(
      () => this.doneFns.forEach((f) => f()),
      () => {},
    )
  }

  // Подписка на конец удачной сверки (после неё Go уже сообщил о пропавших файлах).
  onDone(fn: () => void) {
    this.doneFns.push(fn)
  }

  start(win: Window, doc: Document) {
    this.stop()
    const shown = () => doc.visibilityState === 'visible'
    const onFocus = () => this.fire(true)
    const onVis = () => {
      if (shown()) this.fire(true)
    }
    const timer = setInterval(() => {
      if (shown() && doc.hasFocus()) this.fire(false)
    }, TICK)
    win.addEventListener('focus', onFocus)
    doc.addEventListener('visibilitychange', onVis)
    this.stopFn = () => {
      clearInterval(timer)
      win.removeEventListener('focus', onFocus)
      doc.removeEventListener('visibilitychange', onVis)
    }
  }

  stop() {
    this.stopFn?.()
    this.stopFn = null
    if (this.soonTimer) clearTimeout(this.soonTimer)
    this.soonTimer = null
    this.soonFull = false
  }

  // Ошибка загрузки миниатюры: один вызов на пачку ошибок через SOON мс.
  soon(full = false) {
    this.soonFull ||= full
    if (this.soonTimer) return
    this.soonTimer = setTimeout(() => {
      const f = this.soonFull
      this.soonTimer = null
      this.soonFull = false
      this.fire(f)
    }, SOON)
  }
}

export const autocheck = new AutoCheck({ call: checkFolders, busy: () => drag.active })
