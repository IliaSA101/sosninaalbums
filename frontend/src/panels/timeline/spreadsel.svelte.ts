import { prune, range, toggle } from './spreadsel'

// Выделение групп в ленте альбома (спека 019 F). Текущий разворот от него не зависит.
class SpreadSel {
  ids = $state<readonly string[]>([])
  // Опора для Shift+щелчка: последняя щёлкнутая с Ctrl группа; нет — берётся текущий разворот.
  anchor = $state<string | null>(null)

  get size() {
    return this.ids.length
  }

  has(id: string) {
    return this.ids.includes(id)
  }

  // Ctrl/Cmd+щелчок.
  toggle(id: string) {
    this.ids = toggle(this.ids, id)
    this.anchor = id
  }

  // Shift+щелчок: диапазон от опоры; без опоры — от текущего разворота. Опора остаётся прежней.
  extend(all: readonly string[], currentId: string, to: string) {
    this.ids = range(all, this.anchor ?? currentId, to)
  }

  // Выделенные группы в порядке альбома.
  ordered(all: readonly string[]) {
    return all.filter((id) => this.ids.includes(id))
  }

  prune(all: readonly string[]) {
    if (this.ids.length) this.ids = prune(this.ids, all)
    if (this.anchor !== null && !all.includes(this.anchor)) this.anchor = null
  }

  clear() {
    if (this.ids.length) this.ids = []
    this.anchor = null
  }
}

export const spreadsel = new SpreadSel()
