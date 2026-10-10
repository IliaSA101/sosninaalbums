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
