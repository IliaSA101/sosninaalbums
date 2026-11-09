// Окно «Альбом из новой версии» (112): состояние и вызовы Go. Показывает хук store.newer (State.newer при старте
// и в ответах открытия вместе с cancelled). Образец — CopyAskDialog; окно: NewerAskDialog.svelte.
import { Api, store } from '../../base/core/store.svelte'
import type { NewerInfo } from '../../base/core/types'
import { capFirst, errText } from '../../base/words/errtext'
import { aboutUi } from '../dialogs/about.svelte'

class NewerAskState {
  cur = $state<NewerInfo | null>(null)
  // Идёт OpenInNewer: второй щелчок по кнопке не запускает вторую копию.
  busy = $state(false)

  show(info: NewerInfo) {
    this.cur = info
  }

  close() {
    this.cur = null
  }

  // «Открыть в X»: успех — Go закрывает программу (асинхронно, поэтому busy остаётся до конца: второй щелчок
  // запустил бы новую версию ещё раз); ошибка — busy снят, тост, окно остаётся (можно «Скачать»).
  async openInNewer() {
    if (this.busy) return
    this.busy = true
    try {
      await Api.OpenInNewer()
    } catch (e) {
      this.busy = false
      store.notify(capFirst(errText(e)), 'error')
    }
  }

  // «Скачать»: страница последнего выпуска; окно остаётся, как «Открыть Фотосферу» в «Готово».
  async download() {
    await aboutUi.link('download') // там же тост «На стенде ссылки не открываются» и ошибка Go
  }
}

export const newerAsk = new NewerAskState()
