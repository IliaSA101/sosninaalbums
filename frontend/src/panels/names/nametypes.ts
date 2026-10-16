// Типы окна «Имена» (043): повторяют Go (model.NamesOpts/NamesPlan, app.NamesFile/NamesResult) руками, как остальные в core/types.
import type { Person, View } from '../../base/core/types'

// Откуда имена и как читать список.
export interface NamesOpts {
  from: 'list' | 'files'
  text: string
  firstFirst: boolean
  caption: boolean
}

// Место листа в предпросмотре: tag — «учитель» | «ученик 3» | «групповое» ('' — не виньетка); no 0 — групповое;
// who null — имя не меняется.
export interface NamesRow {
  photo: string
  tag: string
  no: number
  line: number
  who: Person | null
}

// Строка списка без фото. reason: extra | nofile | many | dup | noseat.
export interface NamesLeft {
  line: number
  who: Person
  reason: string
  file: string
}

// Предпросмотр и план вставки (Api.PreviewNames). cols: роль каждого столбца ('' — не используется), null — без заголовка.
export interface NamesPlan {
  rows: NamesRow[]
  left: NamesLeft[]
  extra: number
  seats: number
  named: number
  missing: number
  cols: string[] | null
  ignored: string[]
  byFile: boolean
  order: boolean
  teachers: number
  pupils: number
  // Места виньетки по ролям, включая пустые (без фото); не виньетка — 0.
  teacherSeats: number
  pupilSeats: number
  needCaption: boolean
}

// Api.PickNamesFile: таблица как текст поля списка и сведения о файле.
export interface NamesFile {
  text: string
  file: string
  sheet: string
  sheets: number
  rows: number
  cancelled: boolean
}

// Ответ Api.PasteNames: named из seats — мест листа с именем после вставки, extra — лишних строк.
export interface NamesResult {
  view: View
  extra: number
  missing: number
  named: number
  seats: number
}
