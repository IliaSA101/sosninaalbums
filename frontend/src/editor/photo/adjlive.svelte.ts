// Живой показ тона (068): пока тянут «Яркость» / «Контраст» в карточке, фото рисуется с этим фильтром
// без Go; SetTone уходит на отпускании. По образцу framelive. filterOf — единая точка для Cell, ленты
// разворотов и крупного просмотра: живое значение главнее строки из View.
import { toneCss, type Tone } from '../../base/geom/tone'

class AdjLive {
  cur = $state<{ id: string; tone: Tone } | null>(null)

  set(id: string, tone: Tone) {
    this.cur = { id, tone }
  }

  clear() {
    this.cur = null
  }

  of(id: string): Tone | null {
    return this.cur?.id === id ? this.cur.tone : null
  }

  // Строка filter для <img> фото id: живая, иначе готовая из Go; пусто — undefined (атрибут не пишем).
  filterOf(id: string, fv?: { filter?: string }): string | undefined {
    const live = this.of(id)
    return (live ? toneCss(live) : fv?.filter) || undefined
  }
}

export const adjlive = new AdjLive()
