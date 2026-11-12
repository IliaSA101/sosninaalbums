// Связь .salbum с программой (112): вопрос в стартовом окне, тост при скрытом окне, пункт «Файл».
// Что и когда спрашивать — решает Go (State.assoc.ask); здесь только ответы и показ.
import { untrack } from 'svelte'
import { Api, store } from '../../base/core/store.svelte'
import type { AssocView } from '../../base/core/types'
import { capFirst, errText } from '../../base/words/errtext'

export const ASSOC_ASK = 'Открывать альбомы .salbum двойным щелчком?'
export const ASSOC_ON = 'Альбомы .salbum открываются этой программой'
export const ASSOC_OFF = 'Альбомы .salbum больше не открываются этой программой'
export const ASSOC_NEVER = 'Включить можно в меню «Файл ▸ Открывать .salbum этой программой»'

class AssocUi {
  // Вопрос в этом сеансе уже показан (окном или тостом) или уступил место чужому тосту: второй раз не спрашиваем.
  private asked = false
  // Ответ в пути: двойной щелчок не шлёт два вызова.
  busy = $state(false)

  // Вызов Go — за busy (call), иначе второй щелчок успел бы его послать. Ответ Go обновляет store.assoc; ошибка короткая, без путей (тост), состояние прежнее. true — ответ принят.
  private async take(call: () => Promise<AssocView>): Promise<boolean> {
    if (this.busy) return false
    this.busy = true
    try {
      store.assoc = await call()
      return true
    } catch (e) {
      store.notify(capFirst(errText(e)), 'error')
      return false
    } finally {
      this.busy = false
    }
  }

  // «Да» (окно, тост).
  async yes() {
    if (await this.take(() => Api.SetAssoc(true))) store.notify(ASSOC_ON)
  }

  // «Не сейчас»: Go запомнит версию, вопрос вернётся в следующей.
  async later() {
    await this.take(() => Api.AssocLater())
  }

  // «Не спрашивать»: подсказка, где это включить.
  async never() {
    if (await this.take(() => Api.SetAssoc(false))) store.notify(ASSOC_NEVER)
  }

  // Пункт «Файл ▸ Открывать .salbum этой программой»: галочка включает и снимает связь.
  async toggle() {
    if (!store.assoc.can) return
    const on = !store.assoc.on
    if (await this.take(() => Api.SetAssoc(on))) store.notify(on ? ASSOC_ON : ASSOC_OFF)
  }

  // Стартовое окно скрыто, а Go просит спросить — вопрос тостом с «Да». Зовёт StartDialog на каждую смену
  // состояния: показанное окно (хоть раз за сеанс) вопрос уже задало; чужой тост (ошибка старта) не затираем.
  see(startOpen: boolean) {
    if (startOpen) this.asked = true
    if (this.asked || !store.loaded || !store.assoc.ask) return
    this.asked = true
    if (untrack(() => store.toast)) return
    store.notify(ASSOC_ASK, 'info', { label: 'Да', run: () => void this.yes() })
  }

  // Для тестов: новый сеанс.
  reset() {
    this.asked = false
    this.busy = false
  }
}

export const assocUi = new AssocUi()
