// Состояние окна «Подборки» (083): какая строка в режиме имени и что сказано под полем. Вынесено из BinMenu.svelte
// ради теста без DOM; вызовы Go — binact.ts.
import { create, rename } from './binact'

export const NEW = 'new' // editing: строка новой подборки

export class BinMenuState {
  // id подборки в режиме имени, NEW — новая, '' — никакая.
  editing = $state('')
  // Текст ошибки под полем («Такая подборка уже есть»).
  err = $state('')

  startEdit(id: string) {
    this.err = ''
    this.editing = id
  }

  // Esc, уход фокуса, закрытие окна.
  stop() {
    this.editing = ''
    this.err = ''
  }

  // Enter в строке новой: подборка вместе с выделенными ids. Ошибка Go остаётся под полем, поле открыто.
  async submitNew(name: string, ids: string[]) {
    const r = await create(name, ids)
    this.settle(r)
  }

  // Enter при переименовании: пустое и прежнее имя — просто закрыть поле.
  async submitRename(id: string, name: string) {
    const r = await rename(id, name)
    this.settle(r)
  }

  private settle(r: { ok: boolean; err?: string }) {
    if (r.err) this.err = r.err
    else this.stop() // успех, а также ошибка, которую уже показал store тостом
  }
}
