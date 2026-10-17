// Типы обложки и печати (спека 032, 047).
import type { Rect, SpreadView } from './sheet'

// ---------- Обложка (спека 032): геометрию и зоны считает Go, UI переводит мм в проценты ----------

// Шаг таблицы корешка: до upto разворотов включительно — mm.
export interface SpineStep {
  upto: number
  mm: number
}

// Размеры обложки (CoverSpec). w/h = 0 — по странице альбома; нет steps — корешок = a + b·n.
export interface CoverSpec {
  preset: string // id пресета или "custom"
  w?: number
  h?: number
  wrap: number
  hinge?: number
  steps?: SpineStep[]
  a?: number
  b?: number
  safe?: number // безопасная линия от загиба, мм (047); 0 — Settings.Safe. Задаёт пресет, в карточке не правится
  spineSafe?: number // то же от корешка
}

// Типография в списке «Типография»: group — Фотосфера / Нетпринт / Свои размеры.
export interface CoverPreset {
  id: string
  name: string
  group: string
  spec: CoverSpec
}

// Зона направляющих (kind: back front spine hinge wrap trim safe) или броска (back front wrap), мм листа.
export interface CoverZone {
  kind: string
  rect: Rect
  label: string
}

export type CoverLayout = 'front' | 'both' | 'wrap'

export interface CoverView {
  sheet: SpreadView // ID "cover"
  spec: CoverSpec
  preset: string
  spine: number // мм при текущем числе разворотов
  n: number
  over: boolean // разворотов больше, чем в таблице пресета
  layout: CoverLayout
  spineBg: string // #rrggbb плашки корешка; "" — нет
  spineText: string // ID надписи на корешке; "" — нет (058, решение №19)
  zones: CoverZone[]
  drop: CoverZone[]
  safe: Rect[]
  textMaxW: number
  guide: string // имя файла разметки; "" — нет
  guideOK: boolean
  guideWarn: boolean // пропорции разметки не совпадают с листом
  pxW: number
  pxH: number
  fix: string // «Не по шаблону Фотосферы…» (047); "" — по шаблону или не Фотосфера
}

// Что уходит в App.Export (JSON-теги Go).
export interface ExportOpts {
  range: string
  quality: number
  dpi: number
  pages: boolean
  cover: boolean // 00.jpg (032); Go игнорирует без обложки и в листе
  coverOnly: boolean // только обложка: диапазон игнорируется
  fsfera: boolean // выгрузка для Фотосферы Pro (047): Go сам берёт всё, разворотами, 300 dpi
}

// Кнопка размера страницы из Go (App.PagePresets, 047). Нет поля (0/false) — пресет его не задаёт.
export interface PagePreset {
  label: string
  group: string // "" — общий ряд; «Фотосфера» — свой ряд
  w: number
  h: number
  dpi?: number
  layflat?: boolean
  safe?: number
  hint?: string
}

// Готовность к выгрузке для Фотосферы Pro (App.FsferaPlan, 047). format "" — флажка нет.
export interface FsferaPlan {
  format: string // «30×30»
  pages: string // «Тонкие» | «Картон» | «Двойной картон» | ""
  spreads: number
  spine: number
  max: number
  folder: string // имя папки книги
  block: string[] // мешает выгрузке
  coverFix: boolean // в block есть «не по шаблону»: у строки «Исправить»
  fixAt: number // номер этой строки в block; -1 — её нет
  warn: string[]
  memo: string[]
}
