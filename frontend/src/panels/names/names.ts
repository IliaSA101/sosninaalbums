// Имена на фото (спека 030): строка имени для предпросмотра и поля имени в карточке фото. Разбор списка,
// заголовки и сопоставление считает Go (Api.PreviewNames, спека 043) — здесь только показ.
import type { Person } from '../../base/core/types'

// «Иванов Пётр Сергеевич» — как читается имя в предпросмотре и в карточке.
export const personLine = (p: Person): string => [p.last, p.first, p.middle].filter(Boolean).join(' ')

// Пустое имя (все четыре поля пусты) — снова имя из файла.
export const emptyPerson = (p: Person): boolean => !p.last?.trim() && !p.first?.trim() && !p.middle?.trim() && !p.subj?.trim()

// Ctrl/Cmd+Enter внутри окна «Имена»: слушаем на window, потому что после щелчка в пустое место фокус уходит на само окно,
// а не на содержимое. Событие из другого окна (виньетка поверх) не наше.
export const isPasteKey = (
  e: { key: string; ctrlKey: boolean; metaKey: boolean; target: EventTarget | null },
  box: Element | null | undefined,
): boolean => e.key === 'Enter' && (e.ctrlKey || e.metaKey) && !!box && !!e.target && box.contains(e.target as Node)

// Поля карточки → Person для Go: края обрезаны.
export const personOf = (p: Person): Person => ({
  last: (p.last ?? '').trim(),
  first: (p.first ?? '').trim(),
  middle: (p.middle ?? '').trim(),
  subj: (p.subj ?? '').trim(),
})
