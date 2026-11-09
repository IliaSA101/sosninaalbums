// «Пустое поле» — единое решение для двойного щелчка «вписать» (Stage.dbl) и начала рамки выделения
// (Marquee.start): пустая часть разворота и серое поле вокруг него. Не поле: объекты на развороте,
// полоса и панели (data-chrome), слой соседнего разворота ([data-peer]), пунктиры «новый разворот» и
// «первый» (.new, .first), кнопки и поля ввода. Фото фона не мешает: оно не ловит мышь.
export function emptyField(t: Element | null): boolean {
  if (!t) return false
  if (t.closest('.spread')) return t.matches('.spread, .page, .content, .spine') && !t.closest('[data-peer]')
  return !t.closest('button, input, [data-chrome], [data-peer], .new, .first, [contenteditable]')
}
