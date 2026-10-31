// Состояние крупного просмотра плитки (133 U2): какая плитка открыта и список соседей на момент открытия.
// Рисует DecorPeek; сам просмотр ничего не правит — «Поставить» зовёт decor.pick, как щелчок по плитке.
import { neighbor } from './peekaddr'

class DecorPeekState {
  id = $state('') // открытая плитка; '' — просмотра нет
  ids = $state.raw<string[]>([]) // видимые плитки на момент открытия: отбор и тема потом не листаются под рукой

  open(ids: readonly string[], id: string) {
    this.ids = [...ids]
    this.id = id
  }
  close() {
    this.id = ''
    this.ids = []
  }
  // ← →: сосед среди плиток, что ещё есть в каталоге (alive); на краю ничего.
  step(dir: 1 | -1, alive: (id: string) => boolean) {
    this.id = neighbor(this.ids.filter(alive), this.id, dir)
  }
}

export const peek = new DecorPeekState()
