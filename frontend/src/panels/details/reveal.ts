// Прокрутка панели деталей (028 Б): чистая арифметика, без DOM. Координаты — в системе
// прокручиваемого контейнера (offsetTop / getBoundingClientRect относительно его верха).

// Заголовок карточки выше видимой части — надо вернуться к началу.
export const needScroll = (cardTop: number, scrollTop: number): boolean => cardTop < scrollTop

// Сдвиг scrollTop, чтобы элемент [top, bottom] (от верха видимой области) оказался в поле зрения.
// Выше края — к верху с отступом, ниже — к низу; не влезает целиком — приоритет верху.
export function revealDelta(top: number, bottom: number, viewH: number, gap = 8): number {
  if (top < 0) return top - gap
  if (bottom > viewH) return Math.min(bottom - viewH + gap, top - gap)
  return 0
}

// Куда реально можно прокрутить к разделу: цель не дальше конца содержимого. Пока раздел раскрывается,
// конец растёт, и достижимая цель меняется — по этому значению панель решает, пора ли перезапускать прокрутку.
export const reachableTop = (target: number, scrollHeight: number, viewH: number): number =>
  Math.round(Math.max(0, Math.min(target, scrollHeight - viewH)))
