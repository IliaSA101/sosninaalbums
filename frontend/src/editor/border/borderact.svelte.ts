// Правки рамочки и прозрачности из карточки (079 U3): вызовы Go и серии merge. Живой показ — borderlive; здесь то, что
// уходит в проект. Одна сессия merge на фокус строки: стрелки и несколько протяжек подряд — один шаг отмены.
import { store } from '../../base/core/store.svelte'
import { frameApi, type BorderPatch } from '../../panels/decor/frameapi'
import { PatSession } from '../pattern/pattern'
import { borderlive } from './borderlive.svelte'

class BorderAct {
  readonly session = new PatSession()

  // merge для строки key: открывает сессию, если её ещё нет (или она про другую строку/выделение).
  // Идентификатор сессии — от времени, поэтому время строго растёт: две сессии в одну миллисекунду слились бы в Go.
  private last = 0
  private merge(key: string): string {
    if (!this.session.merge(key)) this.session.begin(key, (this.last = Math.max(Date.now(), this.last + 1)))
    return this.session.merge(key)
  }

  // Толщина / размер: серия под одним merge, очередь вызовов — порядок ответов Go не гарантирован.
  series(spread: string, row: 'width' | 'size', ids: string[], patch: BorderPatch) {
    const merge = this.merge(`border:${spread}:${row}:${ids.join(',')}`)
    return borderlive.commit(() => store.run(frameApi.set(spread, ids, patch, merge)))
  }

  // Цвет: шаг на выбор, без серии.
  ink(spread: string, ids: string[], patch: BorderPatch) {
    return store.run(frameApi.set(spread, ids, patch, ''))
  }

  // «Убрать»: ApplyBorder с пустой плиткой; рамочки, которой нет, — холостая правка.
  clear(spread: string, ids: string[]) {
    return store.run(frameApi.apply(spread, ids, ''))
  }

  // Прозрачность фото ids; key — ключ сессии, который строит строка (выделение, лист), а begin зовёт её onfocus.
  fade(photos: string[], fade: number, key: string) {
    const merge = this.session.merge(key)
    return borderlive.commit(() => store.run(frameApi.fade(photos, fade, merge)))
  }

  // Фокус ушёл из строки или сменилось выделение: серия кончилась, просмотр снимается (если Go ничего не ждёт).
  end() {
    this.session.end()
    borderlive.release()
  }
}

export const borderAct = new BorderAct()
