// Размер и признаки листа (спека 032): единственное место, где интерфейс читает settings.pageW.
// Всё остальное берёт w/h/mid из SpreadView (их считает Go): разворот — 2·pageW × pageH, шов pageW;
// лист виньетки — pageW, шва нет (mid = 0); обложка — шов в центре корешка.
import type { Settings, SpreadView } from '../core/types'

export interface SheetSize {
  w: number // ширина листа, мм
  h: number // высота, мм
  mid: number // шов, мм от левого края; 0 — шва и «левой/правой» страниц нет
}

export type SheetLike = Pick<SpreadView, 'w' | 'h' | 'mid'>

export function sheetSize(sv: SheetLike): SheetSize {
  return { w: sv.w, h: sv.h, mid: sv.mid }
}

// Есть ли шов: на нём рисуется корешок, а лист делится на левую и правую страницы.
export function hasSeam(sv: Pick<SpreadView, 'mid'>): boolean {
  return sv.mid > 0
}

// Размер ещё не существующего листа (плитка «новый разворот», оценка экспорта): по настройкам альбома.
// Настройки, по которым считается размер нового листа (в тестах и окне экспорта достаточно этих полей).
export type SheetSettings = Pick<Settings, 'pageW' | 'pageH' | 'sheet'>

export function newSheetSize(st: SheetSettings): SheetSize {
  return st.sheet ? { w: st.pageW, h: st.pageH, mid: 0 } : { w: 2 * st.pageW, h: st.pageH, mid: st.pageW }
}

// Формат страницы альбома (подпись «Формат 30×30 см», итог в настройках).
export function pageSize(st: Pick<Settings, 'pageW' | 'pageH'>): { w: number; h: number } {
  return { w: st.pageW, h: st.pageH }
}

// Страница под точкой x (мм листа): 0 — левая, 1 — правая; без шва страница одна.
export function pageAt(sv: Pick<SpreadView, 'mid'>, x: number): 0 | 1 {
  return sv.mid > 0 && x >= sv.mid ? 1 : 0
}

// Режим листа (031): альбом из отдельных листов без корешка. Страниц «левая/правая» в нём нет.
export function isSheet(st: Pick<Settings, 'sheet'> | undefined): boolean {
  return !!st?.sheet
}

// Слова интерфейса: у виньетки-листа «лист», у альбома «разворот».
export interface Unit {
  one: string // лист / разворот
  cap: string // Лист / Разворот
  new: string // Новый лист / Новый разворот
  all: string // Весь лист / Весь разворот
  every: string // Все листы / Все развороты
  none: string // Листов пока нет / Разворотов пока нет
  many: string // листы / развороты (именительный, множественное)
  of: string // листа / разворота (родительный: «название листа»)
  plural: [string, string, string] // формы для plural(): 1, 2, 5
}

const UNIT_SPREAD: Unit = {
  one: 'разворот',
  cap: 'Разворот',
  new: 'Новый разворот',
  all: 'Весь разворот',
  every: 'Все развороты',
  none: 'Разворотов пока нет',
  many: 'развороты',
  of: 'разворота',
  plural: ['разворот', 'разворота', 'разворотов'],
}
const UNIT_SHEET: Unit = {
  one: 'лист',
  cap: 'Лист',
  new: 'Новый лист',
  all: 'Весь лист',
  every: 'Все листы',
  none: 'Листов пока нет',
  many: 'листы',
  of: 'листа',
  plural: ['лист', 'листа', 'листов'],
}

export function unitOf(st: Pick<Settings, 'sheet'> | undefined): Unit {
  return isSheet(st) ? UNIT_SHEET : UNIT_SPREAD
}

// Размеры листа-виньетки для стартового окна, мм (20×30 и 30×40 см); свои — в пределах, что зажмёт Go.
export const SHEET_PRESETS: readonly { label: string; w: number; h: number }[] = [
  { label: '20×30 см', w: 204, h: 304 },
  { label: '30×40 см', w: 304, h: 404 },
]
// ID листа обложки (032): она не входит в View.spreads и лежит в View.cover.sheet.
export const COVER_ID = 'cover'

export const isCover = (sv: Pick<SpreadView, 'id'> | null | undefined): boolean => sv?.id === COVER_ID

// Листы вида: [обложка?] + развороты. Жесты и правки ищут лист по id здесь, а не в View.spreads (там обложки нет).
type Sheets = { spreads: readonly SpreadView[]; cover?: { sheet: SpreadView } | null }

export function sheetsOf(v: Sheets): readonly SpreadView[] {
  return v.cover ? [v.cover.sheet, ...v.spreads] : v.spreads
}

export function sheetIn(v: Sheets, id: string): SpreadView | undefined {
  return id === COVER_ID ? (v.cover?.sheet ?? undefined) : v.spreads.find((s) => s.id === id)
}
