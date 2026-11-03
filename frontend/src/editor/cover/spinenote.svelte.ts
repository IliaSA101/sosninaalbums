// Отметка «Корешок X → Y мм» (спека 032, сценарий 7): корешок сам пересчитался от числа разворотов, и
// фотограф должен это заметить. Go отметку не хранит (развилка 5): UI помнит последний увиденный корешок.
// Исчезает, когда обложку открыли (dismiss) или по отметке щёлкнули.
import { spineNoteText } from './cover'

const EPS = 1e-6

class SpineNote {
  note = $state<{ from: number; to: number } | null>(null)
  private last: number | null = null

  get text(): string {
    return this.note ? spineNoteText(this.note.from, this.note.to) : ''
  }

  // После каждой смены вида: spine — корешок обложки (null — обложки нет), open — она сейчас открыта
  // (смотрят на неё сами — отметка не нужна). Несколько правок подряд складываются: 12 → 14 → 16 даёт «12 → 16».
  see(spine: number | null, open: boolean) {
    if (spine === null) return this.reset()
    const last = this.last
    this.last = spine
    if (last === null || Math.abs(spine - last) < EPS) return
    if (open) {
      this.note = null
      return
    }
    const from = this.note?.from ?? last
    this.note = Math.abs(spine - from) < EPS ? null : { from, to: spine }
  }

  dismiss() {
    this.note = null
  }

  // Другой альбом или обложки нет: прежнее значение больше не сравнивается.
  reset() {
    this.last = null
    this.note = null
  }
}

export const spineNote = new SpineNote()
export { SpineNote }
