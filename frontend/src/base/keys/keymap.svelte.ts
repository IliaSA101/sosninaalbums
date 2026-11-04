// Реестр горячих клавиш в работе (спека 074, design §7.1): вид от Go (State.keymap и ответы окна)
// и индекс «сочетание → команды». Логики клавиш здесь нет — только разбор и поиск; диспетчер — shell/menu.
import { isMac } from '../core/platform'
import type { KeymapEntry, KeymapView } from '../types/keys'
import { chordText } from './chordtext'
import { buildIndex, EMPTY_KEYMAP } from './keyindex'
import { fillKeys } from './keytext'

class Keymap {
  view = $state<KeymapView>(EMPTY_KEYMAP)
  index = $derived(buildIndex(this.view))

  // Принять вид от Go. Пустой ответ (null, старый Go без поля) вид не затирает.
  apply(view: KeymapView | null | undefined) {
    if (view) this.view = view
  }

  entry(id: string): KeymapEntry | undefined {
    return this.view.commands.find((e) => e.id === id)
  }

  // Действующие клавиши команды (канонические записи).
  keys(id: string): string[] {
    return this.entry(id)?.keys ?? []
  }

  // Клавиша текстом для меню, подсказки и тоста: первая, n — по номеру (Shift+Пробел у spaceDown — вторая); нет — ''.
  hint(id: string, n = 0): string {
    const k = this.keys(id)[n]
    return k ? chordText(k, isMac()) : ''
  }

  // Подпись с клавишей в скобках: «Отменить (Ctrl+Z)»; клавиши нет — подпись как есть.
  tip(label: string, id: string, n = 0): string {
    const h = this.hint(id, n)
    return h ? `${label} (${h})` : label
  }

  // Текст с метками {key:id} (статьи справочника, подсказки).
  text(s: string): string {
    return fillKeys(s, (id) => this.hint(id))
  }
}

export const keymap = new Keymap()
