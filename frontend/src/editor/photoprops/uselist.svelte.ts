// Вхождения одного фото для подменю «Где стоит» (спека 106, 108): запрос к Go и поздний ответ, который отбрасывается.
// Один класс на меню ленты и меню холста, чтобы они вели себя одинаково.
import * as Api from '../../../wailsjs/go/app/App'
import { store } from '../../base/core/store.svelte'
import { errText } from '../../base/words/errtext'
import type { UseSrc } from '../../base/words/whereuse'

export class UseList {
  // Вхождения; null — запрос идёт или не нужен.
  lines = $state.raw<UseSrc[] | null>(null)
  // Номер запроса: ответ закрытого или другого меню отбрасывается.
  private req = 0

  // Спросить Go; ошибка — тост и onFail (меню закрывается).
  load(id: string, onFail: () => void) {
    this.lines = null
    const my = ++this.req
    void this.fetch(id, my, onFail)
  }

  private async fetch(id: string, my: number, onFail: () => void) {
    try {
      const r = (await Api.WhereUsed([id])) ?? []
      if (my === this.req) this.lines = (r[0]?.uses ?? []) as unknown as UseSrc[]
    } catch (e) {
      if (my !== this.req) return
      onFail()
      store.notify(errText(e), 'error')
    }
  }

  // Меню закрыто или запрос не нужен: поздний ответ уже не нужен.
  reset() {
    this.req++
    this.lines = null
  }
}
