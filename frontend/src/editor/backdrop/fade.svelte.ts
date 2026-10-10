// Живой просмотр прозрачности (спека 017): пока тянут ползунок, фон на холсте, в ленте и на
// мини-разворотах рисуется по этому значению, а не по View. Go спрашивается на отпускании.
import type { BackdropScope } from '../../base/core/types'

interface Live {
  spread: string
  scope: BackdropScope
  fade: number
}

class FadeLive {
  cur = $state<Live | null>(null)
  // Очередь коммитов: удержанная стрелка шлёт много change, порядок ответов Go иначе не гарантирован.
  private chain: Promise<unknown> = Promise.resolve()
  private pending = 0

  set(spread: string, scope: BackdropScope, fade: number) {
    this.cur = { spread, scope, fade }
  }

  clear() {
    this.cur = null
  }

  // Просмотр снимается, только когда нет коммитов в пути: иначе фон мигнёт старым значением.
  release() {
    if (this.pending === 0) this.cur = null
  }

  // Коммит в очередь; просмотр снимается после последнего.
  commit(run: () => Promise<unknown>): Promise<unknown> {
    this.pending++
    const p = this.chain.then(run, run)
    this.chain = p.catch(() => {})
    return this.chain.finally(() => {
      this.pending--
      this.release()
    })
  }

  // Прозрачность области: просмотр, если он про неё, иначе значение из View.
  of(spread: string, scope: BackdropScope, fade: number): number {
    const c = this.cur
    return c && c.spread === spread && c.scope === scope ? c.fade : fade
  }
}

export const fadeLive = new FadeLive()
