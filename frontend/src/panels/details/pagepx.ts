// Строка «Файл разворота при 300 dpi, px» в «Настройках альбома» (123, design §1.3): запросы страницы у Go.
import { Api } from '../../base/core/store.svelte'
import type { PagePxView } from '../../base/core/types'
import { capFirst, errText } from '../../base/words/errtext'

// Запросы идут по одному вводу, ответы приходят в любом порядке: считается только последний.
// settled() ждёт ответа — «Применить» не должно уйти со страницей, которую Go ещё не дал.
export class PxAsker {
  private seq = 0
  private cur: Promise<void> = Promise.resolve()

  settled(): Promise<void> {
    return this.cur
  }

  ask(w: number, h: number, ok: (r: PagePxView) => void, fail: (text: string) => void): Promise<void> {
    const my = ++this.seq
    this.cur = Api.PagePx(w, h).then(
      (r) => {
        if (my === this.seq) ok(r)
      },
      (e) => {
        if (my === this.seq) fail(capFirst(errText(e)))
      },
    )
    return this.cur
  }
}
