// Отметки фото (спека 004): слияние своей отметки с XMP из файла и условия фильтра ленты.
// Чистая логика без Svelte. markOf — единственное место слияния на фронте (зеркало Go effective,
// паритет — testdata/parity/marks.json).
import type { Label, Mark, MarkView, Photo } from '../../base/core/types'

export type FlagFilter = 'all' | 'picked' | 'notRejected' | 'rejected'

// Условия фильтра ленты. Экземпляр libfilter подходит сюда как есть.
export interface MarkFilter {
  minRating: number // 0 — любая, 1–5 — не ниже, −1 — без оценки
  colors: ReadonlySet<Label> // пусто — любой; '' — «без метки»
  flag: FlagFilter
}

export type OwnMarks = Readonly<Record<string, MarkView>>

const sameMark = (a: Mark, b: Mark) => a.rating === b.rating && a.label === b.label && a.flag === b.flag

// Отметка из файла как Mark (нули там, где в файле ничего нет).
function fromFile(photo: Photo): Mark {
  const x = photo.xmp
  const m: Mark = { rating: x?.rating ?? 0, label: x?.label ?? '', flag: x?.flag ?? 0 }
  if (m.label === 'other' && x?.labelText) m.labelText = x.labelText
  return m
}

// Действующая отметка: своя по полю, иначе из файла. Своя 0 / '' — задана (это не «нет своей»).
// lr — отметка из файла, только если действующая с ней расходится (подсказка «в Lightroom: …»).
export function markOf(photo: Photo, own?: MarkView | null): Mark {
  const file = fromFile(photo)
  if (!own) return file
  const m: Mark = { ...file }
  if (own.rating !== undefined) m.rating = own.rating
  if (own.label !== undefined) {
    m.label = own.label
    delete m.labelText // текст — только у метки из файла
  }
  if (own.flag !== undefined) m.flag = own.flag
  if (!sameMark(m, file)) m.lr = file
  return m
}

// Подходит ли действующая отметка под условия (между условиями «и»).
export function matches(mark: Mark, f: MarkFilter): boolean {
  if (f.minRating > 0 && mark.rating < f.minRating) return false
  if (f.minRating < 0 && mark.rating !== 0) return false
  // Незнакомая метка не подходит ни под цвет, ни под «без метки».
  if (f.colors.size > 0 && (mark.label === 'other' || !f.colors.has(mark.label))) return false
  if (f.flag === 'picked' && mark.flag !== 1) return false
  if (f.flag === 'notRejected' && mark.flag === -1) return false
  if (f.flag === 'rejected' && mark.flag !== -1) return false
  return true
}

export function isActive(f: MarkFilter): boolean {
  return f.minRating !== 0 || f.colors.size > 0 || f.flag !== 'all'
}

const FLAG_TEXT: Record<FlagFilter, string> = { all: '', picked: 'отобранные', notRejected: 'без отклонённых', rejected: 'отклонённые' }

// Короткая запись условий для кнопки «Фильтр»: «★≥3 · ●●».
export function filterSummary(f: MarkFilter): string {
  const parts: string[] = []
  if (f.minRating > 0) parts.push(`★≥${f.minRating}`)
  else if (f.minRating < 0) parts.push('без оценки')
  if (f.colors.size > 0) parts.push([...f.colors].map((c) => (c === '' ? '○' : '●')).join(''))
  if (f.flag !== 'all') parts.push(FLAG_TEXT[f.flag])
  return parts.join(' · ')
}

// Фото ленты под условия. Без условий — тот же массив (без пересчёта по тысячам фото).
export function filterPhotos(photos: Photo[], marks: OwnMarks, f: MarkFilter): Photo[] {
  if (!isActive(f)) return photos
  return photos.filter((p) => matches(markOf(p, marks[p.id]), f))
}
