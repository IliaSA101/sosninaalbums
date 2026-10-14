// Когда ленте можно прокрутиться за рядом разворотов (спека 012 §7).
// Ручную прокрутку сбрасывает только следующее движение ряда: сам по себе новый rev
// (автосохранение, правка) или ширина ленты её не трогают.
export interface ScrollState {
  posChanged: boolean // сменилась позиция ряда
  movingStarted: boolean // начался ход ряда
  scrollLeft: number // текущая прокрутка ленты
  written: number | null // что записали мы в прошлый раз (null — ещё не писали)
}

export function shouldScroll(s: ScrollState): boolean {
  if (s.posChanged || s.movingStarted || s.written === null) return true
  // Прокрутка всё ещё наша (допуск на округление браузера) — можно подправить под новую раскладку.
  return Math.abs(s.scrollLeft - s.written) < 1
}

// Центры ленты по порядку слотов ряда: обложка (только настоящая — заглушка «+ Обложка» слотом ряда не является),
// группы разворотов, последний — «+». Индекс центра = слот ряда (row.pos).
export const CENTER_SEL = '[data-cover-slot], [data-drop="mini"], .addcol'

// Память ленты между кадрами: последняя позиция ряда, был ли ход, что записали в scrollLeft.
export class StripFollow {
  private lastPos = NaN
  private wasMoving = false
  private written: number | null = null

  // Лента опустела (0 слотов) или скрыта: содержимое сжалось, и браузер сам прижал scrollLeft к 0.
  // Без сброса первый лист после пустой ленты сравнивался бы с прежней записью — «пользователь прокрутил сам» —
  // и лента оставалась на 0, сдвинутая от уголка на всю прежнюю прокрутку.
  reset() {
    this.lastPos = NaN
    this.wasMoving = false
    this.written = null
  }

  // Ход ряда начался с прошлого кадра — центры групп перемеряются.
  starting(moving: boolean): boolean {
    return moving && !this.wasMoving
  }

  // Прокручивать ли в этом кадре; запоминает позицию и ход.
  next(pos: number, moving: boolean, scrollLeft: number): boolean {
    const go = shouldScroll({ posChanged: pos !== this.lastPos, movingStarted: this.starting(moving), scrollLeft, written: this.written })
    this.wasMoving = moving
    this.lastPos = pos
    return go
  }

  wrote(scrollLeft: number) {
    this.written = scrollLeft
  }
}
