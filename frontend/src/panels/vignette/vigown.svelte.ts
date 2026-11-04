// Состояние окон своих шаблонов виньетки (038): «Сохранить как шаблон», «Переименовать», «Удалить».
// Окно одно на три места (карточка, галерея, окно «Настроить виньетку») — рисует VigTplSave в App.
import type { VigChoice } from '../../base/core/types'
import { deleteVigTemplate, renameVigTemplate, saveVigTemplate } from './vigown'

export type OwnAsk = { kind: 'save'; spread: string } | { kind: 'rename'; id: string } | { kind: 'delete'; id: string; name: string }

class VigOwn {
  ask = $state<OwnAsk | null>(null)
  // Имя в поле — отдельно от ask: окно закрывается раньше, чем bind:value отпустит поле.
  name = $state('')
  // Счётчик правок списка шаблонов: у Go событий нет, окна и галерея перезапрашивают VignetteChoices по нему.
  rev = $state(0)

  save(spread: string) {
    this.name = ''
    this.ask = { kind: 'save', spread }
  }
  rename(c: Pick<VigChoice, 'id' | 'name'>) {
    this.name = c.name
    this.ask = { kind: 'rename', id: c.id }
  }
  del(c: Pick<VigChoice, 'id' | 'name'>) {
    this.ask = { kind: 'delete', id: c.id, name: c.name }
  }
  close() {
    this.ask = null
  }

  // «Сохранить» / «Удалить» окна: закрывается сразу, список перезапрашивается после успеха.
  async commit(): Promise<boolean> {
    const a = this.ask
    if (!a) return false
    this.ask = null
    const ok =
      a.kind === 'save'
        ? await saveVigTemplate(a.spread, this.name)
        : a.kind === 'rename'
          ? await renameVigTemplate(a.id, this.name)
          : await deleteVigTemplate(a.id)
    if (ok) this.rev++
    return ok
  }
}

export const vigOwn = new VigOwn()
