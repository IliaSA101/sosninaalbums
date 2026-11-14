export interface Photo {
  id: string
  path: string
  name: string
  w: number
  h: number
  // Время съёмки для порядка ленты (075): секунды «стенного» времени; EXIF, иначе время файла (считает Go).
  shot?: number
  // Цветовой профиль (спеки 005, 119): пусто — sRGB или профиля нет; adobe, p3 и icc Go пересчитывает в sRGB сам,
  // other — не пересчитывается (CMYK, табличные профили). profile — имя из файла.
  color?: 'srgb' | 'adobe' | 'p3' | 'icc' | 'other'
  profile?: string
  // Отметки из файла (спека 004): XMP внутри JPEG и/или сайдкар; только чтение.
  xmp?: XmpMarks
}

// Цвет метки: '' — нет. У отметки из файла бывает ещё 'other' (незнакомый текст, он в labelText).
export type Label = '' | 'red' | 'yellow' | 'green' | 'blue' | 'purple'
export type Flag = -1 | 0 | 1
export interface XmpMarks {
  rating?: number
  label?: Label | 'other'
  labelText?: string // только у 'other': выводить текстом, не HTML
  flag?: -1 | 1
}
// Своя отметка (Api.Marks): нет поля — своя не задана, действует отметка из файла. 0 и '' — заданы.
export interface MarkView {
  rating?: number
  label?: Label
  flag?: Flag
  bins?: string[] // 083: ID подборок, куда входит фото
}
// Подборка фото (083): определение; членство — в MarkView.bins.
export interface Bin {
  id: string
  name: string
}
// Что поменять у выделенных (Api.SetMarks).
export interface MarkPatch {
  rating?: number
  step?: number // ±1 от действующей оценки каждого фото
  label?: Label
  toggle?: boolean // с label: у всех уже этот цвет — снять
  flag?: Flag
}
// Действующая отметка (library/marks.ts: markOf); lr — отметка из файла, если своя с ней расходится.
export interface Mark {
  rating: number
  label: Label | 'other'
  labelText?: string
  flag: Flag
  lr?: Mark
}

// Ход чтения папок (событие scan и State.scan).
export interface ScanProgress {
  active: boolean
  walking: boolean // ещё обходим папки: found растёт
  found: number
  read: number
}
