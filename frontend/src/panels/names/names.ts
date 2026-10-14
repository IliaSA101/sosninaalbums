// Имена на фото (спека 030): разбор вставленного списка для предпросмотра окна «Имена» и поля имени в карточке фото.
// Правила разбора те же, что в Go (internal/album/names/parse.go): сам разбор и вставку делает Go, это лишь
// показ «фото ← строка» до нажатия кнопки; расхождение видно сразу после вставки в подписях на холсте.
import type { Person, SpreadView } from '../../base/core/types'

// Пределы вставки (как в Go): больше — отказ без правки.
export const MAX_BYTES = 64 << 10
export const MAX_LINES = 1000
// Знаков в одном слове имени (model.MaxPersonRunes).
export const MAX_WORD = 60

export interface Parsed {
  people: Person[]
  error: string // "" — разобрано
}

const clip = (w: string): string => [...w].slice(0, MAX_WORD).join('')

// Нумерация «12.» или «3)» в начале строки не часть имени.
function unnumber(line: string): string {
  const t = line.replace(/^\s+/, '')
  const m = /^\d+[.)]/.exec(t)
  return m ? t.slice(m[0].length) : line
}

// Слова строки: по Tab и «;», если они есть (столбцы Excel), иначе по пробелам.
function split(line: string): string[] {
  if (!/[\t;]/.test(line)) return line.split(/\s+/).filter(Boolean)
  return line
    .split(/[\t;]/)
    .map((w) => w.trim())
    .filter(Boolean)
}

// Слова в имя: «Фамилия Имя Отчество» или, при firstFirst, «Имя Отчество Фамилия»; остаток — в отчество.
function person(w: string[], firstFirst: boolean): Person {
  if (firstFirst) {
    if (w.length === 1) return { first: w[0] }
    if (w.length === 2) return { first: w[0], last: w[1] }
    const n = w.length - 1
    return { first: w[0], middle: w.slice(1, n).join(' '), last: w[n] }
  }
  if (w.length === 1) return { last: w[0] }
  if (w.length === 2) return { last: w[0], first: w[1] }
  return { last: w[0], first: w[1], middle: w.slice(2).join(' ') }
}

// Список из окна → имена по порядку строк. Пустые строки пропускаются.
export function parseNames(s: string, firstFirst: boolean): Parsed {
  if (new TextEncoder().encode(s).length > MAX_BYTES) return { people: [], error: 'Список слишком длинный: не больше 1000 строк и 64 КБ' }
  const lines = s.replace(/[\r\n]+$/, '').split('\n')
  if (lines.length > MAX_LINES) return { people: [], error: 'Список слишком длинный: не больше 1000 строк и 64 КБ' }
  const people: Person[] = []
  for (const line of lines) {
    const words = split(unnumber(line.replace(/\r$/, '')))
    if (words.length === 0) continue
    const p = person(words, firstFirst)
    people.push({ last: clip(p.last ?? ''), first: clip(p.first ?? ''), middle: clip(p.middle ?? '') })
  }
  return { people, error: '' }
}

// «Иванов Пётр Сергеевич» — как читается имя в предпросмотре и в карточке.
export const personLine = (p: Person): string => [p.last, p.first, p.middle].filter(Boolean).join(' ')

// Пустое имя (все три поля пусты) — снова имя из файла.
export const emptyPerson = (p: Person): boolean => !p.last?.trim() && !p.first?.trim() && !p.middle?.trim()

// Поля карточки → Person для Go: края обрезаны.
export const personOf = (p: Person): Person => ({
  last: (p.last ?? '').trim(),
  first: (p.first ?? '').trim(),
  middle: (p.middle ?? '').trim(),
})

export interface Row {
  photo: string
  line: Person | null // null — строки не хватило
}

export interface Preview {
  rows: Row[]
  extra: number // строк больше, чем фото
  missing: number // фото без строки и без имени (из файла или своего)
}

// Сопоставление как в Go: i-я строка — i-му фото листа. named — есть ли у фото имя уже сейчас.
export function preview(photos: readonly string[], people: readonly Person[], named: (photo: string) => boolean): Preview {
  const rows = photos.map((photo, i): Row => ({ photo, line: people[i] ?? null }))
  return {
    rows,
    extra: Math.max(0, people.length - photos.length),
    missing: rows.filter((r) => !r.line && !named(r.photo)).length,
  }
}

// Итог внизу окна: «строк лишних: 2 / фото без имени: 1».
export const summary = (p: Pick<Preview, 'extra' | 'missing'>): string => `строк лишних: ${p.extra} / фото без имени: ${p.missing}`

// Места листа с фото в порядке Go (Grid): роль нужна виньетке — групповое имени не получает, учитель
// идёт в общем порядке мест (у head-t-left — последним). Не-виньетка: роль пустая, порядок — SpreadView.photos.
export interface Slot {
  photo: string
  role: '' | 'pupil' | 'teacher' | 'group'
}

export function slotsOf(sv: Pick<SpreadView, 'photos' | 'places' | 'vig'>): Slot[] {
  if (!sv.vig || !sv.places) return sv.photos.map((photo) => ({ photo, role: '' }))
  return sv.places
    .filter((pl) => pl.photo !== '')
    .map((pl) => ({ photo: pl.photo, role: pl.role === 'teacher' || pl.role === 'group' ? pl.role : 'pupil' }))
}

// Строка окна: слот + подпись роли («учитель», «ученик 3», «групповое») и строка списка, что на него ляжет.
export interface SlotRow {
  photo: string
  tag: string // "" — не виньетка
  skipped: boolean // групповое: имя не получает (как в Go names.Paste)
  line: Person | null
  no: number // номер среди получающих имя; у пропущенного 0
}

export interface SlotPreview extends Preview {
  slots: SlotRow[]
}

// Предпросмотр по слотам: имена получают все, кроме группового, по порядку; итог — как у preview.
export function previewSlots(slots: readonly Slot[], people: readonly Person[], named: (photo: string) => boolean): SlotPreview {
  const recv = slots.filter((s) => s.role !== 'group')
  const base = preview(
    recv.map((s) => s.photo),
    people,
    named,
  )
  let i = 0
  let pupil = 0
  const rows = slots.map((s): SlotRow => {
    if (s.role === 'group') return { photo: s.photo, tag: 'групповое', skipped: true, line: null, no: 0 }
    const line = base.rows[i].line
    i++
    if (s.role === 'pupil') pupil++
    const tag = s.role === 'teacher' ? 'учитель' : s.role === 'pupil' ? `ученик ${pupil}` : ''
    return { photo: s.photo, tag, skipped: false, line, no: i }
  })
  return { rows: base.rows, extra: base.extra, missing: base.missing, slots: rows }
}
